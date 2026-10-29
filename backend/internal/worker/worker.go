// Package worker defines the background jobs run by cmd/worker on River
// (Postgres-backed, no Redis). Jobs are thin: they call internal/plans,
// which does the work and can be tested without River.
package worker

import (
	"context"
	"errors"
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
	"github.com/shrujancheruku/issho/backend/internal/crews"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/sentry"
	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// MatchEvery is how often every city is matched. Signup deadlines fall at
// local midnight and ready plans should be scheduled soon after they fill,
// so hourly rather than the daily minimum.
const MatchEvery = time.Hour

// MatchAllArgs starts a matching pass for every city.
type MatchAllArgs struct{}

func (MatchAllArgs) Kind() string { return "match_all" }

// MatchCityArgs runs one city's matching pass (plans.Runner.Match), then
// its crews.
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
	crews  *crews.Runner
	now    func() time.Time
}

// MatchCityTimeout bounds a city's pass. River's default is a minute, but
// composing walks asks a model that can think for minutes (a few walks at
// once, each retried once).
const MatchCityTimeout = 20 * time.Minute

func (w *matchCityWorker) Timeout(*river.Job[MatchCityArgs]) time.Duration { return MatchCityTimeout }

// Work runs the city's matching pass (plans.Runner.Match), then its crews.
func (w *matchCityWorker) Work(ctx context.Context, job *river.Job[MatchCityArgs]) error {
	now := w.now()
	rep, err := w.runner.Match(ctx, job.Args.City, now)
	if err != nil {
		return err
	}
	// Crews (offers, forming, weekly plan suggestions) don't hold up matching.
	crewPass, err := w.crews.Pass(ctx, job.Args.City, now)
	if err != nil {
		w.runner.Log.Warn("crews", "city", job.Args.City, "err", err)
	}
	w.runner.Log.Info("city matched", "city", job.Args.City, "pass", rep,
		"crews_offered", crewPass.Offered, "crews_formed", crewPass.Formed, "crews_lapsed", crewPass.Lapsed, "crew_suggestions", crewPass.Suggested)
	return nil
}

// RefreshMapsEvery is how often cities' travel times are checked; each is
// rebuilt only when it's geo.RefreshAge old.
const RefreshMapsEvery = 24 * time.Hour

// RefreshMapsArgs rebuilds stale travel-time matrices (Maps caching terms).
type RefreshMapsArgs struct{}

func (RefreshMapsArgs) Kind() string { return "refresh_maps" }

type refreshMapsWorker struct {
	river.WorkerDefaults[RefreshMapsArgs]
	pool   *pgxpool.Pool
	router maps.Router // nil: no Maps key
	log    *slog.Logger
}

// Work checks every city; one city failing doesn't stop the others, but
// fails the job so River retries it.
func (w *refreshMapsWorker) Work(ctx context.Context, _ *river.Job[RefreshMapsArgs]) error {
	cities, err := dbgen.New(w.pool).ListCities(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, c := range cities {
		rebuilt, deleted, err := geo.Refresh(ctx, w.pool, w.router, c.ID, time.Now())
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("%s: %w", c.ID, err))
		case w.router == nil && deleted > 0:
			w.log.Error("travel times past the Maps caching limit deleted; set GOOGLE_MAPS_API_KEY on the worker to rebuild them",
				"city", c.ID, "deleted", deleted)
		case rebuilt:
			w.log.Info("travel times rebuilt", "city", c.ID, "stale_deleted", deleted)
		}
	}
	return errors.Join(errs...)
}

// NotifyEvery is how often queued push notifications are sent.
const NotifyEvery = time.Minute

// SendNotificationsArgs sends queued push notifications.
type SendNotificationsArgs struct{}

func (SendNotificationsArgs) Kind() string { return "send_notifications" }

// One pass at a time: two would send the same notifications twice.
func (SendNotificationsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{UniqueOpts: river.UniqueOpts{ByState: []rivertype.JobState{
		rivertype.JobStateAvailable, rivertype.JobStatePending, rivertype.JobStateRunning,
		rivertype.JobStateRetryable, rivertype.JobStateScheduled,
	}}}
}

type sendNotificationsWorker struct {
	river.WorkerDefaults[SendNotificationsArgs]
	notifier *plans.Notifier
	now      func() time.Time
}

// Work sends what's queued, then checks the receipts of pushes sent a
// while ago (one job, so the two never overlap).
func (w *sendNotificationsWorker) Work(ctx context.Context, _ *river.Job[SendNotificationsArgs]) error {
	now := w.now()
	rep, err := w.notifier.Send(ctx, now)
	if rep != (plans.NotifyReport{}) {
		w.notifier.Log.Info("notifications", "sent", rep.Sent, "no_device", rep.NoDevice, "failed", rep.Failed, "unregistered", rep.Unregistered)
	}
	rec, rErr := w.notifier.CheckReceipts(ctx, now)
	if rec != (plans.ReceiptReport{}) {
		w.notifier.Log.Info("push receipts", "delivered", rec.Delivered, "refused", rec.Refused, "unregistered", rec.Unregistered, "expired", rec.Expired)
	}
	return errors.Join(err, rErr)
}

// NewClient builds the River client with every job and its schedule. The
// pool must be a direct or session-pooler connection: River needs
// LISTEN/NOTIFY, which the transaction pooler doesn't support.
// Maps are the Google Maps lookups the worker uses; all nil without a key.
type Maps struct {
	Router     maps.Router
	Hours      maps.HoursLookup
	Routes     maps.RouteFinder
	Places     maps.PlaceSearcher
	WalkRoutes maps.WalkRouter
	// Walks composes walking plans (Gemini and Maps); nil opens none.
	Walks *walks.Composer
}

func NewClient(pool *pgxpool.Pool, log *slog.Logger, timing plans.Timing, notifier *plans.Notifier, m Maps) (*river.Client[pgx.Tx], error) {
	if err := timing.Validate(); err != nil {
		return nil, err
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, &matchAllWorker{q: dbgen.New(pool)})
	river.AddWorker(workers, &matchCityWorker{runner: &plans.Runner{
		DB: pool, Timing: timing, Log: log, Hours: m.Hours, Routes: m.Routes, Places: m.Places, Walks: m.Walks, WalkRoutes: m.WalkRoutes,
	}, crews: &crews.Runner{DB: pool, Log: log}, now: time.Now})
	river.AddWorker(workers, &refreshMapsWorker{pool: pool, router: m.Router, log: log})
	river.AddWorker(workers, &sendNotificationsWorker{notifier: notifier, now: time.Now})
	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema: db.RiverSchema,
		Logger: log,
		// Failed and panicking jobs are reported (no-op without SENTRY_DSN).
		ErrorHandler: sentry.JobErrors{},
		Queues:       map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}},
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(river.PeriodicInterval(MatchEvery),
				func() (river.JobArgs, *river.InsertOpts) { return MatchAllArgs{}, nil },
				&river.PeriodicJobOpts{ID: "match_all", RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(NotifyEvery),
				func() (river.JobArgs, *river.InsertOpts) { return SendNotificationsArgs{}, nil },
				&river.PeriodicJobOpts{ID: "send_notifications", RunOnStart: true}),
			river.NewPeriodicJob(river.PeriodicInterval(RefreshMapsEvery),
				func() (river.JobArgs, *river.InsertOpts) { return RefreshMapsArgs{}, nil },
				&river.PeriodicJobOpts{ID: "refresh_maps", RunOnStart: true}),
		},
		Workers: workers,
	})
}
