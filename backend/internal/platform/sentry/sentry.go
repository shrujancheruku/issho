// Package sentry reports unexpected errors and panics to Sentry when
// SENTRY_DSN is set. Without it every function is a no-op, so development,
// tests and CI never send anything.
//
// Only unexpected failures are reported: errors handlers return on purpose
// (connect errors with a code: not found, invalid argument…) are the
// caller's, not ours. Personal data isn't sent (the SDK's default: no IP
// addresses, headers or cookies).
package sentry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	sentrygo "github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

var enabled bool

// Init starts reporting for one service ("api", "worker"); an empty dsn
// leaves it off. tracesRate is the share of requests traced (0 to 1; see
// tracing.go). Call the returned flush before exiting so queued reports are
// sent.
func Init(dsn, release, environment, service string, tracesRate float64) (flush func(), err error) {
	if dsn == "" {
		return func() {}, nil
	}
	if err := sentrygo.Init(sentrygo.ClientOptions{
		Dsn: dsn, Release: release, Environment: environment, ServerName: service,
		AttachStacktrace: true,
		EnableTracing:    tracesRate > 0, TracesSampler: sampler(tracesRate),
	}); err != nil {
		return nil, fmt.Errorf("sentry: %w", err)
	}
	enabled = true
	return func() { sentrygo.Flush(2 * time.Second) }, nil
}

// Report sends one error with tags (the RPC, the job kind).
func Report(ctx context.Context, err error, tags map[string]string) {
	if !enabled || err == nil {
		return
	}
	hub := sentrygo.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentrygo.CurrentHub().Clone()
	}
	hub.WithScope(func(s *sentrygo.Scope) {
		for k, v := range tags {
			s.SetTag(k, v)
		}
		hub.CaptureException(err)
	})
}

// Handler reports panics in h, then lets them continue to net/http, which
// logs them and closes the connection as before. With tracing on, each
// request is also a transaction, named by its route ("POST
// /issho.v1.PlanService/GetPlan").
func Handler(h http.Handler) http.Handler {
	if !enabled {
		return h
	}
	return sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle(h)
}

// JobErrors reports River jobs that fail or panic (river.Config.ErrorHandler);
// River's own retries are unchanged.
type JobErrors struct{}

func (JobErrors) HandleError(ctx context.Context, job *rivertype.JobRow, err error) *river.ErrorHandlerResult {
	if !enabled || job == nil {
		return nil
	}
	Report(ctx, err, map[string]string{"job": job.Kind, "attempt": fmt.Sprint(job.Attempt)})
	return nil
}

func (JobErrors) HandlePanic(ctx context.Context, job *rivertype.JobRow, v any, trace string) *river.ErrorHandlerResult {
	if !enabled || job == nil {
		return nil
	}
	Report(ctx, fmt.Errorf("panic in %s job: %v\n%s", job.Kind, v, trace), map[string]string{"job": job.Kind, "panic": "true"})
	return nil
}
