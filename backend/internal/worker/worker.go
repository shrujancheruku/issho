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

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/adminjobs"
	"github.com/shrujancheruku/issho/backend/internal/chat"
	"github.com/shrujancheruku/issho/backend/internal/crews"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/moderation"
	"github.com/shrujancheruku/issho/backend/internal/notifications"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/sentry"
	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// MatchEvery is how often every city is matched. Signup deadlines fall at
// local midnight and ready plans should be scheduled soon after they fill,
// so hourly rather than the daily minimum.
const MatchEvery = time.Hour

// The queues. They are separate so a long pass can't hold up pushes: a
// 20-minute match_city sits on a `passes` worker for its whole run, while
// `notify` has a worker of its own, so a push still goes out within the
// minute. River runs each queue's jobs on that queue's workers only.
const (
	// QueueNotify: send_notifications (and its receipt checks).
	QueueNotify = "notify"
	// QueuePasses: match_all, match_city and crews_city.
	QueuePasses = "passes"
	// QueueMaintenance: refresh_maps, retention, remove_suspended and the admin SPA's long
	// operations (apply_city_pack, discover_area_venues).
	QueueMaintenance = adminjobs.Queue
)

// queues is the client's queue config. River's `default` queue is kept, with
// one worker, for one deploy window: jobs enqueued by the previous release
// sit in `default` (the queue is stored on the row), and a client that
// doesn't serve it would leave them to wait forever. Nothing inserts into
// `default` any more, so it drains within a pass or two (the periodic jobs
// queue their replacements in the new queues, and uniqueness ignores the
// queue, so a leftover row only delays its own kind's next insert until it
// finishes). Remove it once `select count(*) from river.river_job where
// queue = 'default' and state in ('available','retryable','scheduled','running')`
// is 0 on the deployed database.
func queues() map[string]river.QueueConfig {
	return map[string]river.QueueConfig{
		QueueNotify:        {MaxWorkers: 1},
		QueuePasses:        {MaxWorkers: 2},
		QueueMaintenance:   {MaxWorkers: 1},
		river.QueueDefault: {MaxWorkers: 1},
	}
}

// liveStates are the job states a unique job is counted in: one queued,
// running or waiting to retry blocks another of its kind (and args).
var liveStates = adminjobs.LiveStates

// Every kind sets its queue and MaxAttempts in InsertOpts and its time
// limit in the worker's Timeout; River's defaults (a minute, 25 attempts)
// fit none of them. The periodic jobs re-run on a schedule, so a few
// attempts are enough: the next run is the retry.

// MatchAllTimeout and MatchAllMaxAttempts: match_all only lists the cities
// and queues a match_city for each.
const (
	MatchAllTimeout     = time.Minute
	MatchAllMaxAttempts = 3
)

// MatchAllArgs starts a matching pass for every city.
type MatchAllArgs struct{}

func (MatchAllArgs) Kind() string { return "match_all" }

// InsertOpts: one fan-out at a time, so a slow one is never doubled.
func (MatchAllArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueuePasses, MaxAttempts: MatchAllMaxAttempts,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: liveStates}}
}

func (w *matchAllWorker) Timeout(*river.Job[MatchAllArgs]) time.Duration { return MatchAllTimeout }

// MatchCityArgs runs one city's matching pass (plans.Runner.Match), then
// queues its crews pass.
type MatchCityArgs struct {
	City string `json:"city"`
}

func (MatchCityArgs) Kind() string { return "match_city" }

// InsertOpts keeps one pass per city queued or running at a time, so two
// passes never schedule the same plans at once (the plan-row locks in
// internal/plans would make the second one skip them anyway).
func (MatchCityArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: QueuePasses, MaxAttempts: MatchCityMaxAttempts,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: liveStates},
	}
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

// MatchCityTimeout bounds a city's pass. River's default is a minute, but
// composing walks asks a model that can think for minutes (a few walks at
// once, each retried once). It is also the budget the walks draw on: the
// pass starts no walk with less than walks.MinBudget of it left.
// MatchCityMaxAttempts is 3 because the next hourly pass retries anyway.
const (
	MatchCityTimeout     = 25 * time.Minute
	MatchCityMaxAttempts = 3
)

