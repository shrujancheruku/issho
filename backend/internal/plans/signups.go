package plans

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/cityclock"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/plans/store"
	"github.com/shrujancheruku/issho/backend/internal/policy"
)

// The ways a signup is refused. ErrNotFound (a plan that isn't there, or whose
// city is closed) is plans' own, in plans.go.
var (
	// ErrInvalid wraps a refusal of malformed ticks: none, a date that isn't a
	// date or a part of the day that isn't one.
	ErrInvalid = errors.New("plans: invalid signup")
	// ErrTickTooSoon: a tick less than policy.MinTickLeadDays ahead, counted on
	// the city's clock.
	ErrTickTooSoon = errors.New("plans: a tick is less than 3 days ahead")
	// ErrTickTooFar: a tick later than a round (policy.RoundLength) after the
	// round's last day.
	ErrTickTooFar = errors.New("plans: a tick is too far ahead")
	// ErrTickPartNotAllowed: a tick on a part of the day the plan doesn't allow.
	ErrTickPartNotAllowed = errors.New("plans: the plan doesn't run in that part of the day")
)

// Tick is one part of one day a person can go: "2026-01-08" and "evening".
// The date is the city's calendar date and the part is in the city's clock
// hours.
type Tick struct {
	Date string
	Part string
}

// Signup is a person's ticks on a plan in a round of it. It commits the
// person to nothing.
type Signup struct {
	PlanID string
	// Ticks are in time order: by date, then morning, afternoon, evening.
	Ticks []Tick
	// HearBy is the round's last day, the city's calendar date by which the
	// person has an invitation or "no group this round" (docs/spec/matching.md).
	HearBy string
}

// Signups keeps who is signed up to which plan, with their ticks. It is part of
// plans' exported API with Service, and reads the same tables.
type Signups struct {
	clock  clock.Clock
	pool   *pgxpool.Pool
	cities *cities.Service
}

// NewSignups returns a Signups that reads "today" from clk on each city's clock.
func NewSignups(clk clock.Clock, pool *pgxpool.Pool, cs *cities.Service) *Signups {
	return &Signups{clock: clk, pool: pool, cities: cs}
}

// noParts: signups need only the city's calendar date, not its hours of the
// day, and cityclock accepts an empty span (as people's profile does).
var noParts = cityclock.DayParts{
	Morning:   cityclock.Span{From: "00:00", To: "00:00"},
	Afternoon: cityclock.Span{From: "00:00", To: "00:00"},
	Evening:   cityclock.Span{From: "00:00", To: "00:00"},
}

// partRank orders a day's parts.
var partRank = map[string]int{"morning": 0, "afternoon": 1, "evening": 2}

// roundDays is how many calendar days a round spans, its first and last
// included.
const roundDays = int(policy.RoundLength / (24 * time.Hour))

// cityToday is the date on the city's clock now, as midnight UTC.
func cityToday(clk clock.Clock, zone string) (time.Time, error) {
	cc, err := cityclock.New(zone, noParts)
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.DateOnly, cc.Date(clk.Now()))
}

// opened is whether a plan that opened on opensOn has opened by today, both
// city calendar dates: its opening day is its first day.
func opened(opensOn, today time.Time) bool { return !today.Before(opensOn) }

// roundOf is the round of a plan that opened on opensOn that holds today, both
// city calendar dates: round k covers opensOn + 7k to opensOn + 7k + 6. open is
// false before the plan's opening day, when it has no round.
func roundOf(opensOn, today time.Time) (number int32, starts, ends time.Time, open bool) {
	if !opened(opensOn, today) {
		return 0, time.Time{}, time.Time{}, false
	}
	days := int(today.Sub(opensOn) / (24 * time.Hour))
	number = int32(days / roundDays)
	starts = addDays(opensOn, int(number)*roundDays)
	return number, starts, addDays(starts, roundDays-1), true
}

// addDays is the calendar date n days after date (negative for before).
func addDays(date time.Time, n int) time.Time { return date.AddDate(0, 0, n) }

