// Package plans stores open plans, signups and scheduled events, converting
// between the database and internal/matching's types. Deciding what to open
// and when to schedule is matching's job and the worker's; this package only
// loads and saves.
package plans

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// ErrClosed means the plan doesn't exist or no longer takes signups.
var ErrClosed = errors.New("plan is not open")

// Close reasons (app.plans.closed_reason).
const (
	ClosedDone      = "done"      // scheduled, nobody left waiting
	ClosedNoGroup   = "no_group"  // the deadline passed and no group could form
	ClosedWithdrawn = "withdrawn" // taken down
)

// wholeDay stands for matching's whole-day Occasion (Block "") in
// app.plan_signup_unavailable, whose primary key can't hold a null.
const wholeDay = "day"

// Window is when a round can happen and when its signups close.
type Window struct {
	Start    time.Time // a local date in the city, as midnight UTC (like matching.Occasion)
	Days     int
	Deadline time.Time
}

// Occasions lists the round's possible times, for matching.ScheduleInput.
func (w Window) Occasions() []matching.Occasion { return matching.Occasions(w.Start, w.Days) }

// Open is an open plan.
type Open struct {
	ID uuid.UUID
	matching.OpenPlan
	Round  int
	Window Window
	// Google place ids of the activity's and the follow-up's venues; "" =
	// none (a meeting point only, or nothing found when it opened).
	Venue, FollowUpVenue string
}

// ListOpen returns a city's open plans with their signup counts, oldest first.
func ListOpen(ctx context.Context, q *dbgen.Queries, cityID string) ([]Open, error) {
	rows, err := q.ListOpenPlans(ctx, cityID)
	if err != nil {
		return nil, err
	}
	out := make([]Open, len(rows))
	for i, r := range rows {
		out[i] = Open{
			ID: r.ID,
			OpenPlan: matching.OpenPlan{
				Plan: matching.Plan{
					Area:      matching.AreaID(r.AreaID),
					Activity:  matching.ActivityID(r.ActivityID),
					FollowUp:  matching.ActivityID(deref(r.FollowUpID)),
					Primary:   r.PrimaryLanguage,
					Secondary: deref(r.SecondaryLanguage),
				},
				Signups: int(r.Signups),
			},
			Round:         int(r.Round),
			Window:        Window{Start: r.WindowStart.Time, Days: int(r.WindowDays), Deadline: r.SignupDeadline.Time},
			Venue:         deref(r.VenuePlaceID),
			FollowUpVenue: deref(r.FollowUpVenuePlaceID),
		}
	}
	return out, nil
}

// Create opens a plan for a suggestion and returns its id. It picks a venue
// in the plan's area for the activity and the follow-up from their venue
// categories (acts; a missing activity gets none, as do activities that need
// only a meeting point).
func Create(ctx context.Context, q *dbgen.Queries, cityID string, s matching.Suggestion, w Window, acts map[matching.ActivityID]*matching.Activity) (uuid.UUID, error) {
	venue, err := pickVenue(ctx, q, s.Area, acts[s.Activity])
	if err != nil {
		return uuid.Nil, err
	}
	var followUpVenue *string
	if s.FollowUp != "" {
		if followUpVenue, err = pickVenue(ctx, q, s.Area, acts[s.FollowUp]); err != nil {
			return uuid.Nil, err
		}
	}
	return q.InsertPlan(ctx, dbgen.InsertPlanParams{
		VenuePlaceID:         venue,
		FollowUpVenuePlaceID: followUpVenue,
		CityID:               cityID,
		AreaID:               string(s.Area),
		ActivityID:           string(s.Activity),
		FollowUpID:           ptr(string(s.FollowUp)),
		PrimaryLanguage:      s.Primary,
		SecondaryLanguage:    ptr(s.Secondary),
		Audience:             int32(s.Audience),
		Score:                float32(s.Score),
		WindowStart:          date(w.Start),
		WindowDays:           int16(w.Days),
		SignupDeadline:       pgtype.Timestamptz{Time: w.Deadline, Valid: true},
	})
}