func (w *matchCityWorker) Timeout(*river.Job[MatchCityArgs]) time.Duration { return MatchCityTimeout }

// Work runs the city's matching pass (plans.Runner.Match), then queues its
// crews pass.
func (w *matchCityWorker) Work(ctx context.Context, job *river.Job[MatchCityArgs]) error {
	now := w.now()
	rep, err := w.runner.Match(ctx, job.Args.City, now)
	if err != nil {
		return err
	}
	w.runner.Log.Info("city matched", "city", job.Args.City, "pass", rep)
	// Crews read what matching just settled, so they are queued only after
	// it succeeded. They are a job of their own so a crews failure retries
	// without re-running Match (which can compose walks with a paid model).
	// For the same reason a failed insert doesn't fail this job: the next
	// hourly pass queues crews again.
	if _, err := river.ClientFromContext[pgx.Tx](ctx).Insert(ctx, CrewsCityArgs{City: job.Args.City}, nil); err != nil {
		w.runner.Log.Error("queue crews", "city", job.Args.City, "err", err)
	}
	return nil
}

// CrewsCityMaxAttempts is modest: the pass is cheap and the next hourly
// match_city queues another.
const CrewsCityMaxAttempts = 5

// CrewsCityArgs runs one city's crews pass (crews.Runner.Pass: decide
// offers, suggest plans, offer new crews). match_city queues it after
// matching.
type CrewsCityArgs struct {
	City string `json:"city"`
}

func (CrewsCityArgs) Kind() string { return "crews_city" }

// InsertOpts keeps one pass per city queued or running, like match_city.
func (CrewsCityArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		MaxAttempts: CrewsCityMaxAttempts,
		Queue:       QueuePasses,
		UniqueOpts:  river.UniqueOpts{ByArgs: true, ByState: liveStates},
	}
}

// CrewsCityTimeout bounds a city's crews pass (River's default is a minute;
// a big city decides and offers many crews, each in its own transaction).
const CrewsCityTimeout = 5 * time.Minute

type crewsCityWorker struct {
	river.WorkerDefaults[CrewsCityArgs]
	crews *crews.Runner
	now   func() time.Time
}

func (w *crewsCityWorker) Timeout(*river.Job[CrewsCityArgs]) time.Duration { return CrewsCityTimeout }

// Work runs the pass. One crew failing doesn't stop the others; the joined
// error fails the job so River retries and Sentry hears of it.
func (w *crewsCityWorker) Work(ctx context.Context, job *river.Job[CrewsCityArgs]) error {
	rep, err := w.crews.Pass(ctx, job.Args.City, w.now())
	w.crews.Log.Info("crews", "city", job.Args.City,
		"offered", rep.Offered, "formed", rep.Formed, "lapsed", rep.Lapsed, "suggestions", rep.Suggested, "failed", rep.Failed)
	if err != nil {
		return fmt.Errorf("crews %s: %w", job.Args.City, err)
	}
	return nil
}

// RefreshMapsEvery is how often cities' travel times are checked; each is
// rebuilt only when it's geo.RefreshAge old.
const RefreshMapsEvery = 24 * time.Hour

// RefreshMapsArgs starts the travel-time check (Maps caching terms): it
// queues a refresh_maps_city for every city.
type RefreshMapsArgs struct{}

func (RefreshMapsArgs) Kind() string { return "refresh_maps" }

// RefreshMapsTimeout and RefreshMapsMaxAttempts: the fan-out only lists
// cities and inserts jobs. RefreshMapsCityTimeout: a city's rebuild asks
// Maps for every pair of areas, which takes minutes in a big city.
const (
	RefreshMapsTimeout         = time.Minute
	RefreshMapsMaxAttempts     = 3
	RefreshMapsCityTimeout     = 10 * time.Minute
	RefreshMapsCityMaxAttempts = 3
)

