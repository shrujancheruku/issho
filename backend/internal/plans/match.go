package plans

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

// MatchReport counts what a city's matching pass did.
type MatchReport struct {
	Settled   SettleReport
	Scheduled ScheduleReport
	Opened    int // plans suggested
	// SuggestFailed: suggestions that could not be opened.
	SuggestFailed int
	// Meeting places found and routes computed or cleared (best effort),
	// and the plans they failed for.
	MeetingPlaces, Routes, RoutesCleared int
	MeetingPlacesFailed, RoutesFailed    int
}

// LogValue lets a report be logged as one attribute group.
func (r MatchReport) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("groups_confirmed", r.Settled.Confirmed), slog.Int("groups_cancelled", r.Settled.Cancelled),
		slog.Int("events_done", r.Settled.Finished), slog.Int("chat_messages_deleted", r.Settled.ChatMessagesDeleted),
		slog.Int("feedback_asked", r.Settled.FeedbackAsked), slog.Int("settle_failed", r.Settled.Failed),
		slog.Int("checked", r.Scheduled.Checked), slog.Int("scheduled", r.Scheduled.Scheduled), slog.Int("closed", r.Scheduled.Closed),
		slog.Int("schedule_failed", r.Scheduled.Failed),
		slog.Int("opened", r.Opened), slog.Int("suggest_failed", r.SuggestFailed),
		slog.Int("meeting_places", r.MeetingPlaces), slog.Int("meeting_places_failed", r.MeetingPlacesFailed),
		slog.Int("routes", r.Routes), slog.Int("routes_cleared", r.RoutesCleared), slog.Int("routes_failed", r.RoutesFailed),
	)
}

// Match runs a city's matching pass in its order (the worker runs it
// hourly): settle events whose confirmation deadline passed, so people from
// cancelled groups are waiting again; schedule, since plans that are
// scheduled or close free room under the city's cap; then suggest. Meeting
// places for walks just opened come before routes, as a walk's route
// starts there.
//
// Failure policy (shared by Settle, Schedule, Suggest, the routes and
// meeting places here, and crews.Runner.Pass):
//   - An item is a plan, an event group, a crew or a notification. If one
//     fails it is logged with its id, reported to Sentry and counted in its
//     stage's Failed. The pass carries on with the rest.
//   - Each item commits in its own transaction, and counters go up only
//     after that commit, so a rollback never over-counts.
//   - The job returns an error (River retries it) only for a setup failure
//     (the city or its items can't be loaded, the database is down) or when
//     every item of a stage failed. Some items failing is success, with
//     Failed > 0 in the report and the log line.
//   - Stages stay in order: a setup failure in settle skips schedule and
//     suggest, whose inputs would be wrong. Meeting places and routes are
//     best effort (plans work without them): their errors are logged only.
//
// The city is loaded once, after settling, with every activity. Schedule
// uses them all (a plan opened for a season keeps its round); Suggest keeps
// only those in season in its round's month (City.InSeason).
func (r *Runner) Match(ctx context.Context, cityID string, now time.Time) (MatchReport, error) {
	var rep MatchReport
	var err error
	if rep.Settled, err = r.Settle(ctx, cityID, now); err != nil {
		return rep, fmt.Errorf("settle %s: %w", cityID, err)
	}
	c, err := LoadCity(ctx, dbgen.New(r.DB), cityID, 0)
	if err != nil {
		return rep, fmt.Errorf("load city %s: %w", cityID, err)
	}
	// A stage that failed outright still lets the Maps caching-terms cleanup
	// run (routes older than RouteMaxAge are cleared) before the error ends
	// the job; the cleanup's own error is only logged.
	var stageErr error
	if rep.Scheduled, err = r.scheduleIn(ctx, c, now); err != nil {
		stageErr = fmt.Errorf("schedule %s: %w", cityID, err)
	} else {
		suggested, err := r.suggestIn(ctx, c, now)
		rep.Opened, rep.SuggestFailed = suggested.Opened, suggested.Failed
		if err != nil {
			stageErr = fmt.Errorf("suggest %s: %w", cityID, err)
		}
	}
	if stageErr != nil {
		if ctx.Err() == nil {
			if n, err := dbgen.New(r.DB).ClearStaleRoutes(ctx, dbgen.ClearStaleRoutesParams{CityID: cityID, Cutoff: pgtype.Timestamptz{Time: now.Add(-RouteMaxAge), Valid: true}}); err != nil {
				r.Log.Warn("clear stale routes", "city", cityID, "err", err)
			} else {
				rep.RoutesCleared = int(n)
			}
		}
		return rep, stageErr
	}
	meeting, err := r.RefreshMeetingPlaces(ctx, cityID)
	rep.MeetingPlaces, rep.MeetingPlacesFailed = meeting.Found, meeting.Failed
	if err != nil {
		r.Log.Warn("meeting places", "city", cityID, "err", err)
	}
	routes, err := r.RefreshRoutes(ctx, cityID, now)
	rep.Routes, rep.RoutesCleared, rep.RoutesFailed = routes.Computed, routes.Cleared, routes.Failed
	if err != nil {
		r.Log.Warn("plan routes", "city", cityID, "err", err)
	}
	// The best-effort stages above only log; a timeout or shutdown during them
	// is still not a success.
	return rep, ctx.Err()
}
