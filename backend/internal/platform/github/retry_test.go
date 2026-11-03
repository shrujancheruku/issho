package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// Reads are retried on a 503; creating a ref is not, since a second try after
// a lost answer would report "already exists" for work that happened.
func TestRetryPolicy(t *testing.T) {
	var gets, posts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if gets.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`[]`))
			return
		}
		posts.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"secret internals"}`))
	}))
	defer srv.Close()
	c := New("tok", "o/r")
	c.API = srv.URL
	c.api.Backoff = time.Millisecond
	if _, err := c.OpenPRs(context.Background(), "x"); err != nil || gets.Load() != 2 {
		t.Errorf("GET: err=%v attempts=%d", err, gets.Load())
	}
	_, err := c.OpenPR(context.Background(), "b", "t", "x")
	if err == nil || posts.Load() != 1 {
		t.Fatalf("POST: err=%v attempts=%d", err, posts.Load())
	}
	if strings.Contains(err.Error(), "secret internals") {
		t.Errorf("upstream message reached the caller's error: %v", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusServiceUnavailable || apiErr.Message != "secret internals" {
		t.Errorf("cause lost: %v", err)
	}
}
