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
	"github.com/shrujancheruku/issho/backend/internal/platform/pgconv"
	"github.com/shrujancheruku/issho/backend/internal/platform/sentry"
	"github.com/shrujancheruku/issho/backend/internal/walks"
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
	// Routes finds the walk between a plan's venues; nil leaves plans
	// without routes (no Maps key).
	Routes maps.RouteFinder
	// Places finds meeting places for plans that start without a venue;
	// nil leaves them without (no Maps key).
	Places maps.PlaceSearcher
	// Walks composes walking plans; nil opens none (no Gemini or Maps key):
	// a walk is only worth opening when it's composed.
	Walks *walks.Composer
	// WalkRoutes keeps composed walks' routes within the Maps caching
	// terms; nil clears them without recomputing (no Maps key).
	WalkRoutes maps.WalkRouter
}

// stayChecks is how often, inside a stay, a venue is checked to be open, so
// a break in the middle of a long stay (a lunch closure) is noticed too.
const stayChecks = 30 * time.Minute

// stayTimes are the times to check a venue is open for a stay of length d
// from start: the start, every stayChecks after it, and the last minute
// (the end itself is closing time, not part of the stay).
func stayTimes(start time.Time, d time.Duration) []time.Time {
	last := start.Add(max(d-time.Minute, 0))
	out := []time.Time{start}
	for t := start.Add(stayChecks); t.Before(last); t = t.Add(stayChecks) {
		out = append(out, t)
	}
	if last.After(start) {
		out = append(out, last)
	}
	return out
}

// venueProblem is why a scheduled time doesn't work for a plan's venues.
type venueProblem int

const (
	venuesOK      venueProblem = iota
	venueClosedAt              // closed at that time: try another occasion
	venueGone                  // closed for good or until further notice: withdraw the plan
)

// checkVenues looks up the plan's venues' opening hours (once per venue per
// pass; cache) and says whether they're open around an event starting at
// start: the activity's venue from the start for the activity's own hours,
// then the follow-up's from where the activity ends for its hours (acts is
// the plan's activity and its follow-up; either may be nil, which counts as
// the default length). Lookup failures don't block scheduling: they're logged and the
// venue is assumed open.
func (r *Runner) checkVenues(ctx context.Context, p Open, acts map[matching.ActivityID]*matching.Activity, start time.Time, cache map[string]*maps.Hours) venueProblem {
	if r.Hours == nil {
		return venuesOK
	}
	if p.Walk {
		return r.checkWalk(ctx, p, start, cache)
	}
	first := acts[p.Activity].Length()
	if v := r.checkPlace(ctx, p, p.Venue, stayTimes(start, first), cache); v != venuesOK {
		return v
	}
	return r.checkPlace(ctx, p, p.FollowUpVenue, stayTimes(start.Add(first), acts[p.FollowUp].Length()), cache)
}

// checkPlace says whether a venue ("" = none) is open at every given time.
func (r *Runner) checkPlace(ctx context.Context, p Open, venue string, times []time.Time, cache map[string]*maps.Hours) venueProblem {
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
	for _, t := range times {
		if !h.OpenAt(t) {
			return venueClosedAt
		}
	}
	return venuesOK
}

// SuggestReport counts what a suggesting pass did.
type SuggestReport struct {
	Opened int // plans opened
	Failed int // suggestions that could not be opened
	// Deferred: walks left for the next run because the job's context had
	// less than walks.MinBudget left. Not a failure.
	Deferred int
}

// Suggest loads the city and opens new plans in it, up to its cap
// (matching.Suggest). Match loads the city once and calls suggestIn instead.
func (r *Runner) Suggest(ctx context.Context, cityID string, now time.Time) (SuggestReport, error) {
	c, err := LoadCity(ctx, dbgen.New(r.DB), cityID, 0)
	if err != nil {
		return SuggestReport{}, err
	}
	return r.suggestIn(ctx, c, now)
}