// Set makes the account's signup to the plan from ticks, or replaces the ticks
// of the one it has: a person changes their times by signing up again. The
// round is the plan's 7-day block that holds today (see roundOf).
func (s *Signups) Set(ctx context.Context, accountID, planID string, ticks []Tick) (Signup, error) {
	plan, city, err := s.planAndCity(ctx, planID)
	if err != nil {
		return Signup{}, err
	}
	today, err := cityToday(s.clock, city.TimeZone)
	if err != nil {
		return Signup{}, fmt.Errorf("plans: signup: %w", err)
	}
	// A plan that isn't shown (a closed city's, or one that hasn't opened yet)
	// takes no signups; the error is the one for a plan that isn't there, as
	// GetPlan's is.
	if !shown(city, plan.OpensOn, today) {
		return Signup{}, ErrNotFound
	}
	norm, err := parseTicks(ticks)
	if err != nil {
		return Signup{}, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Signup{}, fmt.Errorf("plans: signup: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	q := store.New(tx)

	// The round is the 7-day block of the plan's days that holds today. Its
	// dates are derived, so everyone signing up in the block shares it; the row
	// is stored by whoever signs up first, and a race for it stores one.
	number, starts, ends, _ := roundOf(plan.OpensOn, today)
	round, err := q.GetRound(ctx, store.GetRoundParams{PlanID: plan.ID, Number: number})
	if errors.Is(err, pgx.ErrNoRows) {
		round, err = q.OpenRound(ctx, store.OpenRoundParams{PlanID: plan.ID, Number: number, StartsOn: starts, EndsOn: ends})
	}
	if err != nil {
		return Signup{}, fmt.Errorf("plans: signup: round: %w", err)
	}

	if err := checkTicks(norm, plan.PartsOfDay, today, round.EndsOn); err != nil {
		return Signup{}, err
	}

	now := s.clock.Now()
	if err := q.UpsertSignup(ctx, store.UpsertSignupParams{RoundID: round.ID, AccountID: accountID, CreatedAt: now}); err != nil {
		return Signup{}, fmt.Errorf("plans: signup: %w", err)
	}
	if err := q.DeleteTicks(ctx, store.DeleteTicksParams{RoundID: round.ID, AccountID: accountID}); err != nil {
		return Signup{}, fmt.Errorf("plans: signup: %w", err)
	}
	for _, t := range norm {
		d, _ := time.Parse(time.DateOnly, t.Date)
		if err := q.InsertTick(ctx, store.InsertTickParams{RoundID: round.ID, AccountID: accountID, TickDate: d, Part: t.Part}); err != nil {
			return Signup{}, fmt.Errorf("plans: signup: tick: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Signup{}, fmt.Errorf("plans: signup: %w", err)
	}
	return Signup{PlanID: plan.ID, Ticks: norm, HearBy: round.EndsOn.Format(time.DateOnly)}, nil
}

// Entry is one person's signup to a round as forming reads it: who, and the
// times they ticked. It carries nothing else of the person.
type Entry struct {
	Account string
	// Ticks are in time order.
	Ticks []Tick
}

// FormingRound is a plan's running round as forming groups reads it
// (docs/spec/architecture.md: "plans and outings load matching's inputs").
type FormingRound struct {
	PlanID string
	// RoundID is the stored round's id; empty while nobody has signed up to the
	// round, which has no row yet.
	RoundID string
	// StartsAt is the plan's start, "HH:MM" on the city's clock.
	StartsAt string
	// Entries are the signups with at least one tick that is still
	// policy.MinTickLeadDays ahead, one per person, by account.
	Entries []Entry
}

// Forming reads the plan's running round for forming groups: the round that
// holds today on the city's calendar. An outing's date is never less than
// policy.MinTickLeadDays after its invitations go out, so only the ticks that
// far ahead *now* are read, and a person with none left isn't an entry. A plan
// that isn't shown is ErrNotFound, as for Set.
func (s *Signups) Forming(ctx context.Context, planID string) (FormingRound, error) {
	plan, city, err := s.planAndCity(ctx, planID)
	if err != nil {
		return FormingRound{}, err
	}
	today, err := cityToday(s.clock, city.TimeZone)
	if err != nil {
		return FormingRound{}, fmt.Errorf("plans: forming: %w", err)
	}
	if !shown(city, plan.OpensOn, today) {
		return FormingRound{}, ErrNotFound
	}
	out := FormingRound{PlanID: plan.ID, StartsAt: plan.StartsAt}
	number, _, _, _ := roundOf(plan.OpensOn, today)
	q := store.New(s.pool)
	round, err := q.GetRound(ctx, store.GetRoundParams{PlanID: plan.ID, Number: number})
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return FormingRound{}, fmt.Errorf("plans: forming: round: %w", err)
	}
	out.RoundID = round.ID
	rows, err := q.ListRoundTicks(ctx, store.ListRoundTicksParams{RoundID: round.ID, FromDate: addDays(today, policy.MinTickLeadDays)})
	if err != nil {
		return FormingRound{}, fmt.Errorf("plans: forming: ticks: %w", err)
	}
	for _, r := range rows {
		if n := len(out.Entries); n == 0 || out.Entries[n-1].Account != r.AccountID {
			out.Entries = append(out.Entries, Entry{Account: r.AccountID})
		}
		e := &out.Entries[len(out.Entries)-1]
		e.Ticks = append(e.Ticks, Tick{Date: r.TickDate.Format(time.DateOnly), Part: r.Part})
	}
	return out, nil
}

// ReleaseTime takes one time, a date and a part of the day, off the account's
// signups to every round but exceptRound, in the caller's transaction: a person
// who is going to an outing can't be grouped for the same time on another plan
// (docs/spec/outings.md, "Going"; docs/spec/core-loop.md). exceptRound is the
// outing's own round, whose signup is what formed the group. A signup left with
// no tick is withdrawn, since a signup always has one. The time is a calendar
// date ("2006-01-02") and a part of the day as in Tick; a person with no such
// tick changes nothing and gets no error.
//
// It takes a transaction so that `outings` can make a person going and take the
// time off in one commit; it writes only plans' own tables.
func (s *Signups) ReleaseTime(ctx context.Context, tx pgx.Tx, accountID, date, part, exceptRound string) error {
	day, err := time.Parse(time.DateOnly, date)
	if err != nil {
		return fmt.Errorf("%w: date %q", ErrInvalid, date)
	}
	q := store.New(tx)
	if err := q.DeleteOtherTicksAt(ctx, store.DeleteOtherTicksAtParams{AccountID: accountID, TickDate: day, Part: part, RoundID: exceptRound}); err != nil {
		return fmt.Errorf("plans: release time: %w", err)
	}
	if err := q.DeleteEmptySignups(ctx, store.DeleteEmptySignupsParams{AccountID: accountID, RoundID: exceptRound}); err != nil {
		return fmt.Errorf("plans: release time: signups: %w", err)
	}
	return nil
}

// cityDates answers "what day is it" for the cities the signups are in, each
// on its own clock, reading each city once.
type cityDates struct {
	s     *Signups
	ctx   context.Context
	dates map[string]time.Time
}

func (s *Signups) cityDates(ctx context.Context) *cityDates {
	return &cityDates{s: s, ctx: ctx, dates: map[string]time.Time{}}
}

// today is the city's calendar date now, at midnight UTC.
func (c *cityDates) today(slug string) (time.Time, error) {
	if d, ok := c.dates[slug]; ok {
		return d, nil
	}
	city, err := c.s.cities.Get(c.ctx, slug)
	if err != nil {
		return time.Time{}, err
	}
	d, err := cityToday(c.s.clock, city.TimeZone)
	if err != nil {
		return time.Time{}, err
	}
	c.dates[slug] = d
	return d, nil
}

// since is the loose lower bound for reads: a day before the UTC date, which no
// city's today is earlier than.
func (s *Signups) since() time.Time {
	now := s.clock.Now().UTC()
	return addDays(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), -1)
}

// Mine returns the account's signups to rounds that haven't ended, the
// earliest hear-by first. Its own, and nobody else's: it takes the account.
func (s *Signups) Mine(ctx context.Context, accountID string) ([]Signup, error) {
	q := store.New(s.pool)
	rows, err := q.ListSignupsOfAccount(ctx, store.ListSignupsOfAccountParams{AccountID: accountID, Since: s.since()})
	if err != nil {
		return nil, fmt.Errorf("plans: mine: %w", err)
	}
	ticks, err := q.ListTicksOfAccount(ctx, store.ListTicksOfAccountParams{AccountID: accountID, Since: s.since()})
	if err != nil {
		return nil, fmt.Errorf("plans: mine: %w", err)
	}
	type key struct {
		plan string
		ends time.Time
	}
	byRound := map[key][]Tick{}
	for _, t := range ticks {
		k := key{t.PlanID, t.EndsOn}
		byRound[k] = append(byRound[k], Tick{Date: t.TickDate.Format(time.DateOnly), Part: t.Part})
	}
	dates := s.cityDates(ctx)
	out := []Signup{}
	for _, r := range rows {
		today, err := dates.today(r.City)
		if err != nil {
			return nil, fmt.Errorf("plans: mine: %w", err)
		}
		if r.EndsOn.Before(today) {
			continue
		}
		out = append(out, Signup{
			PlanID: r.PlanID,
			Ticks:  normalise(byRound[key{r.PlanID, r.EndsOn}]),
			HearBy: r.EndsOn.Format(time.DateOnly),
		})
	}
	return out, nil
}

// Withdraw ends the account's signup to the plan. A person with none withdraws
// nothing and gets no error. A closed city's signups can be withdrawn too.
func (s *Signups) Withdraw(ctx context.Context, accountID, planID string) error {
	plan, city, err := s.planAndCity(ctx, planID)
	if err != nil {
		return err
	}
	today, err := cityToday(s.clock, city.TimeZone)
	if err != nil {
		return fmt.Errorf("plans: withdraw: %w", err)
	}
	if err := store.New(s.pool).DeleteSignup(ctx, store.DeleteSignupParams{PlanID: plan.ID, AccountID: accountID, Today: today}); err != nil {
		return fmt.Errorf("plans: withdraw: %w", err)
	}
	return nil
}

// Count returns how many people are signed up to each of the plans' running
// rounds, 0 for a plan with none. It never says who.
func (s *Signups) Count(ctx context.Context, planIDs []string) (map[string]int, error) {
	out := make(map[string]int, len(planIDs))
	if len(planIDs) == 0 {
		return out, nil
	}
	rows, err := store.New(s.pool).CountSignups(ctx, store.CountSignupsParams{PlanIds: planIDs, Since: s.since()})
	if err != nil {
		return nil, fmt.Errorf("plans: count: %w", err)
	}
	dates := s.cityDates(ctx)
	for _, r := range rows {
		today, err := dates.today(r.City)
		if err != nil {
			return nil, fmt.Errorf("plans: count: %w", err)
		}
		if !r.EndsOn.Before(today) {
			out[r.PlanID] += int(r.SignedUp)
		}
	}
	return out, nil
}

// planAndCity reads the plan and its city, or ErrNotFound for a plan that isn't
// there or whose id couldn't be one.
func (s *Signups) planAndCity(ctx context.Context, planID string) (store.AppPlan, cities.City, error) {
	planID = strings.ToLower(planID)
	if !idRe.MatchString(planID) {
		return store.AppPlan{}, cities.City{}, ErrNotFound
	}
	plan, err := store.New(s.pool).GetPlan(ctx, planID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.AppPlan{}, cities.City{}, ErrNotFound
	}
	if err != nil {
		return store.AppPlan{}, cities.City{}, fmt.Errorf("plans: signup: %w", err)
	}
	city, err := s.cities.Get(ctx, plan.City)
	if err != nil {
		return store.AppPlan{}, cities.City{}, fmt.Errorf("plans: signup: city: %w", err)
	}
	return plan, city, nil
}

// normalise drops repeated ticks and puts the rest in time order.
func normalise(ticks []Tick) []Tick {
	out := slices.Clone(ticks)
	slices.SortFunc(out, func(a, b Tick) int {
		if c := strings.Compare(a.Date, b.Date); c != 0 {
			return c
		}
		return partRank[a.Part] - partRank[b.Part]
	})
	return slices.Compact(out)
}

// parseTicks refuses malformed ticks and returns the rest normalised.
func parseTicks(ticks []Tick) ([]Tick, error) {
	if len(ticks) == 0 {
		return nil, fmt.Errorf("%w: a signup needs at least one tick", ErrInvalid)
	}
	for _, t := range ticks {
		// time.DateOnly's own parse takes "2026-1-9"; compare the round trip so a
		// date has one spelling.
		d, err := time.Parse(time.DateOnly, t.Date)
		if err != nil || d.Format(time.DateOnly) != t.Date {
			return nil, fmt.Errorf("%w: %q is not a date (YYYY-MM-DD)", ErrInvalid, t.Date)
		}
		if _, ok := partRank[t.Part]; !ok {
			return nil, fmt.Errorf("%w: %q is not morning, afternoon or evening", ErrInvalid, t.Part)
		}
	}
	return normalise(ticks), nil
}

// checkTicks holds each tick to the plan and the clock: a part the plan allows,
// at least policy.MinTickLeadDays after today and no later than a round after
// the round's last day.
func checkTicks(ticks []Tick, allowed []string, today, roundEnds time.Time) error {
	earliest := addDays(today, policy.MinTickLeadDays)
	latest := addDays(roundEnds, roundDays)
	for _, t := range ticks {
		if !slices.Contains(allowed, t.Part) {
			return fmt.Errorf("%w: %s", ErrTickPartNotAllowed, t.Part)
		}
		d, _ := time.Parse(time.DateOnly, t.Date) // parseTicks has checked it
		switch {
		case d.Before(earliest):
			return fmt.Errorf("%w: %s", ErrTickTooSoon, t.Date)
		case d.After(latest):
			return fmt.Errorf("%w: %s", ErrTickTooFar, t.Date)
		}
	}
	return nil
}
