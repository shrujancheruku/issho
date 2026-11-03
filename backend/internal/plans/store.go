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
	"github.com/shrujancheruku/issho/backend/internal/platform/pgconv"
)

// ErrClosed means the plan doesn't exist or no longer takes signups.
var ErrClosed = errors.New("plan is not open")

// ErrSignupsChanged means someone the round would place is no longer signed
// up for the plan (they withdrew after the pass read the signups). Nothing
// was written; the next pass schedules the plan from fresh signups.
var ErrSignupsChanged = errors.New("a signup changed since the pass read them")

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
	// Route is the walk between them; nil until computed (RefreshRoutes).
	Route *Route
	// MeetingPlace is where a walk starts when the first activity has no
	// venue (Google place id); "" otherwise or until found.
	MeetingPlace string
	// Walk is a composed walk (stops in app.plan_stops, LoadWalk), with its
	// title and summary as stored ({"en": "..."}); nil for other plans.
	Walk                   bool
	WalkTitle, WalkSummary []byte
}

// ListOpen returns a city's open plans with their signup counts, oldest
// first. It reads them all: the worker schedules from this list, so it must
// never be cut short (the feed caps its own read, see listFeed).
func ListOpen(ctx context.Context, q *dbgen.Queries, cityID string) ([]Open, error) {
	return listOpen(ctx, q, cityID, nil)
}

func listOpen(ctx context.Context, q *dbgen.Queries, cityID string, max *int32) ([]Open, error) {
	rows, err := q.ListOpenPlans(ctx, dbgen.ListOpenPlansParams{CityID: cityID, Max: max})
	if err != nil {
		return nil, err
	}
	out := make([]Open, len(rows))
	for i, r := range rows {
		out[i] = openFromRow(r)
	}
	return out, nil
}

// openFromRow is a plan as ListOpenPlans (and GetRecentlyClosedPlan, whose
// rows convert) returns it.
func openFromRow(r dbgen.ListOpenPlansRow) Open {
	return Open{
		ID: r.ID,
		OpenPlan: matching.OpenPlan{
			Plan: matching.Plan{
				Area:      matching.AreaID(r.AreaID),
				Activity:  matching.ActivityID(r.ActivityID),
				FollowUp:  matching.ActivityID(pgconv.Deref(r.FollowUpID)),
				Primary:   r.PrimaryLanguage,
				Secondary: pgconv.Deref(r.SecondaryLanguage),
			},
			Signups: int(r.Signups),
		},
		Round:         int(r.Round),
		Window:        Window{Start: r.WindowStart.Time, Days: int(r.WindowDays), Deadline: r.SignupDeadline.Time},
		Venue:         pgconv.Deref(r.VenuePlaceID),
		FollowUpVenue: pgconv.Deref(r.FollowUpVenuePlaceID),
		Route:         routeFrom(r.RoutePolyline, r.RouteSeconds),
		MeetingPlace:  pgconv.Deref(r.MeetingPlaceID),
		Walk:          r.Walk,
		WalkTitle:     r.Title,
		WalkSummary:   r.Description,
	}
}

// Create opens a plan for a suggestion and returns its id. It picks a venue
// in the plan's area for the activity and the follow-up from their venue
// categories (acts; a missing activity gets none, as do activities that need
// only a meeting point).
func Create(ctx context.Context, q *dbgen.Queries, cityID string, s matching.Suggestion, w Window, acts map[matching.ActivityID]*matching.Activity) (uuid.UUID, error) {
	return create(ctx, q, cityID, s, w, acts, PlanOpen)
}