// suggestIn is Suggest for a city loaded with every activity (month 0).
func (r *Runner) suggestIn(ctx context.Context, all *City, now time.Time) (SuggestReport, error) {
	q := dbgen.New(r.DB)
	cityID := all.ID
	w := r.Timing.Round(now, all.Location)
	// Seasonal activities are judged by the month the round starts in.
	c, err := all.InSeason(ctx, q, w.Start.Month())
	if err != nil {
		return SuggestReport{}, err
	}
	open, err := ListOpen(ctx, q, cityID)
	if err != nil {
		return SuggestReport{}, err
	}
	failed, err := q.ListRecentlyFailedPlans(ctx, dbgen.ListRecentlyFailedPlansParams{
		CityID: cityID, Reasons: failedReasons, Since: pgtype.Timestamptz{Time: now.Add(-r.Timing.Cooldown), Valid: true},
	})
	if err != nil {
		return SuggestReport{}, err
	}
	in := matching.SuggestInput{
		Users: c.Users, Areas: c.Areas, Travel: c.Travel, Activities: c.Activities, Venues: c.Venues,
	}
	for _, p := range open {
		in.Open = append(in.Open, p.Plan)
	}
	// Drafts waiting for review count as open: neither repeated nor beyond
	// the city's cap.
	drafts, err := q.ListDraftPlanKeys(ctx, cityID)
	if err != nil {
		return SuggestReport{}, err
	}
	for _, d := range drafts {
		in.Open = append(in.Open, matching.Plan{
			Area: matching.AreaID(d.AreaID), Activity: matching.ActivityID(d.ActivityID),
			FollowUp: matching.ActivityID(pgconv.Deref(d.FollowUpID)), Primary: d.PrimaryLanguage, Secondary: pgconv.Deref(d.SecondaryLanguage),
		})
	}
	// Recently failed plans (and rejected drafts) count as open, so they
	// aren't suggested again straight away. They also take up room under the city's cap until the
	// cool-down ends, which slows reopening plans in a city that's too quiet.
	for _, f := range failed {
		in.Open = append(in.Open, matching.Plan{
			Area: matching.AreaID(f.AreaID), Activity: matching.ActivityID(f.ActivityID),
			FollowUp: matching.ActivityID(pgconv.Deref(f.FollowUpID)), Primary: f.PrimaryLanguage, Secondary: pgconv.Deref(f.SecondaryLanguage),
		})
	}
	return r.suggest(ctx, q, c, in, w, now)
}

// ScheduleReport counts what a scheduling pass did.
type ScheduleReport struct {
	Checked, Scheduled, Closed int
	Failed                     int // plans that could not be checked
}

// Schedule loads the city and checks every open plan in it
// (matching.Schedule): a plan whose signups can make a good group gets an
// event; at its signup deadline it goes ahead with whatever groups can form,
// or closes. Plans with the most signups go first, since each event blocks
// its occasion for the people invited. A plan that fails is logged and
// counted in Failed; the others are still checked (Match has the policy).
// Match loads the city once and calls scheduleIn instead.
func (r *Runner) Schedule(ctx context.Context, cityID string, now time.Time) (ScheduleReport, error) {
	// Any month: a plan opened for a season keeps its round even if the
	// month ends; only disabling the activity withdraws it.
	c, err := LoadCity(ctx, dbgen.New(r.DB), cityID, 0)
	if err != nil {
		return ScheduleReport{}, err
	}
	return r.scheduleIn(ctx, c, now)
}

// scheduleIn is Schedule for a city loaded with every activity (month 0).
func (r *Runner) scheduleIn(ctx context.Context, c *City, now time.Time) (ScheduleReport, error) {
	var rep ScheduleReport
	cityID := c.ID
	q := dbgen.New(r.DB)
	users, acts := c.UsersByID(), c.ActivitiesByID()
	open, err := ListOpen(ctx, q, cityID)
	if err != nil {
		return rep, err
	}
	// Plans whose proposal waits for an admin, or was just rejected, are
	// left alone (00022).
	held, err := q.ListPlansOnHold(ctx, dbgen.ListPlansOnHoldParams{
		CityID: cityID, RejectedSince: pgtype.Timestamptz{Time: now.Add(-ProposalRest), Valid: true},
	})
	if err != nil {
		return rep, err
	}
	open = slices.DeleteFunc(open, func(p Open) bool { return slices.Contains(held, p.ID) })
	slices.SortStableFunc(open, func(a, b Open) int { return cmp.Compare(b.Signups, a.Signups) })
	var errs []error
	for _, p := range open {
		// Cancelled (timeout, shutdown): stop, don't count the rest as failed.
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		rep.Checked++
		closed, scheduled, err := r.schedulePlan(ctx, q, c, users, acts, p, now)
		if err != nil {
			rep.Failed++
			errs = append(errs, fmt.Errorf("plan %s: %w", p.ID, err))
			r.itemFailed(ctx, "schedule", cityID, "plan", p.ID, err)
			continue
		}
		// Counted once the plan is done, not while it is in progress.
		rep.Closed += closed
		rep.Scheduled += scheduled
	}
	if rep.Checked > 0 && rep.Failed == rep.Checked {
		return rep, allFailed("schedule", errs)
	}
	return rep, nil
}

