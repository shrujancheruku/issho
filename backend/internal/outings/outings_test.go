package outings_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/fixtures"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/outings"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/policy"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

const (
	city     = "da-nang-hoi-an"
	lanterns = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0001" // runs in the evening, from 17:00
	sunrise  = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0002" // runs in the morning, from 06:30
)

// 09:00 UTC on Monday 5 January 2026 is 16:00 the same day in Da Nang–Hoi An
// (UTC+7): the city's today is the 5th, the fixture plans' opening day, so the
// first date a tick can be on is the 8th.
var rigStart = time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)

// rig is the fixture city and its two plans on a fake clock, with the real
// signups, people and outings on a throwaway Postgres.
type rig struct {
	pool    *pgxpool.Pool
	clk     *clock.Fake
	signups *plans.Signups
	people  *people.Service
	svc     *outings.Service
}

func newRig(t *testing.T) *rig {
	t.Helper()
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := cities.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	if err := plans.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	clk := clock.NewFake(rigStart)
	cs := cities.New(pool)
	ppl := people.New(clk, pool, cs, people.Config{DefaultCity: city})
	signups := plans.NewSignups(clk, pool, cs)
	return &rig{pool: pool, clk: clk, signups: signups, people: ppl, svc: outings.New(clk, pool, signups, ppl)}
}

// person is an account with a profile: n names it, languages are the ones they
// speak.
type person struct {
	id        string
	gender    people.Gender
	languages []string
	womenOnly bool
}

func account(n int) string { return fmt.Sprintf("00000000-0000-4000-8000-%012d", n) }

// member makes the account n with a profile that speaks languages ("en" and
// "vi" when none are named).
func (r *rig) member(t *testing.T, n int, p person) string {
	t.Helper()
	id := account(n)
	if _, err := r.people.Join(context.Background(), people.Claims{Subject: id, Email: fmt.Sprintf("p%d@example.com", n)}, people.Preferences{}); err != nil {
		t.Fatal(err)
	}
	if p.gender == "" {
		p.gender = people.GenderMan
	}
	if p.languages == nil {
		p.languages = []string{"en", "vi"}
	}
	if _, err := r.people.SaveProfile(context.Background(), id, people.ProfileInput{
		Name: fmt.Sprintf("Person %d", n), DateOfBirth: "1990-06-15", Gender: p.gender, Languages: p.languages,
		LivesInCity: true, AcceptedCodeOfConduct: true, OnlyWomenAndNonBinary: p.womenOnly,
	}); err != nil {
		t.Fatal(err)
	}
	return id
}

// signUp signs the account up to the plan with ticks, as ("2026-01-09", "evening") pairs.
func (r *rig) signUp(t *testing.T, id, plan string, ticks ...plans.Tick) {
	t.Helper()
	if _, err := r.signups.Set(context.Background(), id, plan, ticks); err != nil {
		t.Fatal(err)
	}
}

var evening9th = plans.Tick{Date: "2026-01-09", Part: "evening"}

