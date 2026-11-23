package plans_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
)

const (
	ada   = "11111111-1111-4111-8111-111111111111"
	grace = "22222222-2222-4222-8222-222222222222"
)

// signupRig is the fixture city and its two plans with a fake clock and two
// accounts, ada and grace. The fixture's lantern plan runs in the evening and
// its sunrise plan in the morning.
type signupRig struct {
	pool *pgxpool.Pool
	clk  *clock.Fake
	svc  *plans.Signups
}

// 09:00 UTC on Monday 5 January 2026 is 16:00 the same day in Da Nang–Hoi An
// (UTC+7), so the city's today is 2026-01-05 and the first date a tick can be
// on is 2026-01-08.
var rigStart = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

func newSignupRig(t *testing.T) *signupRig {
	t.Helper()
	ctx := context.Background()
	pool := seeded(t)
	clk := clock.NewFake(rigStart)
	ppl := people.New(clk, pool, cities.New(pool), people.Config{DefaultCity: city})
	for id, email := range map[string]string{ada: "ada@example.com", grace: "grace@example.com"} {
		if _, err := ppl.Join(ctx, people.Claims{Subject: id, Email: email}, people.Preferences{}); err != nil {
			t.Fatal(err)
		}
	}
	return &signupRig{pool: pool, clk: clk, svc: plans.NewSignups(clk, pool, cities.New(pool))}
}

// A signup is a set of (date, part of the day) ticks on a plan. The person
// hears by the last day of the plan's round: the fixture's plans opened on the
// 5th, so today's round is the 5th to the 11th.
func TestASignupCarriesItsTicksAndTheDayThePersonWillHearBy(t *testing.T) {
	r := newSignupRig(t)
	ticks := []plans.Tick{{Date: "2026-01-08", Part: "evening"}, {Date: "2026-01-10", Part: "evening"}}

	got, err := r.svc.Set(context.Background(), ada, lanterns, ticks)
	if err != nil {
		t.Fatal(err)
	}

	if got.PlanID != lanterns || !reflect.DeepEqual(got.Ticks, ticks) || got.HearBy != "2026-01-11" {
		t.Fatalf("signup = %+v, want the lantern plan, both ticks, hear by 2026-01-11", got)
	}
	mine, err := r.svc.Mine(context.Background(), ada)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mine, []plans.Signup{got}) {
		t.Fatalf("Mine = %+v, want %+v", mine, got)
	}
}

// A tick less than 3 days ahead on the city's clock is refused: today and the
// two days after it, as the city counts them.
func TestATickLessThanThreeDaysAheadIsRefused(t *testing.T) {
	r := newSignupRig(t)
	for _, date := range []string{"2026-01-04", "2026-01-05", "2026-01-06", "2026-01-07"} {
		_, err := r.svc.Set(context.Background(), ada, lanterns, []plans.Tick{{Date: date, Part: "evening"}})
		if !errors.Is(err, plans.ErrTickTooSoon) {
			t.Fatalf("%s: err = %v, want ErrTickTooSoon", date, err)
		}
	}
	if _, err := r.svc.Set(context.Background(), ada, lanterns, []plans.Tick{{Date: "2026-01-08", Part: "evening"}}); err != nil {
		t.Fatalf("3 days ahead is the first day allowed: %v", err)
	}
}