func (r *Runner) schedulePlan(ctx context.Context, q *dbgen.Queries, c *City, users map[matching.UserID]*matching.User,
	acts map[matching.ActivityID]*matching.Activity, p Open, now time.Time) (closed, scheduled int, err error) {
	closeAs := func(reason string) (int, int, error) {
		r.Log.Info("plan closed", "city", c.ID, "plan", p.ID, "reason", reason)
		if reason == ClosedNoGroup {
			// Whoever was still waiting hears; the feed shows them other plans.
			return 1, 0, q.ClosePlanNoGroup(ctx, dbgen.ClosePlanNoGroupParams{ID: p.ID, ClosedAt: pgtype.Timestamptz{Time: now, Valid: true}})
		}
		return 1, 0, Close(ctx, q, p.ID, reason, now)
	}
	// A plan of two activities asks its groups for both (sizes, part of the
	// day): Activity.With.
	act := acts[p.Activity].With(acts[p.FollowUp])
	if acts[p.Activity] == nil {
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
		problem := r.checkVenues(ctx, p, acts, r.Timing.startsAt(res.Occasion, c.Location), hours)
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
		// In a city under review the round waits for an admin (00022).
		Propose:   c.Review,
		StartsAt:  r.Timing.startsAt(res.Occasion, c.Location),
		ConfirmBy: confirmBy(res.Occasion, c.Location),
		Next:      r.Timing.Round(now, c.Location),
	}
	invited := 0
	for _, g := range res.Groups {
		invited += len(g.Invited)
	}
	id, done, err := SaveEvent(ctx, r.DB, ev)
	if errors.Is(err, ErrClosed) {
		// Closed or scheduled by someone else since ListOpen.
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	if c.Review {
		r.Log.Info("round proposed for review", "city", c.ID, "plan", p.ID, "event", id,
			"date", res.Occasion.Date.Format(time.DateOnly), "block", res.Occasion.Block, "groups", len(res.Groups))
		return 0, 1, nil
	}
	r.Log.Info("plan scheduled", "city", c.ID, "plan", p.ID, "event", id, "date", res.Occasion.Date.Format(time.DateOnly),
		"block", res.Occasion.Block, "groups", len(res.Groups), "invited", invited, "waiting", len(signups)-invited, "forced", force)
	if done {
		return 1, 1, nil
	}
	return 0, 1, nil
}

// itemFailed records one item's failure under the policy on Runner.Match:
// logged with its id and reported to Sentry. The caller counts it in Failed
// and carries on.
func (r *Runner) itemFailed(ctx context.Context, stage, cityID, item string, id any, err error) {
	r.Log.Error("item failed", "stage", stage, "city", cityID, item, id, "err", err)
	sentry.Report(ctx, fmt.Errorf("%s %s %v: %w", stage, item, id, err), map[string]string{"stage": stage, "city": cityID})
}

// allFailed is the error for a stage where every item failed: that is
// more likely a shared fault than bad items, so the job is retried.
func allFailed(stage string, errs []error) error {
	return fmt.Errorf("%s: every item failed: %w", stage, errors.Join(errs...))
}

// location returns the city's time zone.
func location(ctx context.Context, q *dbgen.Queries, cityID string) (*time.Location, error) {
	row, err := q.GetCityMatching(ctx, cityID)
	if err != nil {
		return nil, fmt.Errorf("city %q: %w", cityID, err)
	}
	return time.LoadLocation(row.Timezone)
}
