package staff

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/clock"
)

// dueWorker is the job Open enqueues for each item, scheduled at its due time.
type dueWorker struct {
	river.WorkerDefaults[DueArgs]
	clock clock.Clock
	pool  *pgxpool.Pool
	kinds *Kinds
}

// NewDueWorker returns the worker for DueArgs. It applies a kind's default in a
// transaction on pool, reading the time from clk.
func NewDueWorker(clk clock.Clock, pool *pgxpool.Pool, kinds *Kinds) river.Worker[DueArgs] {
	return &dueWorker{clock: clk, pool: pool, kinds: kinds}
}

// AddWorker registers the due worker on workers (see jobs.NewWorkers).
func AddWorker(workers *river.Workers, clk clock.Clock, pool *pgxpool.Pool, kinds *Kinds) {
	river.AddWorker(workers, NewDueWorker(clk, pool, kinds))
}

func (w *dueWorker) Work(ctx context.Context, job *river.Job[DueArgs]) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The row lock serialises this against a person's Resolve and against a
	// second delivery of the same job (River runs jobs at least once): whoever
	// comes second sees the item closed and does nothing, so the default is
	// applied once.
	q := dbgen.New(tx)
	row, err := q.LockQueueItem(ctx, job.Args.ItemID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock queue item: %w", err)
	}
	if State(row.State) != StateOpen {
		return nil
	}

	// River fires the job at the due time by the database's clock; the injected
	// clock is the one the rest of the code (and the tests) move, so check it too.
	now := w.clock.Now()
	if now.Before(row.DueAt) {
		return river.JobSnooze(row.DueAt.Sub(now))
	}

	apply, ok := w.kinds.lookup(row.Kind)
	if !ok {
		return fmt.Errorf("%w: %q (item %d)", ErrUnknownKind, row.Kind, row.ID)
	}
	if err := apply(ctx, tx, itemOf(row)); err != nil {
		return fmt.Errorf("apply default of %q (item %d): %w", row.Kind, row.ID, err)
	}
	if err := q.DefaultQueueItem(ctx, dbgen.DefaultQueueItemParams{ID: row.ID, ClosedAt: &now}); err != nil {
		return fmt.Errorf("mark queue item defaulted: %w", err)
	}
	return tx.Commit(ctx)
}