// "Ahead" is on the city's clock, not UTC's: at 18:00 UTC on 5 January it is
// already 01:00 on the 6th in Da Nang–Hoi An, so the 8th is only 2 days ahead
// there though 3 by the UTC date.
func TestThreeDaysAheadIsCountedOnTheCitysClock(t *testing.T) {
	r := newSignupRig(t)
	r.clk.Set(time.Date(2026, 1, 5, 18, 0, 0, 0, time.UTC))

	_, err := r.svc.Set(context.Background(), ada, lanterns, []plans.Tick{{Date: "2026-01-08", Part: "evening"}})
	if !errors.Is(err, plans.ErrTickTooSoon) {
		t.Fatalf("err = %v, want ErrTickTooSoon", err)
	}
	if _, err := r.svc.Set(context.Background(), ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
}

// Only the parts of the day the plan allows are offered: the lantern plan runs
// in the evening and the sunrise plan in the morning.
func TestATickOnAPartOfTheDayThePlanDoesNotAllowIsRefused(t *testing.T) {
	r := newSignupRig(t)
	for _, c := range []struct{ plan, part string }{
		{lanterns, "morning"}, {lanterns, "afternoon"}, {sunrise, "evening"}, {sunrise, "afternoon"},
	} {
		_, err := r.svc.Set(context.Background(), ada, c.plan, []plans.Tick{{Date: "2026-01-09", Part: c.part}})
		if !errors.Is(err, plans.ErrTickPartNotAllowed) {
			t.Fatalf("%s on %s: err = %v, want ErrTickPartNotAllowed", c.part, c.plan, err)
		}
	}
}

// A refused signup leaves nothing: one bad tick among good ones refuses all,
// and no round or signup is made.
func TestARefusedSignupLeavesNothingBehind(t *testing.T) {
	r := newSignupRig(t)
	_, err := r.svc.Set(context.Background(), ada, lanterns, []plans.Tick{
		{Date: "2026-01-09", Part: "evening"}, {Date: "2026-01-06", Part: "evening"},
	})
	if !errors.Is(err, plans.ErrTickTooSoon) {
		t.Fatalf("err = %v, want ErrTickTooSoon", err)
	}
	if mine, err := r.svc.Mine(context.Background(), ada); err != nil || len(mine) != 0 {
		t.Fatalf("Mine = %v, %v; want none", mine, err)
	}
}

// Ticks run to 7 days after the round's last day (docs/spec/matching.md,
// "Rounds"): the round of the 5th to the 11th, so the 18th is the
// last date a tick can be on.
func TestATickMoreThanARoundBeyondTheRoundsEndIsRefused(t *testing.T) {
	r := newSignupRig(t)
	if _, err := r.svc.Set(context.Background(), ada, lanterns, []plans.Tick{{Date: "2026-01-18", Part: "evening"}}); err != nil {
		t.Fatalf("the last allowed date: %v", err)
	}
	_, err := r.svc.Set(context.Background(), ada, lanterns, []plans.Tick{{Date: "2026-01-19", Part: "evening"}})
	if !errors.Is(err, plans.ErrTickTooFar) {
		t.Fatalf("err = %v, want ErrTickTooFar", err)
	}
}

// A signup with no ticks, a date that isn't one or a part that isn't a part is
// invalid; so is a plan that doesn't exist, NotFound.
func TestMalformedSignupsAreRefused(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	for name, ticks := range map[string][]plans.Tick{
		"no ticks":              nil,
		"a date that isn't one": {{Date: "next friday", Part: "evening"}},
		"a part that isn't one": {{Date: "2026-01-09", Part: "midnight"}},
		"a date with a time":    {{Date: "2026-01-09T10:00:00Z", Part: "evening"}},
	} {
		if _, err := r.svc.Set(ctx, ada, lanterns, ticks); !errors.Is(err, plans.ErrInvalid) {
			t.Fatalf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	good := []plans.Tick{{Date: "2026-01-09", Part: "evening"}}
	for _, id := range []string{"6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b00ff", "not-an-id"} {
		if _, err := r.svc.Set(ctx, ada, id, good); !errors.Is(err, plans.ErrNotFound) {
			t.Fatalf("plan %s: err = %v, want ErrNotFound", id, err)
		}
	}
}

func ticksOf(t *testing.T, r *signupRig, account, plan string) []plans.Tick {
	t.Helper()
	mine, err := r.svc.Mine(context.Background(), account)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range mine {
		if s.PlanID == plan {
			return s.Ticks
		}
	}
	return nil
}

// Signing up again changes the person's times: the new set replaces the old,
// and it is still one signup.
func TestSigningUpAgainChangesTheTicks(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-08", Part: "evening"}, {Date: "2026-01-09", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
	want := []plans.Tick{{Date: "2026-01-10", Part: "evening"}}

	if _, err := r.svc.Set(ctx, ada, lanterns, want); err != nil {
		t.Fatal(err)
	}

	mine, err := r.svc.Mine(ctx, ada)
	if err != nil || len(mine) != 1 || !reflect.DeepEqual(mine[0].Ticks, want) {
		t.Fatalf("Mine = %+v, %v; want one signup with %v", mine, err, want)
	}
	if n := countOf(t, r, lanterns); n != 1 {
		t.Fatalf("signed up = %d, want 1: changing is not a second signup", n)
	}
}

// Withdrawing takes the signup away, for that plan only, and withdrawing
// twice, or from a plan never signed up to, is no error.
func TestWithdrawingRemovesTheSignup(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	for _, p := range []struct{ id, part string }{{lanterns, "evening"}, {sunrise, "morning"}} {
		if _, err := r.svc.Set(ctx, ada, p.id, []plans.Tick{{Date: "2026-01-09", Part: p.part}}); err != nil {
			t.Fatal(err)
		}
	}

	if err := r.svc.Withdraw(ctx, ada, lanterns); err != nil {
		t.Fatal(err)
	}
	if err := r.svc.Withdraw(ctx, ada, lanterns); err != nil {
		t.Fatalf("withdrawing twice: %v", err)
	}
	if err := r.svc.Withdraw(ctx, grace, sunrise); err != nil {
		t.Fatalf("withdrawing with no signup: %v", err)
	}

	mine, err := r.svc.Mine(ctx, ada)
	if err != nil || len(mine) != 1 || mine[0].PlanID != sunrise {
		t.Fatalf("Mine = %+v, %v; want only the sunrise plan", mine, err)
	}
	if n := countOf(t, r, lanterns); n != 0 {
		t.Fatalf("lantern signed up = %d, want 0", n)
	}
	if n := countOf(t, r, sunrise); n != 1 {
		t.Fatalf("sunrise signed up = %d, want 1: grace's withdraw touched ada's signup", n)
	}
	// Withdrawn, a person can sign up again.
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
}

// A person can sign up to both fixture plans at once, each with its own times.
func TestAPersonCanSignUpToBothFixturePlans(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Set(ctx, ada, sunrise, []plans.Tick{{Date: "2026-01-09", Part: "morning"}, {Date: "2026-01-10", Part: "morning"}}); err != nil {
		t.Fatal(err)
	}

	mine, err := r.svc.Mine(ctx, ada)
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 2 {
		t.Fatalf("Mine = %+v, want both plans", mine)
	}
	if got := ticksOf(t, r, ada, lanterns); !reflect.DeepEqual(got, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}) {
		t.Fatalf("lantern ticks = %v", got)
	}
	if got := ticksOf(t, r, ada, sunrise); len(got) != 2 {
		t.Fatalf("sunrise ticks = %v", got)
	}
}

// Mine is the member's own and nobody else's.
func TestMineIsOnlyTheCallersOwnSignups(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
	if mine, err := r.svc.Mine(ctx, grace); err != nil || len(mine) != 0 {
		t.Fatalf("grace's Mine = %+v, %v; want none", mine, err)
	}
}

func countOf(t *testing.T, r *signupRig, plan string) int {
	t.Helper()
	counts, err := r.svc.Count(context.Background(), []string{plan})
	if err != nil {
		t.Fatal(err)
	}
	return counts[plan]
}

// A plan counts how many are signed up and the count is a number: the type
// has nowhere to put who.
func TestACountSaysHowManyAreSignedUpToEachPlan(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	for _, who := range []string{ada, grace} {
		if _, err := r.svc.Set(ctx, who, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.svc.Set(ctx, ada, sunrise, []plans.Tick{{Date: "2026-01-09", Part: "morning"}}); err != nil {
		t.Fatal(err)
	}

	counts, err := r.svc.Count(ctx, []string{lanterns, sunrise, "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b00ff"})
	if err != nil {
		t.Fatal(err)
	}
	if counts[lanterns] != 2 || counts[sunrise] != 1 || counts["6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b00ff"] != 0 {
		t.Fatalf("counts = %v, want lantern 2, sunrise 1, the rest 0", counts)
	}
}

// The plan opened on the 5th, so a round lasts 7 days, the 5th to the 11th.
// Signups in it share one hear-by day, whenever in the round they were made;
// once it has ended the signup is no longer the member's current one and the
// next signup is in the next round, the 12th to the 18th.
func TestARoundIsSevenDaysAndTheNextSignupOpensTheNext(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	first, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}})
	if err != nil {
		t.Fatal(err)
	}
	r.clk.Set(time.Date(2026, 1, 9, 9, 0, 0, 0, time.UTC))
	late, err := r.svc.Set(ctx, grace, lanterns, []plans.Tick{{Date: "2026-01-12", Part: "evening"}})
	if err != nil {
		t.Fatal(err)
	}
	if first.HearBy != "2026-01-11" || late.HearBy != "2026-01-11" {
		t.Fatalf("hear by %s and %s, want 2026-01-11 for both", first.HearBy, late.HearBy)
	}

	// The 11th is still in the round on the city's clock, the 12th is not.
	r.clk.Set(time.Date(2026, 1, 11, 16, 59, 0, 0, time.UTC)) // 23:59 on the 11th there
	if n := countOf(t, r, lanterns); n != 2 {
		t.Fatalf("on the round's last day: signed up = %d, want 2", n)
	}
	r.clk.Set(time.Date(2026, 1, 11, 17, 0, 0, 0, time.UTC)) // 00:00 on the 12th there
	if n := countOf(t, r, lanterns); n != 0 {
		t.Fatalf("after the round: signed up = %d, want 0", n)
	}
	if mine, err := r.svc.Mine(ctx, ada); err != nil || len(mine) != 0 {
		t.Fatalf("after the round Mine = %+v, %v; want none", mine, err)
	}
	next, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-16", Part: "evening"}})
	if err != nil {
		t.Fatal(err)
	}
	if next.HearBy != "2026-01-18" {
		t.Fatalf("the next round hear-by = %s, want 2026-01-18", next.HearBy)
	}
}

// A closed city's plans are kept but not shown, so they take no signups: the
// same NotFound as for a plan that isn't there.
func TestAClosedCitysPlanTakesNoSignups(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	if _, err := r.pool.Exec(ctx, "update app.cities set state = 'closed' where slug = $1", city); err != nil {
		t.Fatal(err)
	}
	_, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}})
	if !errors.Is(err, plans.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// One signup per person per plan round is the database's rule, not only the
// code's: matching leaves out a person with two differing entries, and the
// outings ticket relies on there being none.
func TestTheDatabaseHoldsOneSignupPerPersonPerPlanRound(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}

	_, err := r.pool.Exec(ctx, `insert into app.signups (round_id, account_id, created_at, updated_at)
		select round_id, account_id, now(), now() from app.signups`)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "signups_pkey" {
		t.Fatalf("err = %v, want a unique violation of signups_pkey", err)
	}
}

