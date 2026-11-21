// Package jobs builds the River client that the worker runs and that the
// domain packages enqueue through. River keeps its jobs in Postgres, so a job
// is inserted with InsertTx in the transaction that causes it: it exists, and is
// worked, only if that transaction commits.
//
// This package holds the wiring only. Each job's args and worker live with the
// domain package it belongs to, and register themselves on the river.Workers
// that the worker command passes in. The queue's due times and written defaults
// (#598) come later.
package jobs

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// Schema is where River's tables are (db/migrations/00001_baseline.sql).
const Schema = "river"

type placeholderArgs struct{}

func (placeholderArgs) Kind() string { return "placeholder" }

type placeholderWorker struct {
	river.WorkerDefaults[placeholderArgs]
}

func (placeholderWorker) Work(context.Context, *river.Job[placeholderArgs]) error { return nil }

// NewWorkers returns the bundle every worker process registers its jobs on.
// River refuses to start with no worker at all, so it holds a placeholder that
// nothing enqueues; delete it when the first domain job registers (#598).
func NewWorkers() *river.Workers {
	w := river.NewWorkers()
	river.AddWorker(w, placeholderWorker{})
	return w
}

// New returns a River client on pool. Its pool must connect to Postgres
// directly or through a session pooler: River listens for new jobs with
// LISTEN/NOTIFY, which a transaction pooler breaks. A nil log uses slog's default.
func New(pool *pgxpool.Pool, workers *river.Workers, log *slog.Logger) (*river.Client[pgx.Tx], error) {
	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Logger:  log,
		Schema:  Schema,
		Queues:  map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 10}},
		Workers: workers,
	})
}
