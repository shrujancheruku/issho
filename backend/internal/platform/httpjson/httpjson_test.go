package httpjson

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

func testClient() *Client {
	c := New("test", 5*time.Second)
	c.Backoff = time.Millisecond
	c.Log = slog.New(slog.NewTextHandler(io.Discard, nil))
	return c
}

func TestRetriesConnectionReset(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.(*net.TCPConn).SetLinger(0) // RST rather than FIN
			_ = conn.Close()
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	var out struct{ OK bool }
	if err := testClient().Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL, Out: &out}); err != nil {
		t.Fatal(err)
	}
	if !out.OK || calls.Load() != 2 {
		t.Errorf("ok=%v calls=%d", out.OK, calls.Load())
	}
}

func TestRetryAfterIsHonoured(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	start := time.Now()
	var out map[string]any
	if err := testClient().Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL, Out: &out}); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d < 900*time.Millisecond || d > 3*time.Second {
		t.Errorf("waited %v, want about 1s", d)
	}
}

func TestRetryAfterBeyondCapFailsNow(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	c := testClient()
	start := time.Now()
	err := c.Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL})
	if err == nil || calls.Load() != 1 || time.Since(start) > time.Second {
		t.Errorf("err=%v calls=%d after %v", err, calls.Load(), time.Since(start))
	}
}

func TestClientErrorIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "bad", http.StatusBadRequest)
	}))
	defer srv.Close()
	err := testClient().Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL})
	if err == nil || calls.Load() != 1 {
		t.Errorf("err=%v calls=%d", err, calls.Load())
	}
}

func TestPostIsRetriedOnlyWhenIdempotent(t *testing.T) {
	for _, tc := range []struct {
		name       string
		idempotent bool
		noRetry    bool
		method     string
		want       int32
	}{
		{"post", false, false, http.MethodPost, 1},
		{"idempotent post", true, false, http.MethodPost, 3},
		{"patch", false, false, http.MethodPatch, 1},
		{"put", false, false, http.MethodPut, 3},
		{"put marked no retry", false, true, http.MethodPut, 1},
		{"delete", false, false, http.MethodDelete, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer srv.Close()
			err := testClient().Do(context.Background(), Request{Method: tc.method, URL: srv.URL, Idempotent: tc.idempotent, NoRetry: tc.noRetry})
			if err == nil || calls.Load() != tc.want {
				t.Errorf("err=%v calls=%d, want %d", err, calls.Load(), tc.want)
			}
		})
	}
}

func TestContextDeadlineCutsOffRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := testClient()
	c.Backoff = 10 * time.Second // the next wait would pass the deadline
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := c.Do(ctx, Request{Method: http.MethodGet, URL: srv.URL})
	if err == nil || calls.Load() != 1 || time.Since(start) > time.Second {
		t.Errorf("err=%v calls=%d after %v", err, calls.Load(), time.Since(start))
	}
}

func TestCancelledContextIsNotAnUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := testClient().Do(ctx, Request{Method: http.MethodGet, URL: srv.URL})
	var e *errs.Error
	if !errors.Is(err, context.DeadlineExceeded) || errors.As(err, &e) {
		t.Errorf("got %v", err)
	}
}

func TestAttemptTimeoutIsRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			<-r.Context().Done()
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := testClient()
	c.Timeout = 100 * time.Millisecond
	if err := c.Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL}); err != nil || calls.Load() != 2 {
		t.Errorf("err=%v calls=%d", err, calls.Load())
	}
}

// The upstream's body is for logs: the error a client receives names only the
// service, and no secret in the URL's query or the body shows up in it.
func TestUpstreamBodyNeverReachesClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal db password=hunter2", http.StatusBadGateway)
	}))
	defer srv.Close()
	var logs bytes.Buffer
	c := testClient()
	c.MaxRetries = 0
	c.Log = slog.New(slog.NewTextHandler(&logs, nil))
	err := c.Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL + "/v1/things?key=SECRETKEY"})

	next := func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) { return nil, err }
	_, cerr := errs.Interceptor(slog.New(slog.NewTextHandler(io.Discard, nil)))(next)(context.Background(), connect.NewRequest(&struct{}{}))
	if connect.CodeOf(cerr) != connect.CodeUnavailable {
		t.Fatalf("code = %v", connect.CodeOf(cerr))
	}
	if msg := cerr.Error(); strings.Contains(msg, "hunter2") || strings.Contains(msg, "SECRETKEY") {
		t.Errorf("client message leaks: %q", msg)
	}
	if se, ok := Status(err); !ok || se.StatusCode != http.StatusBadGateway || !strings.Contains(se.Body, "hunter2") {
		t.Errorf("cause lost: %v", err)
	}
	if !strings.Contains(logs.String(), "hunter2") || strings.Contains(logs.String(), "SECRETKEY") {
		t.Errorf("log = %s", logs.String())
	}
}

func TestEncodesAndDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if r.Header.Get("Content-Type") != "application/json" || r.Header.Get("X-Test") != "1" || string(b) != `{"a":1}` {
			t.Errorf("request: %s %v %s", r.Header, r.Method, b)
		}
		_, _ = w.Write([]byte(`{"b":2}`))
	}))
	defer srv.Close()
	var out struct{ B int }
	err := testClient().Do(context.Background(), Request{
		Method: http.MethodPost, URL: srv.URL, Header: map[string]string{"X-Test": "1"},
		Body: map[string]int{"a": 1}, Out: &out,
	})
	if err != nil || out.B != 2 {
		t.Errorf("err=%v out=%+v", err, out)
	}
}

func TestBadJSONIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()
	var out map[string]any
	if err := testClient().Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL, Out: &out}); err == nil || calls.Load() != 1 {
		t.Errorf("err=%v calls=%d", err, calls.Load())
	}
}

func TestParseRetryAfter(t *testing.T) {
	if parseRetryAfter("3") != 3*time.Second || parseRetryAfter("") != 0 || parseRetryAfter("soon") != 0 {
		t.Error("delta-seconds or garbage")
	}
	if d := parseRetryAfter(time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)); d < 8*time.Second || d > 10*time.Second {
		t.Errorf("http date: %v", d)
	}
}