// opensOn sets the day a plan opened on the city's calendar. Its rounds are the
// 7-day blocks counted from it, whenever anyone signs up.
func opensOn(t *testing.T, r *signupRig, plan, date string) {
	t.Helper()
	if _, err := r.pool.Exec(context.Background(), "update app.plans set opens_on = $2::date where id = $1", plan, date); err != nil {
		t.Fatal(err)
	}
}

func roundRows(t *testing.T, r *signupRig, plan string) int {
	t.Helper()
	var n int
	if err := r.pool.QueryRow(context.Background(), "select count(*) from app.plan_rounds where plan_id = $1", plan).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// A plan's rounds are fixed by the day it opened, not by who signs up when
// (docs/spec/matching.md, "Rounds"). The plan opened on Wednesday the 31st, so
// its rounds are 31 Dec–6 Jan, 7–13 Jan, and so on. Two people signing up on
// different days of one block land in one round with one hear-by; the day
// after its last day is the next round.
func TestRoundsAreSevenDayBlocksFromThePlansOpeningDay(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	opensOn(t, r, lanterns, "2025-12-31")

	early, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}) // the 5th there
	if err != nil {
		t.Fatal(err)
	}
	r.clk.Set(time.Date(2026, 1, 6, 9, 0, 0, 0, time.UTC)) // the 6th, the block's last day
	late, err := r.svc.Set(ctx, grace, lanterns, []plans.Tick{{Date: "2026-01-10", Part: "evening"}})
	if err != nil {
		t.Fatal(err)
	}
	if early.HearBy != "2026-01-06" || late.HearBy != "2026-01-06" {
		t.Fatalf("hear by %s and %s, want 2026-01-06 for both", early.HearBy, late.HearBy)
	}
	if n := roundRows(t, r, lanterns); n != 1 {
		t.Fatalf("%d rounds stored, want 1", n)
	}

	r.clk.Set(time.Date(2026, 1, 6, 17, 0, 0, 0, time.UTC)) // 00:00 on the 7th there
	next, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-12", Part: "evening"}})
	if err != nil {
		t.Fatal(err)
	}
	if next.HearBy != "2026-01-13" {
		t.Fatalf("the next round's hear-by = %s, want 2026-01-13", next.HearBy)
	}
	if n := roundRows(t, r, lanterns); n != 2 {
		t.Fatalf("%d rounds stored, want 2", n)
	}
}

