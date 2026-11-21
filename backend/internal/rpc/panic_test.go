package rpc_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/logging"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
	"github.com/shrujancheruku/issho/backend/internal/serve"
)

// syncBuffer is a log destination the server's goroutines write to while the
// test reads it.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// panicAuth panics with what a person gave us, as a buggy verifier might.
type panicAuth struct{}

func (panicAuth) Authenticate(context.Context, string) (rpc.Identity, error) {
	panic("cannot verify ana@example.com")
}

// A request whose handler panics still gets its one line, with the route and a
// 500, and nothing else: not the caller's address, which net/http's own panic
// log prints, nor a header, the body or the panic's value. The server is the
// real one (serve.HTTP), and the standard logger, where net/http writes when
// nothing else is set, is captured too.
func TestAPanickingRequestLogsOneLineWithRouteAndStatusOnly(t *testing.T) {
	var std syncBuffer
	prev := log.Writer()
	log.SetOutput(&std)
	t.Cleanup(func() { log.SetOutput(prev) })

	var out syncBuffer
	logger := logging.New(&out, "api")
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() {
		served <- serve.HTTP(ctx, logger, "127.0.0.1:0", rpc.NewHandler(rpc.Config{
			Version: "1.2.3",
			Auth:    panicAuth{},
			Log:     logger,
			Clock:   clock.NewFake(time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)),
		}))
	}()

	addrRe := regexp.MustCompile(`"addr":"([^"]+)"`)
	var addr string
	for range 200 {
		if m := addrRe.FindStringSubmatch(out.String()); m != nil {
			addr = m[1]
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if addr == "" {
		t.Fatal("the server never started")
	}

	req, _ := http.NewRequest(http.MethodPost, "http://"+addr+healthCheck, strings.NewReader(`{"email":"ana@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer good")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", res.StatusCode)
	}
	cancel()
	if err := <-served; err != nil {
		t.Fatal(err)
	}

	// The started line names the address the server bound, which is not a
	// person's; every other line is about the request.
	var lines []map[string]any
	var raw []string
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %s", l)
		}
		if m["msg"] == "started" || m["msg"] == "stopped" {
			continue
		}
		lines = append(lines, m)
		raw = append(raw, l)
	}
	if len(lines) != 1 || lines[0]["msg"] != "request" || lines[0]["route"] != healthCheck || lines[0]["status"] != float64(500) {
		t.Errorf("want one request line with the route and 500, got:\n%s", strings.Join(raw, "\n"))
	}
	for _, leak := range []string{"127.0.0.1", "ana@example.com", "203.0.113.9", "Bearer", "cannot verify"} {
		if strings.Contains(strings.Join(raw, "\n"), leak) {
			t.Errorf("a request line holds %q:\n%s", leak, strings.Join(raw, "\n"))
		}
	}
	if std.String() != "" {
		t.Errorf("the standard logger got output, outside the helper:\n%s", std.String())
	}
}
