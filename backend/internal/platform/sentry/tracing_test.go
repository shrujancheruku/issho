package sentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sentrygo "github.com/getsentry/sentry-go"
	"github.com/jackc/pgx/v5"
)

func TestQueryName(t *testing.T) {
	for sql, want := range map[string]string{
		"-- name: GetPlan :one\nselect 1":     "GetPlan",
		"  -- name: ListFeed :many\nselect 1": "ListFeed",
		"begin":                               "begin",
		"SELECT 1":                            "select",
		"":                                    "query",
	} {
		if got := queryName(sql); got != want {
			t.Errorf("queryName(%q) = %q, want %q", sql, got, want)
		}
	}
}

func TestSampler(t *testing.T) {
	s := sampler(0.25)
	for name, want := range map[string]float64{
		"POST /issho.v1.PlanService/GetPlan":    0.25,
		"OPTIONS /issho.v1.PlanService/GetPlan": 0,
		"GET /up":                               0,
		"GET /ws/chat":                          0,
		"POST /issho.v1.HealthService/Check":    0,
	} {
		if got := s(sentrygo.SamplingContext{Span: &sentrygo.Span{Name: name}}); got != want {
			t.Errorf("%s: rate %v, want %v", name, got, want)
		}
	}
	traced := sentrygo.SamplingContext{Span: &sentrygo.Span{Name: "POST /x"}, ParentSampled: sentrygo.SampledTrue}
	if s(traced) != 1 {
		t.Error("a request the app traced should be traced too")
	}
}

// Outside a traced request the transport and tracer change nothing, and
// never add headers (Google's key travels in one; trace headers aren't sent out).
func TestTransportUntracedAndNoHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { got = r.Header.Clone() }))
	defer srv.Close()
	client := &http.Client{Transport: Transport(nil)}
	req, _ := http.NewRequestWithContext(context.Background(), "GET", srv.URL+"/v1/places/x?fields=a", nil)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if got.Get(sentrygo.SentryTraceHeader) != "" || got.Get(sentrygo.SentryBaggageHeader) != "" {
		t.Fatalf("trace headers sent to an outside API: %v", got)
	}
	if ctx := child(context.Background(), "op", "d"); ctx != context.Background() {
		t.Fatal("a span started without a traced request")
	}
}

type captured struct{ events []*sentrygo.Event }

func (c *captured) Configure(sentrygo.ClientOptions)      {}
func (c *captured) SendEvent(e *sentrygo.Event)           { c.events = append(c.events, e) }
func (c *captured) Flush(time.Duration) bool              { return true }
func (c *captured) FlushWithContext(context.Context) bool { return true }
func (c *captured) Close()                                {}

// A traced request is one transaction, named by its route, holding the
// spans made inside it: queries (here a fake one) and outside calls.
func TestRequestTransaction(t *testing.T) {
	sent := &captured{}
	if err := sentrygo.Init(sentrygo.ClientOptions{
		Dsn: "https://public@example.com/1", Transport: sent,
		EnableTracing: true, TracesSampler: sampler(1),
	}); err != nil {
		t.Fatal(err)
	}
	enabled = true
	t.Cleanup(func() { enabled = false; sentrygo.CurrentHub().BindClient(nil) })

	google := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer google.Close()
	client := &http.Client{Transport: Transport(nil)}
	h := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := DBTracer{}.TraceQueryStart(r.Context(), nil, pgx.TraceQueryStartData{SQL: "-- name: GetPlan :one\nselect 1"})
		DBTracer{}.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
		req, _ := http.NewRequestWithContext(r.Context(), "GET", google.URL+"/v1/places/abc?key=secret", nil)
		if res, err := client.Do(req); err == nil {
			res.Body.Close()
		}
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/issho.v1.PlanService/GetPlan", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/up", nil))

	if len(sent.events) != 1 {
		t.Fatalf("want one transaction (not /up), got %d", len(sent.events))
	}
	tx := sent.events[0]
	if tx.Transaction != "POST /issho.v1.PlanService/GetPlan" {
		t.Errorf("transaction %q", tx.Transaction)
	}
	var descs []string
	for _, s := range tx.Spans {
		descs = append(descs, s.Op+": "+s.Description)
	}
	want := []string{"db.sql.query: GetPlan", "http.client: GET " + strings.TrimPrefix(google.URL, "http://") + "/v1/places/abc"}
	if strings.Join(descs, "|") != strings.Join(want, "|") {
		t.Errorf("spans %q, want %q (no query string)", descs, want)
	}
}
