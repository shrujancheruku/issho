package plans

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// Timing sets when a round can happen. Dates are the city's local dates.
type Timing struct {
	// LeadDays: the earliest possible occasion is this many days after
	// today, so invitees have at least a day to confirm.
	LeadDays int
	// WindowDays: how many days a round's occasions span.
	WindowDays int
	// SignupDays: the signup deadline is this many days into the window.
	// At the deadline the round is scheduled with whoever can go, on a date
	// at least LeadDays later, so it must leave room for that.
	SignupDays int
	// Cooldown: a plan that closed without a group isn't suggested again
	// for this long.
	Cooldown time.Duration
	// BlockStarts is when an event starts in each part of the day, local
	// time since midnight (e.g. 14h for the afternoon). The venue's hours
	// aren't known in advance, so this is a sensible default people can
	// plan around; the meeting time can be refined later.
	BlockStarts map[matching.Block]time.Duration
}

// DefaultTiming: rounds open for signups for a week, then pick a date in
// the week after.
func DefaultTiming() Timing {
	return Timing{
		LeadDays: 2, WindowDays: 14, SignupDays: 7, Cooldown: 14 * 24 * time.Hour,
		BlockStarts: map[matching.Block]time.Duration{
			matching.Morning:   10 * time.Hour,
			matching.Afternoon: 14 * time.Hour,
			matching.Evening:   18*time.Hour + 30*time.Minute,
		},
	}
}

// Validate reports a timing that would leave no occasion at the deadline.
func (t Timing) Validate() error {
	if t.LeadDays < 1 || t.SignupDays < 1 || t.SignupDays+t.LeadDays >= t.WindowDays {
		return fmt.Errorf("timing: need lead ≥ 1, signup ≥ 1 and signup + lead < window days (got %d, %d, %d)",
			t.LeadDays, t.SignupDays, t.WindowDays)
	}
	for _, b := range matching.Blocks {
		if d, ok := t.BlockStarts[b]; !ok || d < 0 || d >= 24*time.Hour {
			return fmt.Errorf("timing: no start time of day for %s", b)
		}
	}
	return nil
}

// Round returns the window of a round opened at now in the given city time
// zone: it starts LeadDays from today, and signups close at the start of
// day SignupDays.
func (t Timing) Round(now time.Time, loc *time.Location) Window {
	start := today(now, loc).AddDate(0, 0, t.LeadDays)
	d := start.AddDate(0, 0, t.SignupDays)
	return Window{
		Start:    start,
		Days:     t.WindowDays,
		Deadline: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc),
	}
}

// occasions returns the round's occasions still far enough ahead at now.
func (t Timing) occasions(w Window, now time.Time, loc *time.Location) []matching.Occasion {
	earliest := today(now, loc).AddDate(0, 0, t.LeadDays)
	var out []matching.Occasion
	for _, o := range w.Occasions() {
		if !o.Date.Before(earliest) {
			out = append(out, o)
		}
	}
	return out
}

// startsAt is when an event on the occasion starts, in the city's time zone.
// Built from the clock time rather than midnight + duration, so a daylight
// saving change that day doesn't shift it.
func (t Timing) startsAt(o matching.Occasion, loc *time.Location) time.Time {
	d := t.BlockStarts[o.Block]
	h, m := int(d/time.Hour), int(d%time.Hour/time.Minute)
	return time.Date(o.Date.Year(), o.Date.Month(), o.Date.Day(), h, m, 0, 0, loc)
}

// confirmBy is the response deadline for an event: the start of the day
// before it, local time.
func confirmBy(o matching.Occasion, loc *time.Location) time.Time {
	d := o.Date.AddDate(0, 0, -1)
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc)
}

// today is the city's local date at now, as midnight UTC (matching's dates).
func today(now time.Time, loc *time.Location) time.Time {
	l := now.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC)
}

// Runner runs the matching passes for a city against the database.
type Runner struct {
	DB     *pgxpool.Pool
	Timing Timing
	Log    *slog.Logger
	// Hours checks venues are open before a round is scheduled; nil skips
	// the check (no Maps key).
	Hours maps.HoursLookup
}

// When venues must be open, from an event's start. Activities have no
// stored duration, so this assumes about two hours, then the follow-up.
var (
	activityOpen = []time.Duration{0, 90 * time.Minute}
	followUpOpen = []time.Duration{2 * time.Hour, 3 * time.Hour}
)

// venueProblem is why a scheduled time doesn't work for a plan's venues.
type venueProblem int

const (
	venuesOK      venueProblem = iota
	venueClosedAt              // closed at that time: try another occasion
	venueGone                  // closed for good or until further notice: withdraw the plan
)

