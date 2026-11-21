package jobs_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/internal/jobs"
	"github.com/shrujancheruku/issho/backend/internal/logging"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

// lockedBuffer is a log destination River's goroutines write to while the test
// reads it.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type secretArgs struct {
	Email string `json:"email"`
}

func (secretArgs) Kind() string { return "secret" }

type secretWorker struct {
	river.WorkerDefaults[secretArgs]
	fail bool
}

func (w secretWorker) Work(context.Context, *river.Job[secretArgs]) error {
	if w.fail {
		return errors.New("boom for ana@example.com")
	}
	return nil
}

type failingArgs struct {
	Email string `json:"email"`
}

func (failingArgs) Kind() string { return "failing" }

type failingWorker struct {
	river.WorkerDefaults[failingArgs]
}

func (failingWorker) Work(context.Context, *river.Job[failingArgs]) error {
	return errors.New("boom for ana@example.com")
}

type panickingArgs struct {
	Email string `json:"email"`
}

func (panickingArgs) Kind() string { return "panicking" }

type panickingWorker struct {
	river.WorkerDefaults[panickingArgs]
}

func (panickingWorker) Work(context.Context, *river.Job[panickingArgs]) error {
	panic("no mailbox for ana@example.com")
}

// The worker logs exactly one line per job, with its kind and its outcome and
// nothing else: not the arguments, the error's text or a panic's value, each of
// which holds an email here, and no second line of River's about the same job.
func TestWorkerLogsOneLinePerJobWithKindAndOutcomeButNoArguments(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}

	var out lockedBuffer
	workers := river.NewWorkers()
	river.AddWorker(workers, secretWorker{})
	river.AddWorker(workers, failingWorker{})
	river.AddWorker(workers, panickingWorker{})
	client, err := jobs.New(pool, workers, logging.New(&out, "worker"))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			_ = client.Stop(context.Background())
		}
	})

	once := &river.InsertOpts{MaxAttempts: 1}
	if _, err := client.Insert(ctx, secretArgs{Email: "ana@example.com"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Insert(ctx, failingArgs{Email: "ana@example.com"}, once); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Insert(ctx, panickingArgs{Email: "ana@example.com"}, once); err != nil {
		t.Fatal(err)
	}

	jobLines := func() []map[string]any {
		var got []map[string]any
		for _, l := range strings.Split(out.String(), "\n") {
			var m map[string]any
			if json.Unmarshal([]byte(l), &m) == nil && m["msg"] == "job" {
				got = append(got, m)
			}
		}
		return got
	}
	for range 400 {
		if len(jobLines()) >= 3 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Stopping waits for River to finish with each job, so any line it writes
	// about one is in by now.
	if err := client.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	stopped = true

	want := map[string]string{"secret": "done", "failing": "failed", "panicking": "panicked"}
	got := map[string]string{}
	for _, m := range jobLines() {
		kind, _ := m["kind"].(string)
		if _, seen := got[kind]; seen {
			t.Errorf("two job lines for %q", kind)
		}
		got[kind], _ = m["outcome"].(string)
		for k := range m {
			switch k {
			case "time", "level", "msg", "cmd", "kind", "outcome":
			default:
				t.Errorf("job line has %q: %v", k, m)
			}
		}
	}
	for kind, outcome := range want {
		if got[kind] != outcome {
			t.Errorf("%s: outcome = %q, want %q", kind, got[kind], outcome)
		}
	}

	// One line per job means River says nothing more about any of them.
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		if json.Unmarshal([]byte(l), &m) != nil {
			t.Fatalf("not JSON: %s", l)
		}
		if m["msg"] == "job" {
			continue
		}
		if _, ok := m["job_id"]; ok {
			t.Errorf("a second line about a job: %s", l)
		}
	}
	if strings.Contains(out.String(), "ana@example.com") {
		t.Errorf("the log holds a job's arguments, error or panic:\n%s", out.String())
	}
}