// The block is counted from the opening day however long ago it was, so a
// plan that opened on 22 November 2025 is in round 6 on 5 January: 3–9 Jan.
func TestALongOpenPlansRoundIsDerivedNotCounted(t *testing.T) {
	r := newSignupRig(t)
	opensOn(t, r, lanterns, "2025-11-22")

	got, err := r.svc.Set(context.Background(), ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.HearBy != "2026-01-09" {
		t.Fatalf("hear by %s, want 2026-01-09", got.HearBy)
	}
	var number int
	var starts, ends time.Time
	err = r.pool.QueryRow(context.Background(),
		"select number, starts_on, ends_on from app.plan_rounds where plan_id = $1", lanterns).Scan(&number, &starts, &ends)
	if err != nil {
		t.Fatal(err)
	}
	if number != 6 || starts.Format(time.DateOnly) != "2026-01-03" || ends.Format(time.DateOnly) != "2026-01-09" {
		t.Fatalf("round %d, %s to %s; want round 6, 2026-01-03 to 2026-01-09", number, starts.Format(time.DateOnly), ends.Format(time.DateOnly))
	}
}

// The opening day is on the city's calendar too: at 18:00 UTC on the 5th it is
// already the 6th there, the plan's first day, and a plan that opens the 6th
// has begun.
func TestAPlanThatHasNotOpenedYetTakesNoSignups(t *testing.T) {
	r := newSignupRig(t)
	opensOn(t, r, lanterns, "2026-01-06")

	_, err := r.svc.Set(context.Background(), ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}})
	if !errors.Is(err, plans.ErrNotFound) {
		t.Fatalf("on the 5th: err = %v, want ErrNotFound", err)
	}
	r.clk.Set(time.Date(2026, 1, 5, 18, 0, 0, 0, time.UTC))
	got, err := r.svc.Set(context.Background(), ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}})
	if err != nil {
		t.Fatalf("on the 6th, its first day: %v", err)
	}
	if got.HearBy != "2026-01-12" {
		t.Fatalf("hear by %s, want 2026-01-12", got.HearBy)
	}
}

