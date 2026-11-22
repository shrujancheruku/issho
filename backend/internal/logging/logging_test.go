package logging_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/logging"
)

// New is the one constructor: a whole run's output lands in the writer it is
// given, one JSON object per line, so a test can capture and parse all of it.
func TestNewWritesOneJSONObjectPerLineToItsWriter(t *testing.T) {
	var buf bytes.Buffer
	log := logging.New(&buf, "api")
	log.Info("started")
	log.Info("request", "route", "/x")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2:\n%s", len(lines), buf.String())
	}
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line is not JSON: %v\n%s", err, lines[0])
	}
	if first["msg"] != "started" || first["cmd"] != "api" {
		t.Errorf("line = %v, want msg started and cmd api", first)
	}
}

// A library's line keeps only the attributes named, so an error's text or a
// panic's value it attaches, under any key, is dropped; a line the filter names
// is dropped whole.
func TestRestrictedKeepsOnlyTheNamedAttributesAndDropsNamedLines(t *testing.T) {
	var buf bytes.Buffer
	log := logging.Restricted(logging.New(&buf, "worker"), []string{"job_id", "kind"},
		func(_ slog.Level, has func(string) bool) bool { return has("panic_val") }).With("job_id", 7, "metadata", "ana@example.com")
	log.Error("failed", "error", "no mailbox ana@example.com", "err", "ana@example.com", "kind", "send")
	log.Error("recovered", "panic_val", "ana@example.com")

	out := buf.String()
	if strings.Contains(out, "ana@example.com") || strings.Contains(out, "recovered") {
		t.Errorf("a dropped attribute or line survived: %s", out)
	}
	if !strings.Contains(out, `"kind":"send"`) || !strings.Contains(out, `"job_id":7`) || !strings.Contains(out, `"cmd":"worker"`) {
		t.Errorf("a kept attribute was lost: %s", out)
	}
}

// net/http's error log is free text that can name the client's address (a
// failed TLS handshake from 203.0.113.9, say), so the line keeps that something
// went wrong and nothing of the text.
func TestServerErrorLogKeepsNoneOfNetHTTPsText(t *testing.T) {
	var buf bytes.Buffer
	logging.ServerErrorLog(logging.New(&buf, "api")).Printf("http: TLS handshake error from %s: EOF", "203.0.113.9:4431")

	out := buf.String()
	if strings.Contains(out, "203.0.113.9") || strings.Contains(out, "handshake") {
		t.Errorf("net/http's text survived: %s", out)
	}
	if !strings.Contains(out, `"msg":"http server error"`) || !strings.Contains(out, `"cmd":"api"`) {
		t.Errorf("no http server error line: %s", out)
	}
}
