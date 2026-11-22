package rpc_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/logging"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
)

const healthCheck = "/issho.v1.HealthService/Check"

// loggedCall sends one request to a handler that logs to the returned buffer, and
// returns the lines it logged.
func loggedCall(t *testing.T, header http.Header, body string) []map[string]any {
	t.Helper()
	var buf bytes.Buffer
	clk := clock.NewFake(time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC))
	h := rpc.NewHandler(rpc.Config{
		Version: "1.2.3",
		Auth:    fakeAuth{"good": {Subject: "acct-7"}},
		Log:     logging.New(&buf, "api"),
		Clock:   clk,
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodPost, srv.URL+healthCheck, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range header {
		req.Header[k] = v
	}
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	srv.Close() // waits for the handler, so its line is in buf

	var lines []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if l == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %s", l)
		}
		lines = append(lines, m)
	}
	return lines
}

// One line per request, with the route, the status and the duration, and the
// account's ID when the caller is signed in.
func TestApiLogsOneLinePerRequestWithRouteStatusDurationAndAccount(t *testing.T) {
	lines := loggedCall(t, http.Header{"Authorization": {"Bearer good"}}, `{}`)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1: %v", len(lines), lines)
	}
	l := lines[0]
	if l["route"] != healthCheck {
		t.Errorf("route = %v, want %s", l["route"], healthCheck)
	}
	if l["status"] != float64(200) {
		t.Errorf("status = %v, want 200", l["status"])
	}
	if _, ok := l["duration_ms"]; !ok {
		t.Errorf("no duration_ms in %v", l)
	}
	if l["account"] != "acct-7" {
		t.Errorf("account = %v, want acct-7", l["account"])
	}
}

func TestApiLogsNoAccountWhenSignedOut(t *testing.T) {
	lines := loggedCall(t, nil, `{}`)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	if _, ok := lines[0]["account"]; ok {
		t.Errorf("signed-out request logged an account: %v", lines[0])
	}
}

// Nothing about a person but the account's ID: the body's email, the
// forwarded-for header, the token and the client's own address are all absent.
func TestApiLogsNoIPAddressEmailBodyOrHeader(t *testing.T) {
	var buf bytes.Buffer
	clk := clock.NewFake(time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC))
	srv := httptest.NewServer(rpc.NewHandler(rpc.Config{
		Version: "1.2.3",
		Auth:    fakeAuth{"good": {Subject: "acct-7"}},
		Log:     logging.New(&buf, "api"),
		Clock:   clk,
	}))
	req, _ := http.NewRequest(http.MethodPost, srv.URL+healthCheck, strings.NewReader(`{"email":"ana@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer good")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("User-Agent", "probe-agent/9")
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	srv.Close()

	out := buf.String()
	if out == "" {
		t.Fatal("nothing was logged")
	}
	for _, leak := range []string{"ana@example.com", "203.0.113.9", "127.0.0.1", "probe-agent", "Bearer", "X-Forwarded-For"} {
		if strings.Contains(out, leak) {
			t.Errorf("log holds %q:\n%s", leak, out)
		}
	}
}

// A path no service owns can carry anything a client types, so it is never
// logged as the route.
func TestApiLogsAnUnknownPathWithoutItsText(t *testing.T) {
	var buf bytes.Buffer
	srv := httptest.NewServer(rpc.NewHandler(rpc.Config{
		Version: "1.2.3",
		Log:     logging.New(&buf, "api"),
		Clock:   clock.NewFake(time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)),
	}))
	res, err := srv.Client().Get(srv.URL + "/ana@example.com")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	srv.Close()
	if strings.Contains(buf.String(), "ana@example.com") {
		t.Errorf("log holds the path:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), `"status":404`) {
		t.Errorf("no 404 line:\n%s", buf.String())
	}
}