// Matching forming a group means creating an outing and sending its
// invitations (docs/spec/matching.md): three people who ticked one evening and
// share a language each get a seat in one new outing, invited, with the time
// its 48 hours end.
func TestFormingMakesAnOutingWithAnInvitedSeatForEachPerson(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	var ids []string
	for n := 1; n <= 3; n++ {
		id := r.member(t, n, person{})
		r.signUp(t, id, lanterns, evening9th)
		ids = append(ids, id)
	}

	formed, err := r.svc.Form(ctx, lanterns)
	if err != nil {
		t.Fatal(err)
	}

	if formed != 1 {
		t.Fatalf("formed = %d outings, want 1", formed)
	}
	var outing string
	for _, id := range ids {
		seats, err := r.svc.Mine(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(seats) != 1 {
			t.Fatalf("%s has %d seats, want 1", id, len(seats))
		}
		s := seats[0]
		if outing == "" {
			outing = s.OutingID
		}
		want := outings.Seat{
			OutingID: outing, PlanID: lanterns, Date: "2026-01-09", Part: "evening", StartsAt: "17:00",
			Languages: []string{"en", "vi"}, State: outings.SeatInvited, OutingState: outings.OutingOffered,
			InvitedAt: rigStart, InvitationEndsAt: rigStart.Add(policy.InvitationWindow),
		}
		if !reflect.DeepEqual(s, want) {
			t.Fatalf("seat = %+v, want %+v", s, want)
		}
	}
}

// seatsOf is every seat the accounts hold.
func (r *rig) seatsOf(t *testing.T, ids ...string) []outings.Seat {
	t.Helper()
	var out []outings.Seat
	for _, id := range ids {
		s, err := r.svc.Mine(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, s...)
	}
	return out
}

// distinctOutings counts the outings the seats are in.
func distinctOutings(seats []outings.Seat) int {
	seen := map[string]bool{}
	for _, s := range seats {
		seen[s.OutingID] = true
	}
	return len(seen)
}

// n members, each signed up to the lanterns plan for the evening of the 9th.
func (r *rig) crowd(t *testing.T, from, to int, p person) []string {
	t.Helper()
	var ids []string
	for n := from; n <= to; n++ {
		id := r.member(t, n, p)
		r.signUp(t, id, lanterns, evening9th)
		ids = append(ids, id)
	}
	return ids
}

// Forming twice for the same signups makes no second outing: the second run
// finds everyone seated and forms nothing, and nobody's seat changes.
func TestFormingTwiceForTheSameSignupsMakesNoSecondOuting(t *testing.T) {
	r := newRig(t)
	ids := r.crowd(t, 1, 4, person{})
	if _, err := r.svc.Form(context.Background(), lanterns); err != nil {
		t.Fatal(err)
	}
	before := r.seatsOf(t, ids...)

	r.clk.Advance(time.Hour) // a later run must not rewrite the invitations either
	formed, err := r.svc.Form(context.Background(), lanterns)
	if err != nil {
		t.Fatal(err)
	}

	if formed != 0 {
		t.Fatalf("the second run formed %d outings, want 0", formed)
	}
	if after := r.seatsOf(t, ids...); !reflect.DeepEqual(before, after) || distinctOutings(after) != 1 {
		t.Fatalf("seats changed or doubled:\nbefore %+v\nafter  %+v", before, after)
	}
}

// People who signed up after a run are formed on the next one, among
// themselves; those already seated are not grouped again.
func TestFormingAgainSeatsOnlyThoseWithNoSeatYet(t *testing.T) {
	r := newRig(t)
	first := r.crowd(t, 1, 3, person{})
	if _, err := r.svc.Form(context.Background(), lanterns); err != nil {
		t.Fatal(err)
	}
	second := r.crowd(t, 4, 6, person{})

	formed, err := r.svc.Form(context.Background(), lanterns)
	if err != nil {
		t.Fatal(err)
	}

	if formed != 1 {
		t.Fatalf("formed = %d, want 1 outing for the three new people", formed)
	}
	a, b := r.seatsOf(t, first...), r.seatsOf(t, second...)
	if len(a) != 3 || len(b) != 3 || distinctOutings(a) != 1 || distinctOutings(b) != 1 || a[0].OutingID == b[0].OutingID {
		t.Fatalf("first %+v, second %+v: want two outings of three seats each", a, b)
	}
}

// Two jobs forming one plan at once (River runs a job at least once) make one
// outing between them.
func TestFormingAtTheSameTimeMakesOneOuting(t *testing.T) {
	r := newRig(t)
	ids := r.crowd(t, 1, 4, person{})
	// Make a run hold its transaction open for half a second, so the others
	// start inside it: without the round's lock they would all form the group.
	if _, err := r.pool.Exec(context.Background(), fmt.Sprintf(`
		create function app.slow_seat() returns trigger language plpgsql as $$
		begin perform pg_sleep(0.5); return new; end $$;
		create trigger slow_seat before insert on app.seats
		for each row when (new.account_id = '%s') execute function app.slow_seat();`, ids[0])); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 6)
	start := make(chan struct{})
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = r.svc.Form(context.Background(), lanterns)
		}()
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if seats := r.seatsOf(t, ids...); len(seats) != 4 || distinctOutings(seats) != 1 {
		t.Fatalf("seats = %+v, want 4 seats in one outing", seats)
	}
}