// A plan is hidden until its opening day, on the city's calendar: ListPlans and
// GetPlan leave it out as they do a missing plan, and signups turn it away the
// same way, so the three agree on whether a plan is shown. The fixture's
// plans open on the 5th.
func TestAPlanIsHiddenUntilItsOpeningDay(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	reader := plans.New(r.clk, r.pool, cities.New(r.pool))
	tick := []plans.Tick{{Date: "2026-01-08", Part: "evening"}}

	r.clk.Set(time.Date(2026, 1, 4, 16, 59, 0, 0, time.UTC)) // 23:59 on the 4th there
	listed, err := reader.List(ctx, city, "en")
	if err != nil || len(listed) != 0 {
		t.Fatalf("on the 4th: List = %d plans, %v; want none", len(listed), err)
	}
	if _, err := reader.Get(ctx, lanterns, "en"); !errors.Is(err, plans.ErrNotFound) {
		t.Fatalf("on the 4th: Get err = %v, want ErrNotFound", err)
	}
	if _, err := r.svc.Set(ctx, ada, lanterns, tick); !errors.Is(err, plans.ErrNotFound) {
		t.Fatalf("on the 4th: Set err = %v, want ErrNotFound", err)
	}

	r.clk.Set(time.Date(2026, 1, 4, 17, 0, 0, 0, time.UTC)) // midnight on the 5th there
	listed, err = reader.List(ctx, city, "en")
	if err != nil || len(listed) != 2 {
		t.Fatalf("on the 5th: List = %d plans, %v; want both", len(listed), err)
	}
	if _, err := reader.Get(ctx, lanterns, "en"); err != nil {
		t.Fatalf("on the 5th: Get: %v", err)
	}
	if _, err := r.svc.Set(ctx, ada, lanterns, tick); err != nil {
		t.Fatalf("on the 5th: Set: %v", err)
	}
}

// Only the plans that haven't opened are left out: a city lists the others.
func TestTheCityListsThePlansThatHaveOpened(t *testing.T) {
	r := newSignupRig(t)
	opensOn(t, r, lanterns, "2026-01-06")
	reader := plans.New(r.clk, r.pool, cities.New(r.pool))

	listed, err := reader.List(context.Background(), city, "en")
	if err != nil || len(listed) != 1 || listed[0].ID != sunrise {
		t.Fatalf("List = %v, %v; want only the sunrise plan", listed, err)
	}
}

// Many people making the first signup of a round at once open one round, and
// all of them hear by the same day.
func TestConcurrentFirstSignupsMakeOneRound(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	opensOn(t, r, lanterns, "2025-12-31")
	ppl := people.New(r.clk, r.pool, cities.New(r.pool), people.Config{DefaultCity: city})
	accounts := []string{ada, grace}
	for i := 3; i <= 8; i++ {
		id := strings.Repeat(string(rune('0'+i)), 8) + "-3333-4333-8333-333333333333"
		if _, err := ppl.Join(ctx, people.Claims{Subject: id, Email: fmt.Sprintf("p%d@example.com", i)}, people.Preferences{}); err != nil {
			t.Fatal(err)
		}
		accounts = append(accounts, id)
	}

	start := make(chan struct{})
	hearBy := make([]string, len(accounts))
	errs := make([]error, len(accounts))
	var wg sync.WaitGroup
	for i, id := range accounts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got, err := r.svc.Set(ctx, id, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}})
			hearBy[i], errs[i] = got.HearBy, err
		}()
	}
	close(start)
	wg.Wait()

	for i := range accounts {
		if errs[i] != nil || hearBy[i] != "2026-01-06" {
			t.Fatalf("signup %d: hear by %q, err %v; want 2026-01-06", i, hearBy[i], errs[i])
		}
	}
	if n := roundRows(t, r, lanterns); n != 1 {
		t.Fatalf("%d rounds stored, want 1", n)
	}
	if n := countOf(t, r, lanterns); n != len(accounts) {
		t.Fatalf("signed up = %d, want %d", n, len(accounts))
	}
}

// A plan has one row per round number: the database refuses a second.
func TestTheDatabaseHoldsOneRowPerPlanRoundNumber(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
	_, err := r.pool.Exec(ctx, `insert into app.plan_rounds (plan_id, number, starts_on, ends_on)
		select plan_id, number, starts_on + 1, ends_on + 1 from app.plan_rounds`)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "plan_rounds_plan_id_number_key" {
		t.Fatalf("err = %v, want a unique violation of plan_rounds_plan_id_number_key", err)
	}
}

