package jobs

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/logging"
)

// River logs through riverLog, and what survives of its lines is riverKeeps:
// the one list standing between a job's arguments (which River's own error
// attributes may quote) and the log. Tested here, on the record River would
// send, because no real job makes River log a warning with these attributes.
func TestRiverLogKeepsOnlyTheListedAttributesOfAWarning(t *testing.T) {
	var out bytes.Buffer
	log := riverLog(logging.New(&out, "worker"))

	log.Warn("Job stuck", "job_id", 7, "job_kind", "email", "queue", "default",
		"err", "bad address a@example.com", "error", "send to a@example.com failed",
		"email", "a@example.com", "metadata", "{}")

	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("no line was logged for a warning about a job: %q (%v)", out.String(), err)
	}
	kept := map[string]bool{}
	for _, k := range riverKeeps {
		kept[k] = true
	}
	for k := range got {
		switch k {
		case "time", "level", "msg", "cmd":
		default:
			if !kept[k] {
				t.Errorf("attribute %q survived, and is not in riverKeeps", k)
			}
		}
	}
	for _, k := range []string{"job_id", "job_kind", "queue"} {
		if _, ok := got[k]; !ok {
			t.Errorf("attribute %q was dropped, want it kept", k)
		}
	}
	for _, leak := range []string{"err", "error", "email", "metadata"} {
		if _, ok := got[leak]; ok {
			t.Errorf("attribute %q survived", leak)
		}
	}
	if bytes.Contains(out.Bytes(), []byte("example.com")) {
		t.Errorf("an email address reached the log: %s", out.String())
	}
}

func TestRiverLogDropsAJobLineBelowAWarningAndAnyPanicValue(t *testing.T) {
	var out bytes.Buffer
	log := riverLog(logging.New(&out, "worker"))

	log.Info("Job errored", "job_id", 7, "error", "a@example.com")
	log.Error("Job panicked", "job_id", 7, "panic_val", "a@example.com")

	if out.Len() != 0 {
		t.Errorf("lines that only repeat the job's outcome were logged: %s", out.String())
	}
}