// What matching forbids stays forbidden through the job: someone with no
// language in common with the others, or a group that breaks the "only women
// and non-binary people" setting, is left without a seat.
func TestFormingKeepsMatchingsRules(t *testing.T) {
	r := newRig(t)
	same := r.crowd(t, 1, 3, person{languages: []string{"vi"}})
	stranger := r.crowd(t, 4, 4, person{languages: []string{"fr"}})

	if _, err := r.svc.Form(context.Background(), lanterns); err != nil {
		t.Fatal(err)
	}

	if seats := r.seatsOf(t, same...); len(seats) != 3 || !reflect.DeepEqual(seats[0].Languages, []string{"vi"}) {
		t.Fatalf("the three Vietnamese speakers have %+v, want three seats and the language they share", seats)
	}
	if seats := r.seatsOf(t, stranger...); len(seats) != 0 {
		t.Fatalf("the one who shares no language has %+v, want no seat", seats)
	}
}

func TestFormingNeverBreaksTheWomenAndNonBinaryOnlySetting(t *testing.T) {
	r := newRig(t)
	only := r.crowd(t, 1, 1, person{gender: people.GenderWoman, womenOnly: true})
	men := r.crowd(t, 2, 4, person{gender: people.GenderMan})

	if _, err := r.svc.Form(context.Background(), lanterns); err != nil {
		t.Fatal(err)
	}

	if seats := r.seatsOf(t, append(only, men...)...); len(seats) != 3 || distinctOutings(seats) != 1 {
		t.Fatalf("seats = %+v, want the three men grouped and the woman with the setting on left out", seats)
	}
	if seats := r.seatsOf(t, only...); len(seats) != 0 {
		t.Fatalf("the person with the setting on has %+v, want no seat", seats)
	}
}

// An outing's date is never less than 3 days after its invitations go out: a
// run two days after the signups ignores a tick that is now 1 day ahead.
func TestFormingNeverSetsADateLessThanThreeDaysAfterTheInvitations(t *testing.T) {
	r := newRig(t)
	var ids []string
	for n := 1; n <= 3; n++ {
		id := r.member(t, n, person{})
		r.signUp(t, id, lanterns, plans.Tick{Date: "2026-01-08", Part: "evening"})
		ids = append(ids, id)
	}
	r.clk.Set(rigStart.Add(48 * time.Hour)) // the 7th: the 8th is 1 day ahead

	formed, err := r.svc.Form(context.Background(), lanterns)
	if err != nil {
		t.Fatal(err)
	}

	if formed != 0 || len(r.seatsOf(t, ids...)) != 0 {
		t.Fatalf("formed %d outings, want none for a date 1 day ahead", formed)
	}
}

// A run for a plan nobody has signed up to, or one that isn't shown, makes
// nothing.
func TestFormingAPlanWithNoSignupsMakesNothing(t *testing.T) {
	r := newRig(t)
	formed, err := r.svc.Form(context.Background(), lanterns)
	if err != nil || formed != 0 {
		t.Fatalf("Form = %d, %v; want 0 and no error", formed, err)
	}
	if _, err := r.svc.Form(context.Background(), "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b00ff"); !errors.Is(err, plans.ErrNotFound) {
		t.Fatalf("a plan nobody has: err = %v, want plans.ErrNotFound", err)
	}
}