// Forming reads the plan's running round: who is signed up and the times each
// ticked. A person who changed their times is one entry with the latest ones,
// and entries come in account order so a run is repeatable.
func TestFormingReadsTheRunningRoundOneEntryPerPerson(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	if _, err := r.svc.Set(ctx, grace, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-08", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-10", Part: "evening"}, {Date: "2026-01-09", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}

	got, err := r.svc.Forming(ctx, lanterns)
	if err != nil {
		t.Fatal(err)
	}

	want := []plans.Entry{
		{Account: ada, Ticks: []plans.Tick{{Date: "2026-01-09", Part: "evening"}, {Date: "2026-01-10", Part: "evening"}}},
		{Account: grace, Ticks: []plans.Tick{{Date: "2026-01-09", Part: "evening"}}},
	}
	if got.RoundID == "" || got.StartsAt != "17:00" || got.EndsAt != "20:30" || got.TimeZone != "Asia/Ho_Chi_Minh" || !reflect.DeepEqual(got.Entries, want) {
		t.Fatalf("Forming = %+v, want a round with the lantern plan's 17:00 start and entries %+v", got, want)
	}
}

// A plan nobody has signed up to has a running round with no entries and no
// stored row yet; a plan that isn't shown is ErrNotFound, as for a signup.
func TestFormingAPlanWithNoSignupsHasNoEntries(t *testing.T) {
	r := newSignupRig(t)

	got, err := r.svc.Forming(context.Background(), lanterns)
	if err != nil {
		t.Fatal(err)
	}
	if got.RoundID != "" || len(got.Entries) != 0 {
		t.Fatalf("Forming = %+v, want no round row and no entries", got)
	}
	if _, err := r.svc.Forming(context.Background(), "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b00ff"); !errors.Is(err, plans.ErrNotFound) {
		t.Fatalf("a plan nobody has: err = %v, want ErrNotFound", err)
	}
	if _, err := r.pool.Exec(context.Background(), "update app.cities set state = 'closed'"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Forming(context.Background(), lanterns); !errors.Is(err, plans.ErrNotFound) {
		t.Fatalf("a closed city's plan: err = %v, want ErrNotFound", err)
	}
}

// An outing's date is never less than 3 days after its invitations go out
// (docs/spec/matching.md, "Invitations"), so forming reads only the ticks that
// are still 3 days ahead on the city's clock *now*, not when they were ticked.
// A person with none left drops out of the round for this run.
func TestFormingLeavesOutTicksNowLessThanThreeDaysAhead(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-08", Part: "evening"}, {Date: "2026-01-10", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Set(ctx, grace, lanterns, []plans.Tick{{Date: "2026-01-08", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
	// Two days later, on the 7th, the 8th is 1 day ahead and the 10th is 3.
	r.clk.Set(rigStart.Add(48 * time.Hour))

	got, err := r.svc.Forming(ctx, lanterns)
	if err != nil {
		t.Fatal(err)
	}

	want := []plans.Entry{{Account: ada, Ticks: []plans.Tick{{Date: "2026-01-10", Part: "evening"}}}}
	if !reflect.DeepEqual(got.Entries, want) {
		t.Fatalf("entries = %+v, want only ada's tick on the 10th", got.Entries)
	}
}

// release runs ReleaseTime in a transaction of its own, as outings does inside
// the one that makes a person going.
func (r *signupRig) release(t *testing.T, account, date, part, exceptRound string) {
	t.Helper()
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := r.svc.ReleaseTime(ctx, tx, account, date, part, exceptRound); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// An outing a person is going to takes that time from their other signups: the
// tick on the same date and part of the day goes from every signup but the
// outing's own round, the rest of those signups stay, and a signup left with no
// tick goes with it (a signup always has one).
func TestReleasingATimeTakesItFromTheOtherSignupsOfThePerson(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	// The sunrise plan also runs in the evening, so the same part of the day can
	// be ticked on two plans.
	if _, err := r.pool.Exec(ctx, `update app.plans set parts_of_day = '{morning,evening}' where id = $1`, sunrise); err != nil {
		t.Fatal(err)
	}
	set := func(account, plan string, ticks ...plans.Tick) {
		t.Helper()
		if _, err := r.svc.Set(ctx, account, plan, ticks); err != nil {
			t.Fatal(err)
		}
	}
	evening9, evening10, morning9 := plans.Tick{Date: "2026-01-09", Part: "evening"}, plans.Tick{Date: "2026-01-10", Part: "evening"}, plans.Tick{Date: "2026-01-09", Part: "morning"}
	set(ada, lanterns, evening9, evening10)
	set(ada, sunrise, evening9, evening10, morning9)
	set(grace, sunrise, evening9) // someone else's signup is never touched
	round, err := r.svc.Forming(ctx, lanterns)
	if err != nil {
		t.Fatal(err)
	}

	r.release(t, ada, "2026-01-09", "evening", round.RoundID)

	if got := ticksOf(t, r, ada, sunrise); !reflect.DeepEqual(got, []plans.Tick{morning9, evening10}) {
		t.Errorf("ada's sunrise ticks = %v, want the 9th's evening gone and the rest kept", got)
	}
	if got := ticksOf(t, r, ada, lanterns); !reflect.DeepEqual(got, []plans.Tick{evening9, evening10}) {
		t.Errorf("ada's lantern ticks = %v, want them kept (it is the outing's own round)", got)
	}
	if got := ticksOf(t, r, grace, sunrise); !reflect.DeepEqual(got, []plans.Tick{evening9}) {
		t.Errorf("grace's ticks = %v, want them kept", got)
	}

	// A signup whose last tick is released is withdrawn.
	r.release(t, grace, "2026-01-09", "evening", round.RoundID)
	mine, err := r.svc.Mine(ctx, grace)
	if err != nil || len(mine) != 0 {
		t.Errorf("grace's signups = %v, %v; want none once its only tick was released", mine, err)
	}
}

// A signup says when it was made or last changed, so the app can prefill the
// ticks from the person's last signup (docs/spec/matching.md, "Times"): the
// newest by that time, whichever plan it is on and whenever its round ends.
func TestASignupSaysWhenItWasMadeOrLastChanged(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	first := r.clk.Now()

	got, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-08", Part: "evening"}})
	if err != nil {
		t.Fatal(err)
	}
	if !got.SignedAt.Equal(first) {
		t.Fatalf("SignedAt = %v, want %v", got.SignedAt, first)
	}

	r.clk.Advance(time.Hour)
	second := r.clk.Now()
	if _, err := r.svc.Set(ctx, ada, sunrise, []plans.Tick{{Date: "2026-01-09", Part: "morning"}}); err != nil {
		t.Fatal(err)
	}
	r.clk.Advance(time.Hour)
	third := r.clk.Now()
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-10", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}

	mine, err := r.svc.Mine(ctx, ada)
	if err != nil {
		t.Fatal(err)
	}
	signedAt := map[string]time.Time{}
	for _, s := range mine {
		signedAt[s.PlanID] = s.SignedAt
	}
	if !signedAt[sunrise].Equal(second) {
		t.Fatalf("sunrise signed at %v, want %v (when it was made)", signedAt[sunrise], second)
	}
	if !signedAt[lanterns].Equal(third) {
		t.Fatalf("lanterns signed at %v, want %v (when it was last changed, not first made)", signedAt[lanterns], third)
	}
}

// The plan says the last day of its round: the running round's, or the one a
// round begun now would end on, which is the same derived day. It is the
// same for everyone, signed up or not.
func TestAPlanSaysTheLastDayOfItsRound(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	reader := plans.New(r.clk, r.pool, cities.New(r.pool))
	roundEnds := func() string {
		t.Helper()
		p, err := reader.Get(ctx, lanterns, "en")
		if err != nil {
			t.Fatal(err)
		}
		return p.RoundEnds
	}

	if got := roundEnds(); got != "2026-01-11" {
		t.Fatalf("with no signups, on the 5th: round ends %s, want 2026-01-11", got)
	}
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-08", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
	if got := roundEnds(); got != "2026-01-11" {
		t.Fatalf("with a round running: round ends %s, want 2026-01-11", got)
	}
	// 18:00 UTC on the 11th is 01:00 on the 12th there: the next round.
	r.clk.Set(time.Date(2026, 1, 11, 18, 0, 0, 0, time.UTC))
	if got := roundEnds(); got != "2026-01-18" {
		t.Fatalf("on the 12th there: round ends %s, want 2026-01-18", got)
	}
	listed, err := reader.List(ctx, city, "en")
	if err != nil || len(listed) != 2 || listed[0].RoundEnds != "2026-01-18" || listed[1].RoundEnds != "2026-01-18" {
		t.Fatalf("List = %+v, %v; want both plans' rounds to end 2026-01-18", listed, err)
	}
}

// The app offers the dates from 3 days ahead to a round after the round's last
// day, from the plan's own round end. Whatever day it is, SignUp accepts every
// date and every part the plan allows in that window, and refuses the day after
// the last (TICK_TOO_FAR) and the day before the first (TICK_TOO_SOON). The
// days run across two rounds, and the times of day on either side of the city's
// midnight (09:00 and 18:00 UTC, the latter already tomorrow there).
func TestEveryDateTheAppOffersIsAcceptedAndTheDayAfterTheLastIsNot(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	reader := plans.New(r.clk, r.pool, cities.New(r.pool))

	for day := 0; day < 16; day++ {
		for _, hour := range []int{9, 18} {
			r.clk.Set(time.Date(2026, 1, 5+day, hour, 0, 0, 0, time.UTC))
			today, err := time.Parse(time.DateOnly, r.clk.Now().In(time.FixedZone("", 7*3600)).Format(time.DateOnly))
			if err != nil {
				t.Fatal(err)
			}
			p, err := reader.Get(ctx, lanterns, "en")
			if err != nil {
				t.Fatal(err)
			}
			last, err := time.Parse(time.DateOnly, p.RoundEnds)
			if err != nil {
				t.Fatalf("RoundEnds %q: %v", p.RoundEnds, err)
			}
			offer := func(from, to time.Time) []plans.Tick {
				var ticks []plans.Tick
				for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
					ticks = append(ticks, plans.Tick{Date: d.Format(time.DateOnly), Part: "evening"})
				}
				return ticks
			}
			offered := offer(today.AddDate(0, 0, 3), last.AddDate(0, 0, 7))
			label := fmt.Sprintf("%s: %d offered", r.clk.Now().Format(time.RFC3339), len(offered))

			if _, err := r.svc.Set(ctx, ada, lanterns, offered); err != nil {
				t.Fatalf("%s: every offered date must be accepted: %v", label, err)
			}
			after := last.AddDate(0, 0, 8).Format(time.DateOnly)
			if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: after, Part: "evening"}}); !errors.Is(err, plans.ErrTickTooFar) {
				t.Fatalf("%s: the day after the last (%s): err = %v, want ErrTickTooFar", label, after, err)
			}
			before := today.AddDate(0, 0, 2).Format(time.DateOnly)
			if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: before, Part: "evening"}}); !errors.Is(err, plans.ErrTickTooSoon) {
				t.Fatalf("%s: the day before the first (%s): err = %v, want ErrTickTooSoon", label, before, err)
			}
		}
	}
}

