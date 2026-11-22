package plans_test

import (
	"context"
	"errors"
	"reflect"
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
// hears by the last day of the plan's round: 7 days from the day the round
// opened, which was today's signup.
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
// "Rounds"): the round opened on the 5th ends on the 11th, so the 18th is the
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

// A round lasts 7 days, the 5th to the 11th. Signups in it share one hear-by
// day, whenever in the round they were made; once it has ended the signup is
// no longer the member's current one and the next signup opens a new round.
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
