package plans

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// MatchReport counts what a city's matching pass did.
type MatchReport struct {
	Settled   SettleReport
	Scheduled ScheduleReport
	Opened    int // plans suggested
	// Meeting places found and routes computed or cleared (best effort).
	MeetingPlaces, Routes, RoutesCleared int
}

// LogValue lets a report be logged as one attribute group.
func (r MatchReport) LogValue() slog.Value {
	return slog.GroupValue(
		slog.Int("groups_confirmed", r.Settled.Confirmed), slog.Int("groups_cancelled", r.Settled.Cancelled),
		slog.Int("events_done", r.Settled.Finished), slog.Int("chat_messages_deleted", r.Settled.ChatMessagesDeleted),
		slog.Int("feedback_asked", r.Settled.FeedbackAsked),
		slog.Int("checked", r.Scheduled.Checked), slog.Int("scheduled", r.Scheduled.Scheduled), slog.Int("closed", r.Scheduled.Closed),
		slog.Int("opened", r.Opened),
		slog.Int("meeting_places", r.MeetingPlaces), slog.Int("routes", r.Routes), slog.Int("routes_cleared", r.RoutesCleared),
	)
}

// Match runs a city's matching pass in its order (the worker runs it
// hourly): settle events whose confirmation deadline passed, so people from
// cancelled groups are waiting again; schedule, since plans that are
// scheduled or close free room under the city's cap; then suggest. Meeting
// places for walks just opened come before routes, as a walk's route
// starts there. Settling, scheduling and suggesting stop the pass on an
// error; meeting places and routes are logged and skipped, since plans work
// without them.
func (r *Runner) Match(ctx context.Context, cityID string, now time.Time) (MatchReport, error) {
	var rep MatchReport
	var err error
	if rep.Settled, err = r.Settle(ctx, cityID, now); err != nil {
		return rep, fmt.Errorf("settle %s: %w", cityID, err)
	}
	if rep.Scheduled, err = r.Schedule(ctx, cityID, now); err != nil {
		return rep, fmt.Errorf("schedule %s: %w", cityID, err)
	}
	if rep.Opened, err = r.Suggest(ctx, cityID, now); err != nil {
		return rep, fmt.Errorf("suggest %s: %w", cityID, err)
	}
	if rep.MeetingPlaces, err = r.RefreshMeetingPlaces(ctx, cityID); err != nil {
		r.Log.Warn("meeting places", "city", cityID, "err", err)
	}
	if rep.Routes, rep.RoutesCleared, err = r.RefreshRoutes(ctx, cityID, now); err != nil {
		r.Log.Warn("plan routes", "city", cityID, "err", err)
	}
	return rep, nil
}