// Every group of a run is stored in one transaction: if one seat can't be
// written the run leaves no outing, not even the groups already written. The
// failure is a trigger on the throwaway database that refuses one person's
// seat, so the first outing is written before the second fails.
func TestAFailedRunLeavesNoOutingBehind(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	// Two groups of three at two times, so two outings are written in a run.
	var ids []string
	for n := 1; n <= 6; n++ {
		id := r.member(t, n, person{})
		tick := evening9th
		if n > 3 {
			tick = plans.Tick{Date: "2026-01-10", Part: "evening"}
		}
		r.signUp(t, id, lanterns, tick)
		ids = append(ids, id)
	}
	if _, err := r.pool.Exec(ctx, fmt.Sprintf(`
		create function app.refuse_seat() returns trigger language plpgsql as $$
		begin raise exception 'no seat for you'; end $$;
		create trigger refuse_seat before insert on app.seats
		for each row when (new.account_id = '%s') execute function app.refuse_seat();`, ids[5])); err != nil {
		t.Fatal(err)
	}

	if _, err := r.svc.Form(ctx, lanterns); err == nil {
		t.Fatal("Form succeeded, want the refused seat's error")
	}

	var outings, seats, jobs int
	if err := r.pool.QueryRow(ctx, `select (select count(*) from app.outings), (select count(*) from app.seats),
		(select count(*) from river.river_job where kind = 'outings_expire_invitations')`).Scan(&outings, &seats, &jobs); err != nil {
		t.Fatal(err)
	}
	// The unanswered-seat jobs are made in the invitations' transaction, so a
	// failed run leaves none for outings that don't exist.
	if outings != 0 || seats != 0 || jobs != 0 {
		t.Fatalf("%d outings, %d seats and %d jobs were left behind, want none", outings, seats, jobs)
	}
}

// The coordinator's ruling on #617: matching leaves out a person whose two
// entries differ, so forming must never meet two. The database holds one signup
// per person per plan round (`signups_pkey` on (round_id, account_id), #615),
// whatever code writes it, so a second entry is refused...
func TestTheDatabaseNeverHoldsTwoSignupsOfOnePersonForTheRoundFormingReads(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	ids := r.crowd(t, 1, 3, person{})

	_, err := r.pool.Exec(ctx, `insert into app.signups (round_id, account_id, created_at, updated_at)
		select round_id, account_id, now(), now() from app.signups`)

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "signups_pkey" {
		t.Fatalf("err = %v, want a unique violation of signups_pkey", err)
	}
	// ...and what forming reads is one entry for each person.
	round, rerr := r.signups.Forming(ctx, lanterns)
	if rerr != nil || len(round.Entries) != len(ids) {
		t.Fatalf("Forming = %+v, %v; want %d entries, one each", round, rerr, len(ids))
	}
}

// ...and a person who changes their times is one entry, the latest, not two
// that differ: they are grouped on the times they last gave.
func TestAPersonWhoChangedTheirTimesIsFormedOnTheLatest(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	ids := r.crowd(t, 1, 2, person{})
	third := r.member(t, 3, person{})
	r.signUp(t, third, lanterns, plans.Tick{Date: "2026-01-10", Part: "evening"}) // no one else can go then
	if formed, err := r.svc.Form(ctx, lanterns); err != nil || formed != 0 {
		t.Fatalf("Form = %d, %v; want nothing yet", formed, err)
	}

	r.signUp(t, third, lanterns, evening9th)
	formed, err := r.svc.Form(ctx, lanterns)

	if err != nil || formed != 1 {
		t.Fatalf("Form = %d, %v; want 1 outing", formed, err)
	}
	if seats := r.seatsOf(t, append(ids, third)...); len(seats) != 3 || seats[2].Date != "2026-01-09" {
		t.Fatalf("seats = %+v, want all three on the 9th", seats)
	}
}

// What a person is shown of an outing is theirs alone: Mine takes the account,
// so someone with no seat in it sees nothing of it.
func TestAPersonWithNoSeatInAnOutingIsNeverShownIt(t *testing.T) {
	r := newRig(t)
	r.crowd(t, 1, 3, person{})
	outsider := r.member(t, 9, person{})
	if _, err := r.svc.Form(context.Background(), lanterns); err != nil {
		t.Fatal(err)
	}

	if seats := r.seatsOf(t, outsider); len(seats) != 0 {
		t.Fatalf("a person with no seat sees %+v, want nothing", seats)
	}
}

// policyWindow is the invitations' fixed 48 hours, written out from the spec
// independently of policy.InvitationWindow.
const policyWindow = 48 * time.Hour
