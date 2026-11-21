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
	"github.com/shrujancheruku/issho/backend/internal/clock"
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

// The worker logs one line per job, with its kind and outcome and nothing of
// its arguments.
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
	client, err := jobs.New(pool, workers, logging.New(&out, "worker"), clock.NewFake(time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Stop(context.Background()) })

	if _, err := client.Insert(ctx, secretArgs{Email: "ana@example.com"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Insert(ctx, failingArgs{Email: "ana@example.com"}, &river.InsertOpts{MaxAttempts: 1}); err != nil {
		t.Fatal(err)
	}

	outcomes := map[string]string{}
	for range 400 {
		if len(outcomes) == 2 {
			break
		}
		time.Sleep(50 * time.Millisecond)
		for _, l := range strings.Split(out.String(), "\n") {
			var m map[string]any
			if json.Unmarshal([]byte(l), &m) != nil || m["msg"] != "job" {
				continue
			}
			outcomes[m["kind"].(string)] = m["outcome"].(string)
		}
	}
	if outcomes["secret"] != "done" || outcomes["failing"] != "failed" {
		t.Errorf("outcomes = %v, want secret done and failing failed\n%s", outcomes, out.String())
	}
	if strings.Contains(out.String(), "ana@example.com") {
		t.Errorf("the log holds the job's arguments or error text:\n%s", out.String())
	}
}
