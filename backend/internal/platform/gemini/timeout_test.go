package gemini

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// A billed call that hit its timeout is not sent again.
func TestAttemptTimeoutIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		// A POST handler that never reads its body may not see the client leave.
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
	}))
	defer srv.Close()
	c := New("key", "m", 50*time.Millisecond)
	c.URL = srv.URL + "/%s"
	c.API.Backoff = time.Millisecond
	if _, err := c.Generate(context.Background(), Request{}); err == nil || calls.Load() != 1 {
		t.Errorf("err=%v calls=%d", err, calls.Load())
	}
}
