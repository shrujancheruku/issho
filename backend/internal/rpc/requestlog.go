package rpc

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/clock"
)

// unmatched is the route logged for a request no service owns. The path of such
// a request is whatever the client typed, so it is never logged.
const unmatched = "unmatched"

// seen is what the access interceptor learns about a request that the logging
// middleware, which wraps it, reports once the request is over. The middleware
// owns it for the request's life, so it needs no lock: the interceptor runs on
// the same goroutine as the handler.
type seen struct {
	route   string
	account string
}

type seenKey struct{}

// note records the route and, when there is one, the caller on the request's
// seen, if the request is being logged.
func note(ctx context.Context, route, account string) {
	if s, ok := ctx.Value(seenKey{}).(*seen); ok {
		if route != "" {
			s.route = route
		}
		if account != "" {
			s.account = account
		}
	}
}

// logRequests logs one line per request: the route, the status, the duration and
// the account's ID when the caller is signed in, and nothing else. Issho keeps
// no IP address, email, body or header in its logs (docs/spec/operations.md,
// "Errors and logs"), so the line is built from those four fields only, never
// from the *http.Request.
func logRequests(log *slog.Logger, clk clock.Clock, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := clk.Now()
		s := &seen{route: unmatched}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), seenKey{}, s)))

		attrs := []any{
			"route", s.route,
			"status", rec.status,
			"duration_ms", float64(clk.Now().Sub(start)) / float64(time.Millisecond),
		}
		if s.account != "" {
			attrs = append(attrs, "account", s.account)
		}
		log.Info("request", attrs...)
	})
}

// statusRecorder remembers the status the handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the real writer for flushing, which
// streaming RPCs need.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
