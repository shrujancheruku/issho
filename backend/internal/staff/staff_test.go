package staff_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertest"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/jobs"
	"github.com/shrujancheruku/issho/backend/internal/staff"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

var start = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

// rig is a migrated database with a queue on it. The queue enqueues through an
// insert-only River client (no workers, no queues), as the API process will.
type rig struct {
	pool  *pgxpool.Pool
	clk   *clock.Fake
	kinds *staff.Kinds
	queue *staff.Queue
}

func newRig(t *testing.T) *rig {
	t.Helper()
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	inserter, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Schema: jobs.Schema})
	if err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(start)
	kinds := staff.NewKinds()
	return &rig{pool: pool, clk: clk, kinds: kinds, queue: staff.NewQueue(clk, kinds, inserter)}
}

// inTx runs f in a transaction and commits it.
func (r *rig) inTx(t *testing.T, f func(tx pgx.Tx)) {
	t.Helper()
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	f(tx)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func (r *rig) open(t *testing.T, kind, subject string, due time.Time) staff.Item {
	t.Helper()
	var item staff.Item
	r.inTx(t, func(tx pgx.Tx) {
		var err error
		item, err = r.queue.Open(context.Background(), tx, staff.OpenParams{Kind: kind, Subject: subject, DueAt: due})
		if err != nil {
			t.Fatal(err)
		}
	})
	return item
}

func noDefault(context.Context, pgx.Tx, staff.Item) error { return nil }

// Opened items come back from ListOpen soonest-due first, whatever order they
// were opened in.
func TestListOpenReturnsItemsSoonestDueFirst(t *testing.T) {
	r := newRig(t)
	r.kinds.Register("plan-review", noDefault)

	r.open(t, "plan-review", "plan-b", start.Add(72*time.Hour))
	r.open(t, "plan-review", "plan-a", start.Add(24*time.Hour))
	r.open(t, "plan-review", "plan-c", start.Add(48*time.Hour))

	items, err := r.queue.ListOpen(context.Background(), r.pool)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range items {
		got = append(got, it.Subject)
	}
	want := []string{"plan-a", "plan-c", "plan-b"}
	if len(got) != len(want) {
		t.Fatalf("subjects = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("subjects = %v, want %v", got, want)
		}
	}
	if first := items[0]; first.Kind != "plan-review" || first.State != staff.StateOpen || !first.DueAt.Equal(start.Add(24*time.Hour)) {
		t.Fatalf("first item = %+v", first)
	}
}

func (r *rig) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Opening an item and enqueueing its due job are one act in the caller's
// transaction (docs/spec/architecture.md, "The seams"): a rollback leaves
// neither, a commit leaves both, with the job set to run at the due time.
func TestOpenAndItsDueJobShareTheCallersTransaction(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	r.kinds.Register("plan-review", noDefault)
	due := start.Add(72 * time.Hour)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.queue.Open(ctx, tx, staff.OpenParams{Kind: "plan-review", Subject: "plan-a", DueAt: due}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if n := r.count(t, `select count(*) from app.queue_items`); n != 0 {
		t.Fatalf("rolled-back caller left %d items", n)
	}
	if n := r.count(t, `select count(*) from river.river_job where kind = $1`, staff.DueArgs{}.Kind()); n != 0 {
		t.Fatalf("rolled-back caller left %d due jobs", n)
	}

	item := r.open(t, "plan-review", "plan-b", due)
	if n := r.count(t, `select count(*) from river.river_job where kind = $1 and (args->>'item_id')::bigint = $2 and scheduled_at = $3`,
		staff.DueArgs{}.Kind(), item.ID, due); n != 1 {
		t.Fatalf("committed caller left %d due jobs scheduled at the due time, want 1", n)
	}
}

// A kind nobody registered has no default to apply, so the item is refused.
func TestOpenRefusesAnUnregisteredKind(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = r.queue.Open(ctx, tx, staff.OpenParams{Kind: "nobody-registered-this", Subject: "x", DueAt: start})
	if !errors.Is(err, staff.ErrUnknownKind) {
		t.Fatalf("Open err = %v, want ErrUnknownKind", err)
	}
}

// Resolving takes an item off the open list and records the reason given; it
// can't be resolved twice, and a reason is required.
func TestResolveClosesTheItemWithAReason(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	r.kinds.Register("plan-review", noDefault)
	keep := r.open(t, "plan-review", "keep", start.Add(time.Hour))
	item := r.open(t, "plan-review", "resolve-me", start.Add(2*time.Hour))
	r.clk.Advance(30 * time.Minute)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.queue.Resolve(ctx, tx, item.ID, ""); !errors.Is(err, staff.ErrReasonRequired) {
		t.Fatalf("Resolve with no reason: err = %v, want ErrReasonRequired", err)
	}
	_ = tx.Rollback(ctx)

	var got staff.Item
	r.inTx(t, func(tx pgx.Tx) {
		var err error
		if got, err = r.queue.Resolve(ctx, tx, item.ID, "reads fine, approved"); err != nil {
			t.Fatal(err)
		}
	})
	if got.State != staff.StateResolved || got.Reason != "reads fine, approved" || got.ClosedAt == nil || !got.ClosedAt.Equal(r.clk.Now()) {
		t.Fatalf("resolved item = %+v", got)
	}

	open, err := r.queue.ListOpen(ctx, r.pool)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].ID != keep.ID {
		t.Fatalf("open after resolve = %+v, want only %q", open, "keep")
	}

	tx, err = r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := r.queue.Resolve(ctx, tx, item.ID, "again"); !errors.Is(err, staff.ErrNotOpen) {
		t.Fatalf("second Resolve: err = %v, want ErrNotOpen", err)
	}
}

// work runs the due job for item once, as River would, against the rig's clock.
func (r *rig) work(t *testing.T, item staff.Item) {
	t.Helper()
	ctx := context.Background()
	w := rivertest.NewWorker(t, riverpgxv5.New(r.pool), &river.Config{Schema: jobs.Schema},
		staff.NewDueWorker(r.clk, r.pool, r.kinds))
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := w.Work(ctx, t, tx, staff.DueArgs{ItemID: item.ID}, nil); err != nil {
		t.Fatal(err)
	}
}

// isOpen reports whether the queue lists the item as open.
func (r *rig) isOpen(t *testing.T, id int64) bool {
	t.Helper()
	items, err := r.queue.ListOpen(context.Background(), r.pool)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}

// With the clock past an item's due time the due job applies the kind's default
// and marks the item defaulted; running the job again does nothing more.
func TestDueJobAppliesTheDefaultExactlyOnce(t *testing.T) {
	r := newRig(t)
	var applied []staff.Item
	r.kinds.Register("plan-review", func(_ context.Context, _ pgx.Tx, item staff.Item) error {
		applied = append(applied, item)
		return nil
	})
	item := r.open(t, "plan-review", "plan-a", start.Add(72*time.Hour))

	r.clk.Advance(72*time.Hour + time.Minute)
	r.work(t, item)
	r.work(t, item) // River delivers at least once; the second run must be a no-op.

	if len(applied) != 1 || applied[0].ID != item.ID || applied[0].Subject != "plan-a" {
		t.Fatalf("default applied to %+v, want exactly once to plan-a", applied)
	}
	if r.isOpen(t, item.ID) {
		t.Fatal("defaulted item is still open")
	}
	var state string
	var closedAt time.Time
	if err := r.pool.QueryRow(context.Background(),
		`select state, closed_at from app.queue_items where id = $1`, item.ID).Scan(&state, &closedAt); err != nil {
		t.Fatal(err)
	}
	if state != string(staff.StateDefaulted) || !closedAt.Equal(r.clk.Now()) {
		t.Fatalf("state = %q closed at %v, want defaulted at %v", state, closedAt, r.clk.Now())
	}
}

// An item resolved before it falls due is left as the person left it.
func TestDueJobLeavesAResolvedItemAlone(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	called := false
	r.kinds.Register("plan-review", func(context.Context, pgx.Tx, staff.Item) error {
		called = true
		return nil
	})
	item := r.open(t, "plan-review", "plan-a", start.Add(72*time.Hour))
	r.inTx(t, func(tx pgx.Tx) {
		if _, err := r.queue.Resolve(ctx, tx, item.ID, "approved"); err != nil {
			t.Fatal(err)
		}
	})

	r.clk.Advance(73 * time.Hour)
	r.work(t, item)

	if called {
		t.Fatal("the default was applied to a resolved item")
	}
	if n := r.count(t, `select count(*) from app.queue_items where id = $1 and state = 'resolved' and reason = 'approved'`, item.ID); n != 1 {
		t.Fatal("the resolved item was changed")
	}
}

// The job is scheduled at the due time on River's clock, which the tests can't
// move, so it also checks the injected clock: run before the item is due it
// leaves the item open.
func TestDueJobDoesNotApplyTheDefaultBeforeTheDueTime(t *testing.T) {
	r := newRig(t)
	called := false
	r.kinds.Register("plan-review", func(context.Context, pgx.Tx, staff.Item) error {
		called = true
		return nil
	})
	item := r.open(t, "plan-review", "plan-a", start.Add(72*time.Hour))

	r.clk.Advance(71 * time.Hour)
	r.work(t, item)

	if called || !r.isOpen(t, item.ID) {
		t.Fatalf("default applied before the due time (called=%v, open=%v)", called, r.isOpen(t, item.ID))
	}
}

// A default that fails leaves the item open for River's retry, and nothing it
// wrote in the transaction survives.
func TestFailedDefaultLeavesTheItemOpen(t *testing.T) {
	ctx := context.Background()
	r := newRig(t)
	r.kinds.Register("plan-review", func(ctx context.Context, tx pgx.Tx, _ staff.Item) error {
		if _, err := tx.Exec(ctx, `create table app.default_side_effect (x int)`); err != nil {
			return err
		}
		return errors.New("boom")
	})
	item := r.open(t, "plan-review", "plan-a", start)
	r.clk.Advance(time.Minute)

	w := rivertest.NewWorker(t, riverpgxv5.New(r.pool), &river.Config{Schema: jobs.Schema},
		staff.NewDueWorker(r.clk, r.pool, r.kinds))
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := w.Work(ctx, t, tx, staff.DueArgs{ItemID: item.ID}, nil); err == nil {
		t.Fatal("Work returned no error for a failing default")
	}
	if !r.isOpen(t, item.ID) {
		t.Fatal("item was closed although its default failed")
	}
	if n := r.count(t, `select count(*) from pg_tables where tablename = 'default_side_effect'`); n != 0 {
		t.Fatal("the failed default's writes were kept")
	}
}
