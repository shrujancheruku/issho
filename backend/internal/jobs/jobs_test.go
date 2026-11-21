package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/jobs"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

type markArgs struct {
	Name string `json:"name"`
}

func (markArgs) Kind() string { return "mark" }

// markWorker reports each job it works on a channel.
type markWorker struct {
	river.WorkerDefaults[markArgs]
	worked chan string
}

func (w *markWorker) Work(_ context.Context, job *river.Job[markArgs]) error {
	w.worked <- job.Args.Name
	return nil
}

// A job is enqueued in the transaction that causes it (docs/spec/architecture.md,
// "The seams"): it runs after that transaction commits and never if it rolls back.
func TestJobEnqueuedInATransactionRunsOnlyAfterCommit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}

	w := &markWorker{worked: make(chan string, 10)}
	workers := jobs.NewWorkers(clock.NewFake(time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)))
	river.AddWorker(workers, w)
	client, err := jobs.New(pool, workers, nil, clock.NewFake(time.Date(2026, 11, 22, 9, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Stop(context.Background()) })

	// Rolled back: the job never existed, so nothing runs.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.InsertTx(ctx, tx, markArgs{Name: "rolled-back"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	// Committed: it runs, and not before the commit.
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.InsertTx(ctx, tx, markArgs{Name: "committed"}, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case name := <-w.worked:
		t.Fatalf("%q ran before its transaction committed", name)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	waitFor(t, ctx, w.worked, "committed")

	// A job inserted after the rolled-back one has been worked, so if the
	// rolled-back job could run it would have by now.
	if _, err := client.Insert(ctx, markArgs{Name: "sentinel"}, nil); err != nil {
		t.Fatal(err)
	}
	waitFor(t, ctx, w.worked, "sentinel")
	select {
	case name := <-w.worked:
		t.Fatalf("unexpected job %q ran", name)
	default:
	}

	var rolledBack int
	if err := pool.QueryRow(ctx,
		`select count(*) from river.river_job where args->>'name' = 'rolled-back'`).Scan(&rolledBack); err != nil {
		t.Fatal(err)
	}
	if rolledBack != 0 {
		t.Fatalf("rolled-back job left %d rows", rolledBack)
	}
}

func waitFor(t *testing.T, ctx context.Context, ch <-chan string, want string) {
	t.Helper()
	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("worked %q, want %q", got, want)
		}
	case <-ctx.Done():
		t.Fatalf("%q was never worked", want)
	}
}
