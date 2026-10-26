package sentry

import (
	"context"
	"net/http"
	"strings"

	sentrygo "github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Performance tracing: each API request is a transaction (Handler), with a
// span per database query and connection wait (DBTracer) and per call to
// Google or another outside API (Transport). The app sends its trace with
// each request, so a slow screen on a phone links to what the API did.
//
// Spans are only made inside a sampled transaction, so all of this does
// nothing when Sentry is off, in the worker, or for requests not sampled.

// untraced are requests not worth a transaction: uptime checks, CORS
// preflights, and the chat socket, which stays open for minutes.
func untraced(name string) bool {
	return strings.HasPrefix(name, "OPTIONS ") || strings.HasSuffix(name, " /up") ||
		strings.HasSuffix(name, " /ws/chat") || strings.Contains(name, "/issho.v1.HealthService/")
}

// sampler traces a request when the app traced its side (so the two
// halves link up), and otherwise samples at rate.
func sampler(rate float64) sentrygo.TracesSampler {
	return func(ctx sentrygo.SamplingContext) float64 {
		if untraced(ctx.Span.Name) {
			return 0
		}
		switch ctx.ParentSampled {
		case sentrygo.SampledTrue:
			return 1
		case sentrygo.SampledFalse:
			return 0
		}
		return rate
	}
}

type spanKey struct{}

// child starts a span under the request's, or returns ctx unchanged when
// the request isn't traced.
func child(ctx context.Context, op, description string) context.Context {
	parent := sentrygo.SpanFromContext(ctx)
	if parent == nil {
		return ctx
	}
	span := parent.StartChild(op, sentrygo.WithDescription(description))
	return context.WithValue(span.Context(), spanKey{}, span)
}

func finish(ctx context.Context, err error) {
	span, ok := ctx.Value(spanKey{}).(*sentrygo.Span)
	if !ok {
		return
	}
	if err != nil {
		span.Status = sentrygo.SpanStatusInternalError
	}
	span.Finish()
}

// DBTracer is a pgx tracer (pgxpool.Config.ConnConfig.Tracer) making a span
// per query, named by its sqlc name ("GetPlan") rather than the SQL, and
// per wait for a pooled connection (where a new connection's ~280 ms TLS
// handshake to the database shows up).
type DBTracer struct{}

var (
	_ pgx.QueryTracer       = DBTracer{}
	_ pgxpool.AcquireTracer = DBTracer{}
)

func (DBTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return child(ctx, "db.sql.query", queryName(data.SQL))
}

func (DBTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	finish(ctx, data.Err)
}

func (DBTracer) TraceAcquireStart(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	return child(ctx, "db.connection", "acquire")
}

func (DBTracer) TraceAcquireEnd(ctx context.Context, _ *pgxpool.Pool, data pgxpool.TraceAcquireEndData) {
	finish(ctx, data.Err)
}

// queryName is sqlc's name for a query ("-- name: GetPlan :one" heads the
// SQL it generates); other SQL (transactions, goose, hand-written) is
// named by its first word.
func queryName(sql string) string {
	sql = strings.TrimSpace(sql)
	if rest, ok := strings.CutPrefix(sql, "-- name: "); ok {
		if name, _, ok := strings.Cut(rest, " "); ok {
			return name
		}
	}
	if first, _, _ := strings.Cut(sql, " "); first != "" {
		return strings.ToLower(first)
	}
	return "query"
}

// Transport makes a span per outgoing request: method, host and path only.
// No headers (Google's key is one), no query string, and no trace headers
// sent to the outside API.
func Transport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return roundTripper{base}
}

type roundTripper struct{ base http.RoundTripper }

func (t roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := child(req.Context(), "http.client", req.Method+" "+req.URL.Host+req.URL.Path)
	if ctx == req.Context() {
		return t.base.RoundTrip(req)
	}
	res, err := t.base.RoundTrip(req)
	if span, ok := ctx.Value(spanKey{}).(*sentrygo.Span); ok && res != nil {
		span.Status = sentrygo.HTTPtoSpanStatus(res.StatusCode)
		span.SetData("http.response.status_code", res.StatusCode)
	}
	finish(ctx, err)
	return res, err
}
