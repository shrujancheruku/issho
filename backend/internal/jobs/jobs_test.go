package jobs_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/jobs"
	"github.com/shrujancheruku/issho/backend/internal/staff"
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
	workers := jobs.NewWorkers(clock.NewFake(time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)), pool, staff.NewKinds())
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

// The worker process's whole path for the queue: an item opened in a committed
// transaction, a fake clock past its due time, and the started River client
// applies the registered default and marks the item defaulted. The item is due
// in the real past, so River itself considers the job ready; the fake clock is
// what the worker's decision turns on.
func TestWorkerAppliesAnItemsDefaultOnceItIsDue(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}

	applied := make(chan string, 2)
	kinds := staff.NewKinds()
	kinds.Register("plan-review", func(_ context.Context, _ pgx.Tx, item staff.Item) error {
		applied <- item.Subject
		return nil
	})
	clk := clock.NewFake(time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC))
	client, err := jobs.New(pool, jobs.NewWorkers(clk, pool, kinds), nil, clk)
	if err != nil {
		t.Fatal(err)
	}
	queue := staff.NewQueue(clk, kinds, client)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := queue.Open(ctx, tx, staff.OpenParams{Kind: "plan-review", Subject: "plan-a", DueAt: clk.Now().Add(72 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	clk.Advance(73 * time.Hour)
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Stop(context.Background()) })

	waitFor(t, ctx, applied, "plan-a")
	for {
		open, err := queue.ListOpen(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		if len(open) == 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("the item was never marked defaulted")
		case <-time.After(50 * time.Millisecond):
		}
	}
}