// create is Create with the plan's status: open, or a draft in a city
// under review.
func create(ctx context.Context, q *dbgen.Queries, cityID string, s matching.Suggestion, w Window, acts map[matching.ActivityID]*matching.Activity, status string) (uuid.UUID, error) {
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
		Status:               status,
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

// Withdraw removes a person's signup. Doing it twice is fine. Run it in a
// transaction (WithdrawSignup starts one): it takes the plan's lock like
// SaveSignup, so a withdrawal waits for a scheduling pass on the plan
// instead of interleaving with it (the pass would otherwise place or invite
// someone who has just withdrawn). A plan that has closed holds its signups
// as a record, so there is nothing to withdraw from.
func Withdraw(ctx context.Context, q *dbgen.Queries, planID, userID uuid.UUID) error {
	if _, err := q.LockOpenPlan(ctx, planID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return err
	}
	return q.DeleteSignup(ctx, dbgen.DeleteSignupParams{PlanID: planID, UserID: userID})
}

// WithdrawSignup is Withdraw in its own transaction.
func WithdrawSignup(ctx context.Context, pool *pgxpool.Pool, planID, userID uuid.UUID) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		return Withdraw(ctx, dbgen.New(tx), planID, userID)
	})
}

// LoadSignups returns a plan's waiting signups, sorted by user, with
// occasions each person is already invited to or going to (from the
// window's start) added to Unavailable, so nobody is double-booked.
func LoadSignups(ctx context.Context, q *dbgen.Queries, planID uuid.UUID, from time.Time) ([]matching.Signup, error) {
	waiting, err := LoadWaiting(ctx, q, []uuid.UUID{planID})
	if err != nil {
		return nil, err
	}
	signups := waiting[planID]
	if signups == nil {
		signups = []matching.Signup{} // as before: empty, not nil
	}
	return AddBusy(ctx, q, signups, from)
}

// LoadWaiting returns the waiting signups of several plans, sorted by user
// and keyed by plan, with the weekly pattern and the dates each person
// stored for that plan: three queries however many plans (#185). It is
// LoadSignups without the person's other commitments (AddBusy), which change
// as plans are scheduled and so are read per plan, when its turn comes.
func LoadWaiting(ctx context.Context, q *dbgen.Queries, planIDs []uuid.UUID) (map[uuid.UUID][]matching.Signup, error) {
	type key struct{ plan, user uuid.UUID }
	rows, err := q.ListPlansSignups(ctx, planIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]matching.Signup, len(planIDs))
	for _, r := range rows {
		out[r.PlanID] = append(out[r.PlanID], matching.Signup{User: matching.UserID(r.UserID.String())})
	}
	// Rows arrive sorted by plan then user, so a person's index in their
	// plan's list is found by counting as the list is filled.
	index := make(map[key]int, len(rows))
	next := map[uuid.UUID]int{}
	for _, r := range rows {
		index[key{r.PlanID, r.UserID}] = next[r.PlanID]
		next[r.PlanID]++
	}
	// nil for a person who withdrew between the queries (they are not in the
	// first result); their rows are skipped.
	signup := func(plan, user uuid.UUID) *matching.Signup {
		i, ok := index[key{plan, user}]
		if !ok {
			return nil
		}
		return &out[plan][i]
	}
	slots, err := q.ListPlansSignupSlots(ctx, planIDs)
	if err != nil {
		return nil, err
	}
	for _, r := range slots {
		s := signup(r.PlanID, r.UserID)
		if s == nil {
			continue
		}
		s.Pattern = append(s.Pattern, matching.Slot{Weekday: time.Weekday(r.Weekday), Block: matching.Block(r.Block)})
	}
	unavailable, err := q.ListPlansSignupUnavailable(ctx, planIDs)
	if err != nil {
		return nil, err
	}
	for _, r := range unavailable {
		b := matching.Block(r.Block)
		if r.Block == wholeDay {
			b = ""
		}
		s := signup(r.PlanID, r.UserID)
		if s == nil {
			continue
		}
		s.Unavailable = append(s.Unavailable, matching.Occasion{Date: r.Date.Time, Block: b})
	}
	return out, nil
}