// Releasing a time takes the person's other signups before their ticks, the
// order Set takes them in, so a person who resubmits their times on another
// plan at the moment they become going waits instead of deadlocking (40P01).
// The test plays Set's two steps by hand around a ReleaseTime: Set holds the
// signup row, ReleaseTime runs until it waits on a lock, and then Set reaches
// for the ticks. With ReleaseTime taking ticks first, the two would deadlock.
func TestReleasingATimeWaitsForASetOnTheSameSignupWithoutDeadlocking(t *testing.T) {
	r := newSignupRig(t)
	ctx := context.Background()
	if _, err := r.svc.Set(ctx, ada, lanterns, []plans.Tick{{Date: "2026-01-09", Part: "evening"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.svc.Set(ctx, ada, sunrise, []plans.Tick{{Date: "2026-01-09", Part: "morning"}}); err != nil {
		t.Fatal(err)
	}
	lanternRound, err := r.svc.Forming(ctx, lanterns)
	if err != nil {
		t.Fatal(err)
	}
	sunriseRound, err := r.svc.Forming(ctx, sunrise)
	if err != nil {
		t.Fatal(err)
	}

	// Set's first step, on ada's sunrise signup: the upsert that locks its row.
	set, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = set.Rollback(ctx) }()
	if _, err := set.Exec(ctx, `update app.signups set updated_at = now() where round_id = $1 and account_id = $2`, sunriseRound.RoundID, ada); err != nil {
		t.Fatal(err)
	}

	// ReleaseTime of ada's only sunrise tick, as making ada going on a lantern
	// outing does.
	release, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = release.Rollback(ctx) }()
	var pid int
	if err := release.QueryRow(ctx, `select pg_backend_pid()`).Scan(&pid); err != nil {
		t.Fatal(err)
	}
	released := make(chan error, 1)
	go func() {
		if err := r.svc.ReleaseTime(ctx, release, ada, "2026-01-09", "morning", lanternRound.RoundID); err != nil {
			released <- err
			return
		}
		released <- release.Commit(ctx)
	}()

	// The barrier: wait until ReleaseTime is blocked on a lock Set holds.
	// Ten seconds of 10ms polls, counted: only internal/clock reads the wall clock.
	polls := 0
	for {
		var waiting bool
		if err := r.pool.QueryRow(ctx, `select exists (select 1 from pg_stat_activity where pid = $1 and wait_event_type = 'Lock')`, pid).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if polls++; polls > 1000 {
			t.Fatal("ReleaseTime never waited on the signup Set holds")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Set's second step: its ticks. Then it writes the new ones and commits.
	if _, err := set.Exec(ctx, `delete from app.signup_ticks where round_id = $1 and account_id = $2`, sunriseRound.RoundID, ada); err != nil {
		t.Fatalf("Set's ticks: %v (want no deadlock)", err)
	}
	if _, err := set.Exec(ctx, `insert into app.signup_ticks (round_id, account_id, tick_date, part) values ($1, $2, '2026-01-10', 'morning')`, sunriseRound.RoundID, ada); err != nil {
		t.Fatal(err)
	}
	if err := set.Commit(ctx); err != nil {
		t.Fatalf("Set's commit: %v", err)
	}
	if err := <-released; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40P01" {
			t.Fatalf("ReleaseTime deadlocked against Set: %v", err)
		}
		t.Fatal(err)
	}
	// Set's new tick, on another day, is kept: the release ran after it.
	if got := ticksOf(t, r, ada, sunrise); !reflect.DeepEqual(got, []plans.Tick{{Date: "2026-01-10", Part: "morning"}}) {
		t.Errorf("ada's sunrise ticks = %v, want Set's new tick kept", got)
	}
}
