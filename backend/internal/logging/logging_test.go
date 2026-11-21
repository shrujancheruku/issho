package logging_test

import (
	"bytes"
	"encoding/json"
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

// A library's failure line keeps its kind and job, and loses the error's text.
func TestWithoutErrorsDropsTheErrorAttributeOnly(t *testing.T) {
	var buf bytes.Buffer
	log := logging.WithoutErrors(logging.New(&buf, "worker")).With("job_id", 7)
	log.Error("failed", "error", "no mailbox ana@example.com", "kind", "send")

	out := buf.String()
	if strings.Contains(out, "ana@example.com") || strings.Contains(out, `"error"`) {
		t.Errorf("error text survived: %s", out)
	}
	if !strings.Contains(out, `"kind":"send"`) || !strings.Contains(out, `"job_id":7`) || !strings.Contains(out, `"cmd":"worker"`) {
		t.Errorf("other attributes were lost: %s", out)
	}
}
