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

// Outside a traced request StartHTTP changes nothing: no span, and no
// trace headers are ever added to outgoing calls (Google's key travels in one).
func TestStartHTTPUntraced(t *testing.T) {
	ctx := context.Background()
	if c := child(ctx, "op", "d"); c != ctx {
		t.Fatal("a span started without a traced request")
	}
	StartHTTP(ctx, "GET example.com/v1")(200, nil) // must not panic
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
	h := Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := DBTracer{}.TraceQueryStart(r.Context(), nil, pgx.TraceQueryStartData{SQL: "-- name: GetPlan :one\nselect 1"})
		DBTracer{}.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
		end := StartHTTP(r.Context(), "GET "+strings.TrimPrefix(google.URL, "http://")+"/v1/places/{id}")
		end(http.StatusOK, nil)
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
	want := []string{"db.sql.query: GetPlan", "http.client: GET " + strings.TrimPrefix(google.URL, "http://") + "/v1/places/{id}"}
	if strings.Join(descs, "|") != strings.Join(want, "|") {
		t.Errorf("spans %q, want %q (no query string)", descs, want)
	}
}