// pickVenue returns a venue for the activity in the area, or nil when it
// needs none or none was discovered there.
func pickVenue(ctx context.Context, q *dbgen.Queries, area matching.AreaID, act *matching.Activity) (*string, error) {
	if act == nil || len(act.VenueCategories) == 0 {
		return nil, nil
	}
	cats := make([]string, len(act.VenueCategories))
	for i, c := range act.VenueCategories {
		cats[i] = string(c)
	}
	id, err := q.PickVenue(ctx, dbgen.PickVenueParams{AreaID: string(area), Categories: cats})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// SaveSignup signs a person up for a plan's current round, or replaces the
// times they gave before. It fails with ErrClosed once the plan has closed.
func SaveSignup(ctx context.Context, pool *pgxpool.Pool, planID uuid.UUID, s matching.Signup) error {
	userID, err := uuid.Parse(string(s.User))
	if err != nil {
		return fmt.Errorf("signup user: %w", err)
	}
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		// Waits for a scheduling pass on the same plan to finish, so a signup
		// lands either before the round is scheduled or in the next one.
		if _, err := q.LockOpenPlan(ctx, planID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrClosed
			}
			return err
		}
		if err := q.UpsertSignup(ctx, dbgen.UpsertSignupParams{PlanID: planID, UserID: userID}); err != nil {
			return err
		}
		key := dbgen.DeleteSignupTimesParams{PlanID: planID, UserID: userID}
		if err := q.DeleteSignupTimes(ctx, key); err != nil {
			return err
		}
		if len(s.Pattern) > 0 {
			p := dbgen.InsertSignupSlotsParams{PlanID: planID, UserID: userID}
			for _, sl := range s.Pattern {
				p.Weekdays = append(p.Weekdays, int16(sl.Weekday))
				p.Blocks = append(p.Blocks, string(sl.Block))
			}
			if err := q.InsertSignupSlots(ctx, p); err != nil {
				return err
			}
		}
		if len(s.Unavailable) > 0 {
			p := dbgen.InsertSignupUnavailableParams{PlanID: planID, UserID: userID}
			for _, o := range s.Unavailable {
				b := string(o.Block)
				if b == "" {
					b = wholeDay
				}
				p.Dates = append(p.Dates, date(o.Date))
				p.Blocks = append(p.Blocks, b)
			}
			if err := q.InsertSignupUnavailable(ctx, p); err != nil {
				return err
			}
		}
		return nil
	})
}

// Withdraw removes a person's signup. Doing it twice is fine.
func Withdraw(ctx context.Context, q *dbgen.Queries, planID, userID uuid.UUID) error {
	return q.DeleteSignup(ctx, dbgen.DeleteSignupParams{PlanID: planID, UserID: userID})
}

// LoadSignups returns a plan's waiting signups, sorted by user, with
// occasions each person is already invited to or going to (from the
// window's start) added to Unavailable, so nobody is double-booked.
func LoadSignups(ctx context.Context, q *dbgen.Queries, planID uuid.UUID, from time.Time) ([]matching.Signup, error) {
	ids, err := q.ListSignups(ctx, planID)
	if err != nil {
		return nil, err
	}
	out := make([]matching.Signup, len(ids))
	byUser := make(map[uuid.UUID]*matching.Signup, len(ids))
	for i, id := range ids {
		out[i].User = matching.UserID(id.String())
		byUser[id] = &out[i]
	}
	slots, err := q.ListSignupSlots(ctx, planID)
	if err != nil {
		return nil, err
	}
	for _, r := range slots {
		s := byUser[r.UserID]
		s.Pattern = append(s.Pattern, matching.Slot{Weekday: time.Weekday(r.Weekday), Block: matching.Block(r.Block)})
	}
	unavailable, err := q.ListSignupUnavailable(ctx, planID)
	if err != nil {
		return nil, err
	}
	for _, r := range unavailable {
		b := matching.Block(r.Block)
		if r.Block == wholeDay {
			b = ""
		}
		s := byUser[r.UserID]
		s.Unavailable = append(s.Unavailable, matching.Occasion{Date: r.Date.Time, Block: b})
	}
	busy, err := q.ListBusyOccasions(ctx, dbgen.ListBusyOccasionsParams{UserIds: ids, FromDate: date(from)})
	if err != nil {
		return nil, err
	}
	for _, r := range busy {
		s := byUser[r.UserID]
		s.Unavailable = append(s.Unavailable, matching.Occasion{Date: r.Date.Time, Block: matching.Block(r.Block)})
	}
	return out, nil
}

