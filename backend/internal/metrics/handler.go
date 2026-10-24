// Package metrics serves MetricsService: domain metrics in the admin SPA
// (M8), per city and week, led by the north star (weekly attendees who've
// been to more than one outing). The numbers are worked out in SQL
// (db/queries/metrics.sql) each time: at MVP scale there's nothing to
// precompute.
package metrics

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

// DefaultWeeks is how many weeks GetCityMetrics returns when asked for 0.
const (
	DefaultWeeks = 12
	MaxWeeks     = 52
)

// Handler serves MetricsService. cmd/api limits the service to admins
// (auth.AdminOnly); the handler checks again so a wiring mistake fails
// closed.
type Handler struct {
	Q   *dbgen.Queries
	Now func() time.Time // nil = time.Now
}

func (h *Handler) GetCityMetrics(ctx context.Context, req *connect.Request[isshov1.GetCityMetricsRequest]) (*connect.Response[isshov1.GetCityMetricsResponse], error) {
	c, ok := auth.FromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no caller"))
	}
	if !c.Admin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("admins only"))
	}
	weeks := int(req.Msg.Weeks)
	if weeks == 0 {
		weeks = DefaultWeeks
	}
	if weeks < 1 || weeks > MaxWeeks {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("weeks: 1 to 52"))
	}
	city, err := h.Q.GetCityMatching(ctx, req.Msg.CityId)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such city"))
	}
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(city.Timezone)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	last := Monday(now.In(loc))
	first := last.AddDate(0, 0, -7*(weeks-1))
	rows, err := h.Q.WeeklyMetrics(ctx, dbgen.WeeklyMetricsParams{
		CityID: req.Msg.CityId, First: pgtype.Date{Time: first, Valid: true}, Last: pgtype.Date{Time: last, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	cur, err := h.Q.CityNow(ctx, req.Msg.CityId)
	if err != nil {
		return nil, err
	}
	out := &isshov1.GetCityMetricsResponse{Now: &isshov1.CityNow{
		People: cur.People, OpenPlans: cur.OpenPlans, Waiting: cur.Waiting, UpcomingOutings: cur.UpcomingOutings, ActiveCrews: cur.ActiveCrews,
	}}
	for _, r := range rows {
		out.Weeks = append(out.Weeks, &isshov1.WeekMetrics{
			Week: r.Week.Time.Format(time.DateOnly), Attendees: r.Attendees, RepeatAttendees: r.RepeatAttendees, FirstTimers: r.FirstTimers,
			Groups: r.Groups, Invited: r.Invited, Confirmed: r.Confirmed, Attended: r.Attended,
			PlanRating: r.PlanRating, GroupRating: r.GroupRating, PicksAgain: r.PicksAgain, Picks: r.Picks, MutualPairs: r.MutualPairs,
			NewPeople: r.NewPeople, CrewsFormed: r.CrewsFormed, Reports: r.Reports,
		})
	}
	return connect.NewResponse(out), nil
}

// Monday is the local date (as midnight UTC, how dates are passed to
// Postgres) of the Monday starting t's week.
func Monday(t time.Time) time.Time {
	back := (int(t.Weekday()) + 6) % 7 // Monday 0 … Sunday 6
	d := t.AddDate(0, 0, -back)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
}
