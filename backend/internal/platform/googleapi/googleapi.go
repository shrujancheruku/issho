// Package googleapi is the shared REST/JSON transport for Google APIs that
// take an API key (Maps Platform, Cloud Translation): key in a header, JSON in
// and out, retries with backoff on 429 and 5xx.
package googleapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	key  string
	http *http.Client
	// Retries for 429 and 5xx responses, with exponential backoff from Backoff.
	MaxRetries int
	Backoff    time.Duration
}

func New(apiKey string) *Client {
	return &Client{
		key:        apiKey,
		http:       &http.Client{Timeout: 60 * time.Second},
		MaxRetries: 3,
		Backoff:    time.Second,
	}
}

// APIError is a non-2xx response. The body is Google's error JSON, which never
// contains the API key.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("google api: HTTP %d: %s", e.StatusCode, e.Body)
}

func retryable(code int) bool { return code == http.StatusTooManyRequests || code >= 500 }

// Post sends body as JSON and decodes the response into out. headers are
// extra request headers, e.g. X-Goog-FieldMask.
func (c *Client) Post(ctx context.Context, url string, headers map[string]string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return c.retry(ctx, func() error { return c.once(ctx, http.MethodPost, url, headers, payload, out) })
}

// Get fetches url and decodes the JSON response into out, with the same
// retries as Post.
func (c *Client) Get(ctx context.Context, url string, headers map[string]string, out any) error {
	return c.retry(ctx, func() error { return c.once(ctx, http.MethodGet, url, headers, nil, out) })
}

func (c *Client) retry(ctx context.Context, call func() error) error {
	for attempt := 0; ; attempt++ {
		err := call()
		var apiErr *APIError
		if err == nil || !errors.As(err, &apiErr) || !retryable(apiErr.StatusCode) || attempt >= c.MaxRetries {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.Backoff << attempt):
		}
	}
}

func (c *Client) once(ctx context.Context, method, url string, headers map[string]string, payload []byte, out any) error {
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Header rather than ?key= so the key never appears in URLs, which end up in
	// logs and error messages.
	req.Header.Set("X-Goog-Api-Key", c.key)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode/100 != 2 {
		return &APIError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(b))}
	}
	return json.Unmarshal(b, out)
}
