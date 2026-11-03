// Package httpjson is the one JSON-over-HTTP client policy for outside APIs
// (Google, GitHub, Expo, Supabase): retries, timeouts, tracing and error
// mapping live here so the clients above it only build requests.
//
// The policy:
//   - Up to 1+MaxRetries attempts (3 by default) with jittered exponential
//     backoff. Connection resets, timeouts, 429 and 5xx are retried; other
//     statuses are not.
//   - Retry-After is honoured up to MaxRetryAfter (30 s). If the server asks
//     for longer, or the context's deadline would pass first, the call fails
//     now instead of sleeping to no purpose.
//   - A request that isn't safe to repeat is never retried. GET, HEAD,
//     OPTIONS, PUT and DELETE are safe by HTTP's own rules; POST and PATCH
//     only when the caller sets Request.Idempotent (a read-only query, or an
//     API documented to tolerate a resend). Request.NoRetry turns retries off
//     for the rest, e.g. a PUT whose second run would report a conflict.
//   - Each attempt is capped at Timeout and also ends with the context, so
//     the caller's deadline bounds the whole call, retries included.
//   - One Sentry span per call, named from the method, host and Request.Route
//     (a path template), never the query string or ids.
//   - A non-2xx answer becomes an errs.Upstream error whose cause is a
//     *StatusError holding the upstream body. The body goes to the log and to
//     Sentry through the cause; the client-facing message is fixed.
package httpjson

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/sentry"
)

// Client is safe for concurrent use. Its exported fields are settings: set
// them after New, before the first call.
type Client struct {
	Service string // for errs.Upstream and logs: "google", "github", "expo", "supabase"
	// Timeout caps one attempt (the context's deadline still ends the whole
	// call earlier).
	Timeout time.Duration
	// MaxRetries is the number of retries after the first attempt.
	MaxRetries int
	// Backoff is the wait before the first retry, doubling after that, with
	// jitter.
	Backoff time.Duration
	// MaxRetryAfter is the longest Retry-After honoured.
	MaxRetryAfter time.Duration
	// MaxBody limits how much of a response is read.
	MaxBody int64
	HTTP    *http.Client
	Log     *slog.Logger
}

// New returns a client with the default policy: 3 attempts, 500 ms backoff,
// 30 s Retry-After cap.
func New(service string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		Service: service, Timeout: timeout,
		MaxRetries: 2, Backoff: 500 * time.Millisecond, MaxRetryAfter: 30 * time.Second,
		MaxBody: 10 << 20,
		HTTP:    &http.Client{}, // no client timeout: each attempt gets its own context
		Log:     slog.Default(),
	}
}

// Request is one call.
type Request struct {
	Method string
	URL    string
	// Route is the path template for the trace and log ("/repos/{repo}/pulls").
	// Empty: the URL's path, which is only right when it carries no ids.
	Route  string
	Header map[string]string
	// Body is marshalled as JSON when not nil.
	Body any
	// Out receives the decoded JSON answer when not nil.
	Out any
	// Idempotent says a POST or PATCH may be sent twice.
	Idempotent bool
	// NoRetry never retries, whatever the method.
	NoRetry bool
}