func (RefreshMapsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueMaintenance, MaxAttempts: RefreshMapsMaxAttempts,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: liveStates}}
}

// RefreshMapsCityArgs rebuilds one city's travel times when they are
// geo.RefreshAge old. A city of its own so one failing (or slow) doesn't
// hold up or retry the others.
type RefreshMapsCityArgs struct {
	City string `json:"city"`
}

func (RefreshMapsCityArgs) Kind() string { return "refresh_maps_city" }

func (RefreshMapsCityArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueMaintenance, MaxAttempts: RefreshMapsCityMaxAttempts,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: liveStates}}
}

type refreshMapsWorker struct {
	river.WorkerDefaults[RefreshMapsArgs]
	pool   *pgxpool.Pool
	router maps.Router // nil: no Maps key
	log    *slog.Logger
}

func (w *refreshMapsWorker) Timeout(*river.Job[RefreshMapsArgs]) time.Duration {
	return RefreshMapsTimeout
}

type refreshMapsCityWorker struct {
	river.WorkerDefaults[RefreshMapsCityArgs]
	// refresh rebuilds a city's matrix if stale (geo.Refresh); a field so
	// tests need no Maps.
	refresh func(ctx context.Context, city string) (rebuilt bool, deleted int64, err error)
	log     *slog.Logger
}

func (w *refreshMapsCityWorker) Timeout(*river.Job[RefreshMapsCityArgs]) time.Duration {
	return RefreshMapsCityTimeout
}

// Work refreshes the city; an error fails only this job, which River retries.
func (w *refreshMapsCityWorker) Work(ctx context.Context, job *river.Job[RefreshMapsCityArgs]) error {
	rebuilt, deleted, err := w.refresh(ctx, job.Args.City)
	switch {
	case err != nil:
		return fmt.Errorf("refresh maps %s: %w", job.Args.City, err)
	case rebuilt:
		w.log.Info("travel times rebuilt", "city", job.Args.City, "stale_deleted", deleted)
	}
	return nil
}

// Work queues a refresh_maps_city for every city.
func (w *refreshMapsWorker) Work(ctx context.Context, _ *river.Job[RefreshMapsArgs]) error {
	// A missing key never deletes data. Without a router, geo.Refresh would
	// delete rows past the Maps caching limit; we skip it instead. The terms
	// are still met by rebuilding, which is the fix: production can't start
	// without GOOGLE_MAPS_API_KEY (platform/config), so this branch is a
	// development or preview worker; its stale rows stay until a key is set
	// and the next pass rebuilds them. Checked before any query.
	if w.router == nil {
		w.log.Error("travel times not checked: GOOGLE_MAPS_API_KEY is not set on the worker, so none can be rebuilt; none are deleted either")
		return nil
	}
	cities, err := dbgen.New(w.pool).ListCities(ctx)
	if err != nil || len(cities) == 0 {
		return err
	}
	params := make([]river.InsertManyParams, len(cities))
	for i, c := range cities {
		params[i] = river.InsertManyParams{Args: RefreshMapsCityArgs{City: c.ID}}
	}
	_, err = river.ClientFromContext[pgx.Tx](ctx).InsertMany(ctx, params)
	return err
}

// RetentionEvery is how often old notifications and crew chats are deleted.
const RetentionEvery = 24 * time.Hour

// RetentionArgs deletes what's past its retention (privacy).
type RetentionArgs struct{}

func (RetentionArgs) Kind() string { return "retention" }

// RetentionTimeout: the deletes go in batches of 1000, so minutes at most.
const (
	RetentionTimeout     = 5 * time.Minute
	RetentionMaxAttempts = 3
)

func (RetentionArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueMaintenance, MaxAttempts: RetentionMaxAttempts,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: liveStates}}
}

func (w *retentionWorker) Timeout(*river.Job[RetentionArgs]) time.Duration { return RetentionTimeout }

