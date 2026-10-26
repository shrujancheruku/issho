package sentry

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// Without a DSN nothing is set up or sent: development, tests and CI.
func TestOffWithoutDSN(t *testing.T) {
	flush, err := Init("", "dev", "development", "api", 0)
	if err != nil || Enabled() {
		t.Fatalf("Init(\"\") = %v, enabled %v", err, Enabled())
	}
	flush()
	h := http.NotFoundHandler()
	if got := Handler(h); got == nil {
		t.Fatal("Handler must return a handler")
	}
	Report(context.Background(), errors.New("boom"), map[string]string{"procedure": "/x"}) // must not panic
	if r := (JobErrors{}).HandleError(context.Background(), nil, errors.New("boom")); r != nil {
		t.Error("River keeps retrying as before")
	}
}