// Event is a scheduled round to save.
type Event struct {
	PlanID    uuid.UUID
	Round     int // the plan's round being scheduled; must still be current
	Scheduled matching.Scheduled
	ConfirmBy time.Time
	Forced    bool
	// Next is the plan's next round, for whoever is still waiting; nil
	// closes the plan as done, at Now.
	Next *Window
	Now  time.Time
}

// SaveEvent stores a scheduled round: the event, its groups and their
// invitations. Everyone invited stops waiting; the rest stay signed up for
// the next round, which starts in the same transaction, so a crash can't
// leave a round scheduled but still open. It fails with ErrClosed if the
// plan closed or moved to another round in the meantime.
func SaveEvent(ctx context.Context, pool *pgxpool.Pool, e Event) (uuid.UUID, error) {
	var eventID uuid.UUID
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		round, err := q.LockOpenPlan(ctx, e.PlanID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && int(round) != e.Round) {
			return ErrClosed
		}
		if err != nil {
			return err
		}
		o := e.Scheduled.Occasion
		eventID, err = q.InsertEvent(ctx, dbgen.InsertEventParams{
			PlanID:    e.PlanID,
			Round:     int16(e.Round),
			Date:      date(o.Date),
			Block:     string(o.Block),
			ConfirmBy: pgtype.Timestamptz{Time: e.ConfirmBy, Valid: true},
			Forced:    e.Forced,
		})
		if err != nil {
			return err
		}
		var invited []uuid.UUID
		for _, g := range e.Scheduled.Groups {
			groupID, err := q.InsertEventGroup(ctx, dbgen.InsertEventGroupParams{
				EventID:       eventID,
				Quorum:        int16(g.Quorum),
				MinAttendance: int16(g.MinAttendance),
				Score:         float32(g.Score),
			})
			if err != nil {
				return err
			}
			members := make([]uuid.UUID, len(g.Invited))
			for i, u := range g.Invited {
				if members[i], err = uuid.Parse(string(u)); err != nil {
					return fmt.Errorf("group member: %w", err)
				}
			}
			if err := q.InsertGroupMembers(ctx, dbgen.InsertGroupMembersParams{GroupID: groupID, EventID: eventID, UserIds: members}); err != nil {
				return err
			}
			invited = append(invited, members...)
		}
		if err := q.DeleteSignups(ctx, dbgen.DeleteSignupsParams{PlanID: e.PlanID, UserIds: invited}); err != nil {
			return err
		}
		if e.Next == nil {
			return Close(ctx, q, e.PlanID, ClosedDone, e.Now)
		}
		return NextRound(ctx, q, e.PlanID, *e.Next)
	})
	if err != nil {
		return uuid.Nil, err
	}
	return eventID, nil
}

// NextRound moves a plan to its next round, keeping whoever is still signed up.
func NextRound(ctx context.Context, q *dbgen.Queries, planID uuid.UUID, w Window) error {
	return q.NextPlanRound(ctx, dbgen.NextPlanRoundParams{
		ID:             planID,
		WindowStart:    date(w.Start),
		WindowDays:     int16(w.Days),
		SignupDeadline: pgtype.Timestamptz{Time: w.Deadline, Valid: true},
	})
}

// Close stops a plan taking signups at the given time; reason is one of the
// Closed* constants.
func Close(ctx context.Context, q *dbgen.Queries, planID uuid.UUID, reason string, at time.Time) error {
	return q.ClosePlan(ctx, dbgen.ClosePlanParams{ID: planID, Reason: reason, ClosedAt: pgtype.Timestamptz{Time: at, Valid: true}})
}

func date(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

// ptr maps matching's "" (none) to SQL null.
func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