type retentionWorker struct {
	river.WorkerDefaults[RetentionArgs]
	pool *pgxpool.Pool
	log  *slog.Logger
	now  func() time.Time
}

// Work deletes finished notifications and the chats of closed crews, both
// past 30 days, and logs the counts. One failing doesn't stop the other;
// the joined error fails the job so River retries and Sentry hears of it.
func (w *retentionWorker) Work(ctx context.Context, _ *river.Job[RetentionArgs]) error {
	q := dbgen.New(w.pool)
	now := w.now()
	notes, nErr := notifications.DeleteOld(ctx, q, now)
	chats, cErr := chat.DeleteOld(ctx, q, now)
	w.log.Info("retention", "notifications_deleted", notes, "chat_messages_deleted", chats)
	return errors.Join(nErr, cErr)
}

// NotifyEvery is how often queued push notifications are sent.
const NotifyEvery = time.Minute

// SendNotificationsArgs sends queued push notifications.
type SendNotificationsArgs struct{}

func (SendNotificationsArgs) Kind() string { return "send_notifications" }

// SendNotificationsTimeout: a batch to Expo and the receipts take seconds;
// a minute's job that hangs is cut off well before the next three.
const (
	SendNotificationsTimeout     = 2 * time.Minute
	SendNotificationsMaxAttempts = 3
)

// One pass at a time: two would send the same notifications twice. On its
// own queue so a long match_city never delays a push.
func (SendNotificationsArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: QueueNotify, MaxAttempts: SendNotificationsMaxAttempts,
		UniqueOpts: river.UniqueOpts{ByState: liveStates}}
}

func (w *sendNotificationsWorker) Timeout(*river.Job[SendNotificationsArgs]) time.Duration {
	return SendNotificationsTimeout
}

type sendNotificationsWorker struct {
	river.WorkerDefaults[SendNotificationsArgs]
	notifier *notifications.Notifier
	now      func() time.Time
}

// Work sends what's queued, then checks the receipts of pushes sent a
// while ago (one job, so the two never overlap).
func (w *sendNotificationsWorker) Work(ctx context.Context, _ *river.Job[SendNotificationsArgs]) error {
	now := w.now()
	rep, err := w.notifier.Send(ctx, now)
	if rep != (notifications.Report{}) {
		w.notifier.Log.Info("notifications", "sent", rep.Sent, "no_device", rep.NoDevice, "failed", rep.Failed, "unregistered", rep.Unregistered)
	}
	rec, rErr := w.notifier.CheckReceipts(ctx, now)
	if rec != (notifications.ReceiptReport{}) {
		w.notifier.Log.Info("push receipts", "delivered", rec.Delivered, "refused", rec.Refused, "unregistered", rec.Unregistered, "expired", rec.Expired)
	}
	return errors.Join(err, rErr)
}

// The admin SPA's long operations (internal/adminjobs). The API validates and
// queues them and the SPA polls; the worker runs them here, on the
// maintenance queue, and records the result or the failure as the job's output.

type applyCityPackWorker struct {
	river.WorkerDefaults[adminjobs.ApplyCityPackArgs]
	pool   *pgxpool.Pool
	router maps.Router // nil: no Maps key, so no travel-time rebuild
	log    *slog.Logger
}

func (w *applyCityPackWorker) Timeout(*river.Job[adminjobs.ApplyCityPackArgs]) time.Duration {
	return adminjobs.ApplyCityPackTimeout
}

// Work applies the pack in one transaction (cities.Apply), then rebuilds the
// city's travel times if areas were added or moved.
func (w *applyCityPackWorker) Work(ctx context.Context, job *river.Job[adminjobs.ApplyCityPackArgs]) error {
	out, err := adminjobs.ApplyCityPack(ctx, w.pool, w.router, job.Args)
	return record(ctx, w.log, job.Args.City, out, err)
}

type discoverAreaVenuesWorker struct {
	river.WorkerDefaults[adminjobs.DiscoverAreaVenuesArgs]
	pool   *pgxpool.Pool
	places maps.PlaceSearcher // nil: no Maps key
	log    *slog.Logger
}

