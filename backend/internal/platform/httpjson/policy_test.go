package httpjson

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
)

func TestBodyIsBounded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, strings.Repeat("x", 100000), http.StatusBadRequest)
	}))
	defer srv.Close()
	err := testClient().Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL})
	se, ok := Status(err)
	if !ok || len(se.Body) > maxErrBody+len("…") || len(err.Error()) > 3*maxErrBody {
		t.Errorf("body and error not bounded: %v", err != nil)
	}
}

func TestHideBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "user a@example.com not found", http.StatusForbidden)
	}))
	defer srv.Close()
	var logs bytes.Buffer
	c := testClient()
	c.HideBody = true
	c.Log = slog.New(slog.NewTextHandler(&logs, nil))
	err := c.Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL})
	se, ok := Status(err)
	if !ok || se.StatusCode != 403 || se.Body != "" || strings.Contains(err.Error(), "example.com") {
		t.Errorf("err = %v", err)
	}
	if strings.Contains(logs.String(), "example.com") || !strings.Contains(logs.String(), "status=403") {
		t.Errorf("log = %s", logs.String())
	}
}

func TestExpectedStatusIsAnAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer srv.Close()
	var logs bytes.Buffer
	c := testClient()
	c.Log = slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo}))
	err := c.Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL, Expect: []int{404}})
	var e *errs.Error
	if se, ok := Status(err); !ok || se.StatusCode != 404 || errors.As(err, &e) {
		t.Errorf("want a bare StatusError, got %v", err)
	}
	if logs.Len() != 0 {
		t.Errorf("an expected status logged above debug: %s", logs.String())
	}
	// Unlisted statuses are still failures.
	err = c.Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL, Expect: []int{409}})
	if !errors.As(err, &e) {
		t.Errorf("got %v", err)
	}
}

func TestTimeoutRetryCanBeOff(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		<-r.Context().Done()
	}))
	defer srv.Close()
	c := testClient()
	c.Timeout = 50 * time.Millisecond
	c.RetryTimeouts = false
	if err := c.Do(context.Background(), Request{Method: http.MethodGet, URL: srv.URL}); err == nil || calls.Load() != 1 {
		t.Errorf("err=%v calls=%d", err, calls.Load())
	}
}

func TestCancelDuringBackoff(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c := testClient()
	c.Backoff = 20 * time.Second // no deadline, so only the cancel can end the sleep
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	start := time.Now()
	err := c.Do(ctx, Request{Method: http.MethodGet, URL: srv.URL})
	if err == nil || calls.Load() != 1 || time.Since(start) > 2*time.Second {
		t.Errorf("err=%v calls=%d after %v", err, calls.Load(), time.Since(start))
	}
}
