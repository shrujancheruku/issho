// Package worker defines the background jobs run by cmd/worker on River
// (Postgres-backed, no Redis). Jobs are thin: they call internal/plans,
// which does the work and can be tested without River.
package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/plans"
)

// MatchEvery is how often every city is matched. Signup deadlines fall at
// local midnight and ready plans should be scheduled soon after they fill,
// so hourly rather than the daily minimum.
const MatchEvery = time.Hour

// MatchAllArgs starts a matching pass for every city.
type MatchAllArgs struct{}

func (MatchAllArgs) Kind() string { return "match_all" }

// MatchCityArgs runs one city's matching pass: schedule, then suggest.
type MatchCityArgs struct {
	City string `json:"city"`
}

func (MatchCityArgs) Kind() string { return "match_city" }

// InsertOpts keeps one pass per city queued or running at a time, so two
// passes never schedule the same plans at once (the plan-row locks in
// internal/plans would make the second one skip them anyway).
func (MatchCityArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{
		ByArgs: true,
		ByState: []rivertype.JobState{
			rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
			rivertype.JobStateRetryable, rivertype.JobStateScheduled,
		},
	}}
}

type matchAllWorker struct {
	river.WorkerDefaults[MatchAllArgs]
	q *dbgen.Queries
}

func (w *matchAllWorker) Work(ctx context.Context, job *river.Job[MatchAllArgs]) error {
	cities, err := w.q.ListCities(ctx)
	if err != nil {
		return err
	}
	params := make([]river.InsertManyParams, len(cities))
	for i, c := range cities {
		params[i] = river.InsertManyParams{Args: MatchCityArgs{City: c.ID}}
	}
	if len(params) == 0 {
		return nil
	}
	_, err = river.ClientFromContext[pgx.Tx](ctx).InsertMany(ctx, params)
	return err
}

type matchCityWorker struct {
	river.WorkerDefaults[MatchCityArgs]
	runner *plans.Runner
	now    func() time.Time
}

// Work settles events whose confirmation deadline passed (people from
// cancelled groups go back on the waiting list), then schedules (plans that
// are scheduled or close free room under the city's cap), then suggests.
func (w *matchCityWorker) Work(ctx context.Context, job *river.Job[MatchCityArgs]) error {
	now := w.now()
	settled, err := w.runner.Settle(ctx, job.Args.City, now)
	if err != nil {
		return fmt.Errorf("settle %s: %w", job.Args.City, err)
	}
	rep, err := w.runner.Schedule(ctx, job.Args.City, now)
	if err != nil {
		return fmt.Errorf("schedule %s: %w", job.Args.City, err)
	}
	opened, err := w.runner.Suggest(ctx, job.Args.City, now)
	if err != nil {
		return fmt.Errorf("suggest %s: %w", job.Args.City, err)
	}
	w.runner.Log.Info("city matched", "city", job.Args.City,
		"groups_confirmed", settled.Confirmed, "groups_cancelled", settled.Cancelled, "events_done", settled.Finished,
		"checked", rep.Checked, "scheduled", rep.Scheduled, "closed", rep.Closed, "opened", opened)
	return nil
}

// NewClient builds the River client with every job and its schedule. The
// pool must be a direct or session-pooler connection: River needs
// LISTEN/NOTIFY, which the transaction pooler doesn't support.
func NewClient(pool *pgxpool.Pool, log *slog.Logger, timing plans.Timing) (*river.Client[pgx.Tx], error) {
	if err := timing.Validate(); err != nil {
		return nil, err
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &matchAllWorker{q: dbgen.New(pool)})
	river.AddWorker(workers, &matchCityWorker{runner: &plans.Runner{DB: pool, Timing: timing, Log: log}, now: time.Now})
	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema: db.RiverSchema,
		Logger: log,
		Queues: map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}},
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(river.PeriodicInterval(MatchEvery),
				func() (river.JobArgs, *river.InsertOpts) { return MatchAllArgs{}, nil },
				&river.PeriodicJobOpts{ID: "match_all", RunOnStart: true}),
		},
		Workers: workers,
	})
}
