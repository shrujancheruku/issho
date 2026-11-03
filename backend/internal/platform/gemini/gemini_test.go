package gemini

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

func fake(t *testing.T, status int, body string) (*Client, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	c := New("key", "m", time.Second)
	c.URL = srv.URL + "/%s"
	c.API.Backoff = time.Millisecond
	return c, &calls
}

// Each finishReason maps to an answer or an error, and only an HTTP failure
// that can pass (429, 5xx) is retried: a model's refusal is the same answer
// every time.
func TestFinishReasons(t *testing.T) {
	cand := func(reason, text string) string {
		return `{"candidates":[{"finishReason":"` + reason + `","content":{"parts":[{"text":"` + text + `"}]}}]}`
	}
	for _, tc := range []struct {
		name     string
		body     string
		want     string
		declined bool
		errText  string
	}{
		{"STOP", cand("STOP", "{}"), "{}", false, ""},
		{"missing reason", `{"candidates":[{"content":{"parts":[{"text":"{}"}]}}]}`, "{}", false, ""},
		{"MAX_TOKENS", cand("MAX_TOKENS", "{"), "", false, "cut off"},
		{"SAFETY", cand("SAFETY", ""), "", true, ""},
		{"unknown", cand("SOMETHING_NEW", "{}"), "", true, ""},
		{"no candidate", `{"candidates":[]}`, "", true, ""},
		{"blocked prompt", `{"promptFeedback":{"blockReason":"SAFETY"}}`, "", true, ""},
		{"thought parts skipped", `{"candidates":[{"finishReason":"STOP","content":{"parts":[{"text":"hmm","thought":true},{"text":"{}"}]}}]}`, "{}", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, calls := fake(t, http.StatusOK, tc.body)
			got, err := c.Generate(context.Background(), Request{})
			switch {
			case tc.declined:
				if !errors.Is(err, ErrDeclined) {
					t.Errorf("err = %v, want ErrDeclined", err)
				}
			case tc.errText != "":
				if err == nil || !strings.Contains(err.Error(), tc.errText) || errors.Is(err, ErrDeclined) {
					t.Errorf("err = %v, want %q", err, tc.errText)
				}
			default:
				if err != nil || got != tc.want {
					t.Errorf("got %q, %v", got, err)
				}
			}
			if calls.Load() != 1 {
				t.Errorf("%d calls: an answer, even a refusal, is never retried", calls.Load())
			}
		})
	}
}

func TestRetriesOnlyTransientHTTPFailures(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   int32
	}{{http.StatusServiceUnavailable, 3}, {http.StatusTooManyRequests, 3}, {http.StatusBadRequest, 1}, {http.StatusForbidden, 1}} {
		c, calls := fake(t, tc.status, `{"error":"x"}`)
		if _, err := c.Generate(context.Background(), Request{}); err == nil || calls.Load() != tc.want {
			t.Errorf("status %d: err=%v calls=%d, want %d", tc.status, err, calls.Load(), tc.want)
		}
	}
}
