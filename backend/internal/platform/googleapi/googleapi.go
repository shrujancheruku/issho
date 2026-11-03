// Package googleapi is the shared REST/JSON transport for Google APIs that
// take an API key (Maps Platform, Cloud Translation, Gemini): key in a header,
// JSON in and out, on the shared retry and tracing policy of
// internal/platform/httpjson.
package googleapi

import (
	"context"
	"net/http"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/platform/httpjson"
)

// Client embeds the policy client, so MaxRetries and Backoff are its settings.
type Client struct {
	key string
	*httpjson.Client
}

func New(apiKey string) *Client {
	return &Client{key: apiKey, Client: httpjson.New("google", 60*time.Second)}
}

// APIError is a non-2xx response, the cause inside the errs.Upstream error
// the calls return. The body is Google's error JSON, which never contains the
// API key.
type APIError = httpjson.StatusError

// SetTimeout bounds each attempt (60 s by default). Model calls that
// think for a while need longer; interactive ones want less.
func (c *Client) SetTimeout(d time.Duration) { c.Timeout = d }

// Post sends body as JSON and decodes the response into out. headers are
// extra request headers, e.g. X-Goog-FieldMask. Every Google call here is a
// read-only query or a pure generation, so a POST may be resent.
func (c *Client) Post(ctx context.Context, url string, headers map[string]string, body, out any) error {
	return c.do(ctx, http.MethodPost, url, headers, body, out)
}

// Get fetches url and decodes the JSON response into out.
func (c *Client) Get(ctx context.Context, url string, headers map[string]string, out any) error {
	return c.do(ctx, http.MethodGet, url, headers, nil, out)
}

func (c *Client) do(ctx context.Context, method, url string, headers map[string]string, body, out any) error {
	// Header rather than ?key= so the key never appears in URLs, which end up in
	// logs and error messages.
	h := map[string]string{"X-Goog-Api-Key": c.key}
	for k, v := range headers {
		h[k] = v
	}
	return c.Client.Do(ctx, httpjson.Request{Method: method, URL: url, Header: h, Body: body, Out: out, Idempotent: true})
}