// AddBusy adds to each signup's Unavailable the occasions their person is
// already invited to or going to, from the given date, so nobody is
// double-booked. It changes signups in place and returns them.
func AddBusy(ctx context.Context, q *dbgen.Queries, signups []matching.Signup, from time.Time) ([]matching.Signup, error) {
	ids := make([]uuid.UUID, len(signups))
	byUser := make(map[uuid.UUID]*matching.Signup, len(signups))
	for i := range signups {
		id, err := uuid.Parse(string(signups[i].User))
		if err != nil {
			return nil, err
		}
		ids[i] = id
		byUser[id] = &signups[i]
	}
	busy, err := q.ListBusyOccasions(ctx, dbgen.ListBusyOccasionsParams{UserIds: ids, FromDate: date(from)})
	if err != nil {
		return nil, err
	}
	for _, r := range busy {
		s := byUser[r.UserID]
		s.Unavailable = append(s.Unavailable, matching.Occasion{Date: r.Date.Time, Block: matching.Block(r.Block)})
	}
	return signups, nil
}

// Event is a scheduled round to save.
type Event struct {
	PlanID    uuid.UUID
	Round     int // the plan's round being scheduled; must still be current
	Scheduled matching.Scheduled
	StartsAt  time.Time // zero = not set
	ConfirmBy time.Time
	Forced    bool
	// Next is the plan's next round, if anyone is still waiting once the
	// invitations go out (startRound); otherwise the plan closes as done,
	// at Now.
	Next Window
	Now  time.Time
	// Propose saves the round for an admin to review (a city under review,
	// 00022): the event and its groups, but nobody is invited or told, and
	// the plan stays in its round with everyone signed up. ApproveProposal
	// does the rest; Next is ignored.
	Propose bool
}

// ProposalRest is how long the scheduler leaves a plan alone after an admin
// rejects its proposal.
const ProposalRest = 24 * time.Hour

// Event states (app.events.status).
const (
	EventProposed  = "proposed"
	EventScheduled = "scheduled"
	EventDone      = "done"
	EventCancelled = "cancelled"
)

// SaveEvent stores a scheduled round: the event, its groups and their
// invitations, which go out (startRound) in the same transaction, so a
// crash can't leave a round scheduled but still open. closed is whether the
// plan closed as done, with nobody left waiting. It fails with ErrClosed if
// the plan closed or moved to another round in the meantime.
func SaveEvent(ctx context.Context, pool *pgxpool.Pool, e Event) (eventID uuid.UUID, closed bool, err error) {
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		round, err := q.LockOpenPlan(ctx, e.PlanID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && int(round) != e.Round) {
			return ErrClosed
		}
		if err != nil {
			return err
		}
		// The pass read the plan's signups once, up front, so check under the
		// lock (which Withdraw and SaveSignup take too) that everyone it is
		// about to invite is still waiting. Before anything is written, so
		// nothing is invited or sent for a stale list.
		waiting, err := q.ListSignups(ctx, e.PlanID)
		if err != nil {
			return err
		}
		still := make(map[uuid.UUID]bool, len(waiting))
		for _, u := range waiting {
			still[u] = true
		}
		for _, g := range e.Scheduled.Groups {
			for _, u := range g.Invited {
				if id, err := uuid.Parse(string(u)); err != nil || !still[id] {
					return ErrSignupsChanged
				}
			}
		}
		o := e.Scheduled.Occasion
		status := EventScheduled
		if e.Propose {
			status = EventProposed
		}
		eventID, err = q.InsertEvent(ctx, dbgen.InsertEventParams{
			PlanID:    e.PlanID,
			Round:     int16(e.Round),
			Date:      date(o.Date),
			Block:     string(o.Block),
			StartsAt:  pgtype.Timestamptz{Time: e.StartsAt, Valid: !e.StartsAt.IsZero()},
			ConfirmBy: pgtype.Timestamptz{Time: e.ConfirmBy, Valid: true},
			Forced:    e.Forced,
			Status:    status,
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
		if e.Propose {
			return nil // ApproveProposal invites them and moves the plan on
		}
		closed, err = startRound(ctx, q, e.PlanID, eventID, invited, func() (Window, error) { return e.Next, nil }, e.Now)
		return err
	})
	if err != nil {
		return uuid.Nil, false, err
	}
	return eventID, closed, nil
}

func date(t time.Time) pgtype.Date { return pgtype.Date{Time: t, Valid: true} }

// ptr maps matching's "" (none) to SQL null.
func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
