// Package jobs builds the River client that the worker runs and that the
// domain packages enqueue through. River keeps its jobs in Postgres, so a job
// is inserted with InsertTx in the transaction that causes it: it exists, and is
// worked, only if that transaction commits.
//
// This package holds the wiring only. Each job's args and worker live with the
// domain package it belongs to, and register themselves on the river.Workers
// that the worker command passes in. The queue's due job (internal/staff) is the
// one worker registered here.
package jobs

import (
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/staff"
)

// Schema is where River's tables are (db/migrations/00001_baseline.sql).
const Schema = "river"

// NewWorkers returns the bundle every worker process registers its jobs on.
// clk is the clock the jobs read the time from: cmd/worker passes clock.Real{},
// a test a clock.Fake. Each worker added here takes it rather than asking the
// system. kinds is the registry of the queue's kinds of item (staff.Kinds),
// which the packages that open items have registered on; pool is where the
// queue's due job applies their defaults.
func NewWorkers(clk clock.Clock, pool *pgxpool.Pool, kinds *staff.Kinds) *river.Workers {
	w := river.NewWorkers()
	staff.AddWorker(w, clk, pool, kinds)
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