// checkVenues looks up the plan's venues' opening hours (once per venue per
// pass; cache) and says whether they're open around an event starting at
// start. Lookup failures don't block scheduling: they're logged and the
// venue is assumed open.
func (r *Runner) checkVenues(ctx context.Context, p Open, start time.Time, cache map[string]*maps.Hours) venueProblem {
	if r.Hours == nil {
		return venuesOK
	}
	check := func(venue string, offsets []time.Duration) venueProblem {
		if venue == "" {
			return venuesOK
		}
		h, seen := cache[venue]
		if !seen {
			got, err := r.Hours.Hours(ctx, venue)
			if err != nil {
				r.Log.Warn("venue hours", "plan", p.ID, "place", venue, "err", err)
			} else {
				h = &got
			}
			cache[venue] = h
		}
		if h == nil {
			return venuesOK
		}
		if h.Closed {
			if h.PermanentlyClosed {
				if err := dbgen.New(r.DB).RejectVenue(ctx, venue); err != nil {
					r.Log.Warn("reject closed venue", "place", venue, "err", err)
				}
			}
			return venueGone
		}
		for _, o := range offsets {
			if !h.OpenAt(start.Add(o)) {
				return venueClosedAt
			}
		}
		return venuesOK
	}
	if v := check(p.Venue, activityOpen); v != venuesOK {
		return v
	}
	return check(p.FollowUpVenue, followUpOpen)
}

// Suggest opens new plans in a city, up to its cap (matching.Suggest),
// and returns how many it opened.
func (r *Runner) Suggest(ctx context.Context, cityID string, now time.Time) (int, error) {
	q := dbgen.New(r.DB)
	loc, err := location(ctx, q, cityID)
	if err != nil {
		return 0, err
	}
	w := r.Timing.Round(now, loc)
	// Seasonal activities are judged by the month the round starts in.
	c, err := LoadCity(ctx, q, cityID, w.Start.Month())
	if err != nil {
		return 0, err
	}
	open, err := ListOpen(ctx, q, cityID)
	if err != nil {
		return 0, err
	}
	failed, err := q.ListRecentlyFailedPlans(ctx, dbgen.ListRecentlyFailedPlansParams{
		CityID: cityID, Since: pgtype.Timestamptz{Time: now.Add(-r.Timing.Cooldown), Valid: true},
	})
	if err != nil {
		return 0, err
	}
	in := matching.SuggestInput{
		Users: c.Users, Areas: c.Areas, Travel: c.Travel, Activities: c.Activities, Venues: c.Venues,
	}
	for _, p := range open {
		in.Open = append(in.Open, p.Plan)
	}
	// Recently failed plans count as open, so they aren't suggested again
	// straight away. They also take up room under the city's cap until the
	// cool-down ends, which slows reopening plans in a city that's too quiet.
	for _, f := range failed {
		in.Open = append(in.Open, matching.Plan{
			Area: matching.AreaID(f.AreaID), Activity: matching.ActivityID(f.ActivityID),
			FollowUp: matching.ActivityID(deref(f.FollowUpID)), Primary: f.PrimaryLanguage, Secondary: deref(f.SecondaryLanguage),
		})
	}
	suggestions := matching.Suggest(in, c.Config)
	acts := make(map[matching.ActivityID]*matching.Activity, len(c.Activities))
	for i := range c.Activities {
		acts[c.Activities[i].ID] = &c.Activities[i]
	}
	for _, s := range suggestions {
		if _, err := Create(ctx, q, cityID, s, w, acts); err != nil {
			return 0, fmt.Errorf("open plan %s in %s: %w", s.Activity, s.Area, err)
		}
	}
	return len(suggestions), nil
}

// ScheduleReport counts what a scheduling pass did.
type ScheduleReport struct {
	Checked, Scheduled, Closed int
}

