package push

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// A send is never resent on a 5xx (it may have reached phones; Expo has no
// idempotency key). Receipts are a read, so they are.
func TestExpoRetryPolicy(t *testing.T) {
	var sends, receipts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/send" {
			sends.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if receipts.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"data":{"a":{"status":"ok"}}}`))
	}))
	defer srv.Close()
	e := NewExpo("")
	e.URL, e.ReceiptsURL = srv.URL+"/send", srv.URL+"/receipts"
	e.api.Backoff = time.Millisecond

	if _, err := e.Send(context.Background(), []Message{{To: "ExponentPushToken[a]", Body: "hi"}}); err == nil || sends.Load() != 1 {
		t.Errorf("send: err=%v attempts=%d", err, sends.Load())
	}
	got, err := e.Receipts(context.Background(), []string{"a"})
	if err != nil || !got["a"].OK || receipts.Load() != 2 {
		t.Errorf("receipts: %v %v attempts=%d", got, err, receipts.Load())
	}
}