// StatusError is a non-2xx answer. Body is the upstream's text, for logs and
// Sentry only: it is the cause inside an errs.Upstream, so it never reaches a
// client.
type StatusError struct {
	StatusCode int
	Body       string
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

// Status returns the HTTP status of a non-2xx error from Do, if err is one.
func Status(err error) (*StatusError, bool) {
	var se *StatusError
	if errors.As(err, &se) {
		return se, true
	}
	return nil, false
}

// Do sends r and decodes the answer into r.Out.
func (c *Client) Do(ctx context.Context, r Request) error {
	var payload []byte
	if r.Body != nil {
		var err error
		if payload, err = json.Marshal(r.Body); err != nil {
			return fmt.Errorf("%s: encode request: %w", c.Service, err)
		}
	}
	route := c.route(r)
	end := sentry.StartHTTP(ctx, r.Method+" "+c.host(r)+route)
	status, err := c.attempts(ctx, r, payload)
	end(status, err)
	if err == nil {
		return nil
	}
	_, isStatus := Status(err)
	if ctx.Err() != nil && !isStatus {
		return err // the caller gave up or ran out of time: not an upstream outage
	}
	if se, ok := Status(err); ok {
		// Method and route only: never the URL's query (keys) or headers.
		c.Log.Warn("upstream error", "service", c.Service, "method", r.Method, "route", route,
			"status", se.StatusCode, "body", truncate(se.Body, 512))
	}
	return errs.Upstream(c.Service, err)
}

func (c *Client) host(r Request) string {
	h := strings.TrimPrefix(strings.TrimPrefix(r.URL, "https://"), "http://")
	h, _, _ = strings.Cut(h, "/")
	return h
}

func (c *Client) route(r Request) string {
	if r.Route != "" {
		return r.Route
	}
	p := strings.TrimPrefix(strings.TrimPrefix(r.URL, "https://"), "http://")
	_, p, _ = strings.Cut(p, "/")
	p, _, _ = strings.Cut(p, "?")
	return "/" + p
}

func safe(r Request) bool {
	if r.NoRetry {
		return false
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	}
	return r.Idempotent
}

func (c *Client) attempts(ctx context.Context, r Request, payload []byte) (int, error) {
	for attempt := 0; ; attempt++ {
		status, retryAfter, err := c.once(ctx, r, payload)
		if err == nil || ctx.Err() != nil || attempt >= c.MaxRetries || !safe(r) || !retryable(status, err) {
			return status, err
		}
		wait := c.Backoff << attempt
		wait = wait/2 + rand.N(wait/2+1) // jitter: 50-100% of the step
		if retryAfter > 0 {
			if retryAfter > c.MaxRetryAfter {
				return status, err // the server wants longer than we'll wait
			}
			wait = retryAfter
		}
		if dl, ok := ctx.Deadline(); ok && time.Until(dl) <= wait {
			return status, err // would sleep past the deadline
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return status, err
		case <-t.C:
		}
	}
}

// retryable: a transport failure (reset, EOF, attempt timeout), 429 or 5xx.
// A request we couldn't build or a body that doesn't decode won't improve.
func retryable(status int, err error) bool {
	var p *permanentError
	if errors.As(err, &p) {
		return false
	}
	return status == 0 || status == http.StatusTooManyRequests || status >= 500
}

type permanentError struct{ err error }

func (e *permanentError) Error() string { return e.err.Error() }
func (e *permanentError) Unwrap() error { return e.err }

func (c *Client) once(ctx context.Context, r Request, payload []byte) (status int, retryAfter time.Duration, err error) {
	actx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	var body io.Reader
	if payload != nil {
		body = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(actx, r.Method, r.URL, body)
	if err != nil {
		return 0, 0, &permanentError{errors.New("build request")} // the URL may hold a key
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if r.Out != nil {
		req.Header.Set("Accept", "application/json")
	}
	for k, v := range r.Header {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		// A *url.Error repeats the URL, which may carry a key in its query.
		var ue interface{ Unwrap() error }
		if errors.As(err, &ue) && ue.Unwrap() != nil {
			err = ue.Unwrap()
		}
		return 0, 0, fmt.Errorf("%s %s: %w", r.Method, c.route(r), err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, c.MaxBody))
	if err != nil {
		return 0, 0, fmt.Errorf("%s %s: read response: %w", r.Method, c.route(r), err)
	}
	if resp.StatusCode/100 != 2 {
		return resp.StatusCode, parseRetryAfter(resp.Header.Get("Retry-After")),
			&StatusError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(b))}
	}
	if r.Out != nil {
		if err := json.Unmarshal(b, r.Out); err != nil {
			return resp.StatusCode, 0, &permanentError{fmt.Errorf("decode response: %w", err)}
		}
	}
	return resp.StatusCode, 0, nil
}

// parseRetryAfter reads delta-seconds or an HTTP date; 0 when absent or bad.
func parseRetryAfter(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	if s, err := strconv.Atoi(v); err == nil {
		return max(0, time.Duration(s)*time.Second)
	}
	if t, err := http.ParseTime(v); err == nil {
		return max(0, time.Until(t))
	}
	return 0
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
