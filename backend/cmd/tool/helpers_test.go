package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/platform/config"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// useToolCfg replaces toolCfg for one test (the subcommands read it as a
// package global, as run sets it) and restores it afterwards. Tests using it
// must not run in parallel.
func useToolCfg(t *testing.T, c config.Tool) {
	t.Helper()
	old := toolCfg
	toolCfg = c
	t.Cleanup(func() { toolCfg = old })
}

// supabaseCall is one request the fake Supabase admin API received.
type supabaseCall struct {
	Method, Path, APIKey string
	Body                 map[string]map[string]any
}

// fakeSupabase stands in for Supabase's admin API, records every request and
// points toolCfg at itself. Status is what it answers (200 by default).
type fakeSupabase struct {
	mu     sync.Mutex
	calls  []supabaseCall
	Status int
}

func newFakeSupabase(t *testing.T) *fakeSupabase {
	t.Helper()
	f := &fakeSupabase{Status: http.StatusOK}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		c := supabaseCall{Method: r.Method, Path: r.URL.Path, APIKey: r.Header.Get("apikey")}
		_ = json.Unmarshal(raw, &c.Body)
		f.mu.Lock()
		f.calls = append(f.calls, c)
		status := f.Status
		f.mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	useToolCfg(t, config.Tool{SupabaseURL: srv.URL, SupabaseSecretKey: "sb_secret_test"})
	return f
}

func (f *fakeSupabase) Calls() []supabaseCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]supabaseCall(nil), f.calls...)
}

// testPool opens the integration-test database (skipped without
// TEST_DATABASE_URL, as everywhere).
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testfixture.DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}
