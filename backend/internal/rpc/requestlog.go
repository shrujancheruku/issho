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
		defer func() {
			// A panic stops here rather than reaching net/http, whose own panic
			// log prints the caller's address and the panic's value, which can
			// quote what a person sent. The request still gets its one line, as a
			// 500. The panic's value is dropped (reporting it is Sentry's job).
			v := recover()
			if v != nil {
				if !rec.wrote {
					rec.WriteHeader(http.StatusInternalServerError)
				} else {
					rec.status = http.StatusInternalServerError
				}
			}
			attrs := []any{
				"route", s.route,
				"status", rec.status,
				"duration_ms", float64(clk.Now().Sub(start)) / float64(time.Millisecond),
			}
			if s.account != "" {
				attrs = append(attrs, "account", s.account)
			}
			log.Info("request", attrs...)
			if v == http.ErrAbortHandler {
				// The handler meant to cut the connection; net/http does that
				// without logging anything.
				panic(v)
			}
		}()
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), seenKey{}, s)))
	})
}

// statusRecorder remembers the status the handler wrote.
type statusRecorder struct {
	http.ResponseWriter
	status int
	// wrote is whether the response has started, after which a panic can no
	// longer change the status the client sees.
	wrote bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status = code
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	r.wrote = true
	return r.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the real writer for flushing, which
// streaming RPCs need.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