func (w *discoverAreaVenuesWorker) Timeout(*river.Job[adminjobs.DiscoverAreaVenuesArgs]) time.Duration {
	return adminjobs.DiscoverAreaVenuesTimeout
}

func (w *discoverAreaVenuesWorker) Work(ctx context.Context, job *river.Job[adminjobs.DiscoverAreaVenuesArgs]) error {
	out, err := adminjobs.DiscoverAreaVenues(ctx, w.pool, w.places, job.Args)
	return record(ctx, w.log, job.Args.City, out, err)
}

// remove_suspended takes a suspended person out of what they'd joined
// (moderation.Service.Suspend queues it in the suspension's transaction).

type removeSuspendedWorker struct {
	river.WorkerDefaults[moderation.RemoveSuspendedArgs]
	remover *moderation.Remover
}

func (w *removeSuspendedWorker) Timeout(*river.Job[moderation.RemoveSuspendedArgs]) time.Duration {
	return moderation.RemoveSuspendedTimeout
}

func (w *removeSuspendedWorker) Work(ctx context.Context, job *river.Job[moderation.RemoveSuspendedArgs]) error {
	return w.remover.Run(ctx, job.Args)
}

// record stores an admin job's output on the job (for GetAdminJob) and
// returns the run's error, which decides whether River retries.
func record(ctx context.Context, log *slog.Logger, city string, out adminjobs.Output, err error) error {
	if rErr := river.RecordOutput(ctx, out); rErr != nil {
		log.Error("record admin job output", "city", city, "err", rErr)
	}
	return err
}

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

// NewClient builds the River client with every job and its schedule. The
// pool must be a direct or session-pooler connection: River needs
// LISTEN/NOTIFY, which the transaction pooler doesn't support.
func NewClient(pool *pgxpool.Pool, log *slog.Logger, notifier *notifications.Notifier, m Maps) (*river.Client[pgx.Tx], error) {
	workers := river.NewWorkers()
	river.AddWorker(workers, &matchAllWorker{q: dbgen.New(pool)})
	river.AddWorker(workers, &matchCityWorker{runner: &plans.Runner{
		DB: pool, Log: log, Hours: m.Hours, Routes: m.Routes, Places: m.Places, Walks: m.Walks, WalkRoutes: m.WalkRoutes,
	}, now: time.Now})
	river.AddWorker(workers, &crewsCityWorker{crews: &crews.Runner{DB: pool, Log: log}, now: time.Now})
	river.AddWorker(workers, &refreshMapsWorker{pool: pool, router: m.Router, log: log})
	river.AddWorker(workers, &refreshMapsCityWorker{log: log, refresh: func(ctx context.Context, city string) (bool, int64, error) {
		return geo.Refresh(ctx, pool, m.Router, city, time.Now())
	}})
	river.AddWorker(workers, &applyCityPackWorker{pool: pool, router: m.Router, log: log})
	river.AddWorker(workers, &discoverAreaVenuesWorker{pool: pool, places: m.Places, log: log})
	river.AddWorker(workers, &removeSuspendedWorker{remover: &moderation.Remover{
		Q: dbgen.New(pool), Plans: &plans.Service{DB: pool}, Crews: &crews.Service{DB: pool},
	}})
	river.AddWorker(workers, &retentionWorker{pool: pool, log: log, now: time.Now})
	river.AddWorker(workers, &sendNotificationsWorker{notifier: notifier, now: time.Now})
	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema: db.RiverSchema,
		Logger: log,
		// Failed and panicking jobs are reported (no-op without SENTRY_DSN).
		ErrorHandler: sentry.JobErrors{},
		Queues:       queues(),
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
			river.NewPeriodicJob(river.PeriodicInterval(RetentionEvery),
				func() (river.JobArgs, *river.InsertOpts) { return RetentionArgs{}, nil },
				&river.PeriodicJobOpts{ID: "retention", RunOnStart: true}),
		},
		Workers: workers,
	})
}
