package supabase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// DELETE and PUT are repeatable, so a 503 is retried; a 403 isn't, and its
// body stays out of the caller's message.
func TestRetries(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	a := NewAdmin(srv.URL, "sb_secret_test")
	a.api.Backoff = time.Millisecond
	if err := a.DeleteUser(context.Background(), uuid.New()); err != nil || calls.Load() != 2 {
		t.Errorf("delete: err=%v calls=%d", err, calls.Load())
	}

	calls.Store(0)
	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "key sb_secret_leak", http.StatusForbidden)
	}))
	defer forbidden.Close()
	a = NewAdmin(forbidden.URL, "sb_secret_test")
	err := a.SetAdmin(context.Background(), uuid.New(), true)
	if err == nil || calls.Load() != 1 || strings.Contains(err.Error(), "sb_secret_leak") {
		t.Errorf("403: err=%v calls=%d", err, calls.Load())
	}
}