// Schedule checks every open plan in a city (matching.Schedule): a plan
// whose signups can make a good group gets an event; at its signup deadline
// it goes ahead with whatever groups can form, or closes. Plans with the
// most signups go first, since each event blocks its occasion for the
// people invited.
func (r *Runner) Schedule(ctx context.Context, cityID string, now time.Time) (ScheduleReport, error) {
	var rep ScheduleReport
	// Any month: a plan opened for a season keeps its round even if the
	// month ends; only disabling the activity withdraws it.
	q := dbgen.New(r.DB)
	c, err := LoadCity(ctx, q, cityID, 0)
	if err != nil {
		return rep, err
	}
	users := make(map[matching.UserID]*matching.User, len(c.Users))
	for i := range c.Users {
		users[c.Users[i].ID] = &c.Users[i]
	}
	acts := make(map[matching.ActivityID]*matching.Activity, len(c.Activities))
	for i := range c.Activities {
		acts[c.Activities[i].ID] = &c.Activities[i]
	}
	open, err := ListOpen(ctx, q, cityID)
	if err != nil {
		return rep, err
	}
	slices.SortStableFunc(open, func(a, b Open) int { return cmp.Compare(b.Signups, a.Signups) })
	for _, p := range open {
		rep.Checked++
		closed, scheduled, err := r.schedulePlan(ctx, q, c, users, acts, p, now)
		if err != nil {
			return rep, fmt.Errorf("plan %s: %w", p.ID, err)
		}
		rep.Closed += closed
		rep.Scheduled += scheduled
	}
	return rep, nil
}

func (r *Runner) schedulePlan(ctx context.Context, q *dbgen.Queries, c *City, users map[matching.UserID]*matching.User,
	acts map[matching.ActivityID]*matching.Activity, p Open, now time.Time) (closed, scheduled int, err error) {
	closeAs := func(reason string) (int, int, error) {
		r.Log.Info("plan closed", "city", c.ID, "plan", p.ID, "reason", reason)
		return 1, 0, Close(ctx, q, p.ID, reason, now)
	}
	act := acts[p.Activity]
	if act == nil {
		// Disabled in the city since the plan opened.
		return closeAs(ClosedWithdrawn)
	}
	window := r.Timing.occasions(p.Window, now, c.Location)
	force := !now.Before(p.Window.Deadline) || len(window) == 0
	signups, err := LoadSignups(ctx, q, p.ID, p.Window.Start)
	if err != nil {
		return 0, 0, err
	}
	// People who left the city or deleted their profile since signing up.
	signups = slices.DeleteFunc(signups, func(s matching.Signup) bool { return users[s.User] == nil })
	if len(signups) == 0 || len(window) == 0 {
		if force {
			return closeAs(ClosedNoGroup)
		}
		return 0, 0, nil
	}
	// Schedule, then check the venues are open then; if not, take that
	// occasion out of the window and schedule again.
	hours := map[string]*maps.Hours{}
	var res matching.Scheduled
	for {
		var ok bool
		res, ok = matching.Schedule(matching.ScheduleInput{
			Plan: p.Plan, Activity: act, Signups: signups, Users: users, Relations: c.Relations, Window: window,
		}, c.Config, force)
		if !ok {
			if force {
				return closeAs(ClosedNoGroup)
			}
			return 0, 0, nil
		}
		problem := r.checkVenues(ctx, p, r.Timing.startsAt(res.Occasion, c.Location), hours)
		if problem == venuesOK {
			break
		}
		if problem == venueGone {
			r.Log.Info("venue closed, plan withdrawn", "city", c.ID, "plan", p.ID)
			return closeAs(ClosedWithdrawn)
		}
		window = slices.DeleteFunc(window, func(o matching.Occasion) bool {
			return o.Date.Equal(res.Occasion.Date) && o.Block == res.Occasion.Block
		})
		if len(window) == 0 {
			// No time in the round suits the venues.
			if force {
				return closeAs(ClosedNoGroup)
			}
			return 0, 0, nil
		}
	}
	ev := Event{
		PlanID: p.ID, Round: p.Round, Scheduled: res, Forced: force, Now: now,
		StartsAt:  r.Timing.startsAt(res.Occasion, c.Location),
		ConfirmBy: confirmBy(res.Occasion, c.Location),
	}
	invited := 0
	for _, g := range res.Groups {
		invited += len(g.Invited)
	}
	if invited < len(signups) {
		next := r.Timing.Round(now, c.Location)
		ev.Next = &next
	}
	id, err := SaveEvent(ctx, r.DB, ev)
	if errors.Is(err, ErrClosed) {
		// Closed or scheduled by someone else since ListOpen.
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	r.Log.Info("plan scheduled", "city", c.ID, "plan", p.ID, "event", id, "date", res.Occasion.Date.Format(time.DateOnly),
		"block", res.Occasion.Block, "groups", len(res.Groups), "invited", invited, "waiting", len(signups)-invited, "forced", force)
	if ev.Next == nil {
		return 1, 1, nil
	}
	return 0, 1, nil
}

// location returns the city's time zone.
func location(ctx context.Context, q *dbgen.Queries, cityID string) (*time.Location, error) {
	row, err := q.GetCityMatching(ctx, cityID)
	if err != nil {
		return nil, fmt.Errorf("city %q: %w", cityID, err)
	}
	return time.LoadLocation(row.Timezone)
}
