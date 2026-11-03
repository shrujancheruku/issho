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

	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
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

// A list that still has a full page at the cap is an error, not a silent
// partial list (the editors would open a duplicate pull request).
func TestOpenPRsCap(t *testing.T) {
	var pages atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		pages.Add(1)
		var b strings.Builder
		b.WriteString("[")
		for i := range 100 {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`{"number":1,"title":"t","html_url":"u","head":{"ref":"renovate/x"}}`)
		}
		b.WriteString("]")
		_, _ = w.Write([]byte(b.String()))
	}))
	defer srv.Close()
	c := New("tok", "o/r")
	c.API = srv.URL
	prs, err := c.OpenPRs(context.Background(), "citypack/")
	var e *errs.Error
	if err == nil || prs != nil || !errors.As(err, &e) || e.Kind != errs.KindUpstream || pages.Load() != maxPRPages {
		t.Errorf("OpenPRs at the cap = %v, %v (%d pages)", prs, err, pages.Load())
	}
}
