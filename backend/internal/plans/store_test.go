package plans

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationStore runs a plan through signups, scheduling and closing
// against a real, migrated database. It's skipped unless TEST_DATABASE_URL is
// set (CI sets it). It creates its own city, activity and users and deletes
// them afterwards.
func TestIntegrationStore(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := dbgen.New(pool)

	const city, area, act = "zz-plans-test", "zz-plans-test-a", "zz-plans-test"
	// On Supabase, app.users references auth.users; plain Postgres (CI) has
	// no auth schema.
	var supabase bool
	if err := pool.QueryRow(ctx, "select exists (select 1 from pg_namespace where nspname = 'auth')").Scan(&supabase); err != nil {
		t.Fatal(err)
	}
	users := []uuid.UUID{
		uuid.MustParse("00000000-0000-4000-8000-00000000a001"),
		uuid.MustParse("00000000-0000-4000-8000-00000000a002"),
		uuid.MustParse("00000000-0000-4000-8000-00000000a003"),
	}
	cleanup := func() {
		// The city first: it cascades to its plans, which still use the activity.
		stmts := [][]any{
			{"delete from app.cities where id = $1", city},
			{"delete from app.activities where id = $1", act},
			{"delete from app.users where id = any($1)", users},
		}
		if supabase {
			stmts = append(stmts, []any{"delete from auth.users where id = any($1)", users})
		}
		for _, s := range stmts {
			if _, err := pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	setup := []string{
		`insert into app.cities (id, name, timezone, currency, languages, travel_modes)
		 values ('` + city + `', '{"en":"Test"}', 'UTC', 'USD', '{en}', '{walk}')`,
		`insert into app.areas (id, city_id, name, centroid)
		 values ('` + area + `', '` + city + `', '{"en":"A"}', 'POINT(0 0)')`,
		`insert into app.activities (id, name, category, indoor, energy, cost_level, conversational, min_group_size, max_group_size)
		 values ('` + act + `', '{"en":"Test"}', 'test', true, 1, 0, true, 2, 6)`,
	}
	for _, s := range setup {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range users {
		if supabase {
			if _, err := pool.Exec(ctx, "insert into auth.users (id, email) values ($1, $2)", id, id.String()+"@example.com"); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := pool.Exec(ctx, "insert into app.users (id) values ($1)", id); err != nil {
			t.Fatal(err)
		}
	}
	uid := func(i int) matching.UserID { return matching.UserID(users[i].String()) }
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }

	w := Window{Start: day(7), Days: 14, Deadline: time.Date(2030, 1, 20, 12, 0, 0, 0, time.UTC)}
	plan := matching.Plan{Area: area, Activity: act, Primary: "ja"}
	planID, err := Create(ctx, q, city, matching.Suggestion{Plan: plan, Audience: 9, Score: 4.5}, w)
	if err != nil {
		t.Fatal(err)
	}
	open, err := ListOpen(ctx, q, city)
	if err != nil {
		t.Fatal(err)
	}
	want := []Open{{ID: planID, OpenPlan: matching.OpenPlan{Plan: plan}, Round: 1, Window: w}}
	if !equalOpen(open, want) {
		t.Fatalf("ListOpen = %+v, want %+v", open, want)
	}

	// Signing up again replaces the times given before.
	first := matching.Signup{User: uid(0), Pattern: []matching.Slot{{Weekday: time.Monday, Block: matching.Morning}}}
	custom := matching.Signup{
		User:    uid(0),
		Pattern: []matching.Slot{{Weekday: time.Sunday, Block: matching.Evening}, {Weekday: time.Saturday, Block: matching.Morning}},
		Unavailable: []matching.Occasion{
			{Date: day(9)},                           // the whole day
			{Date: day(12), Block: matching.Evening}, // one part of it
		},
	}
	for _, s := range []matching.Signup{first, custom, {User: uid(1)}, {User: uid(2)}} {
		if err := SaveSignup(ctx, pool, planID, s); err != nil {
			t.Fatal(err)
		}
	}
	signups, err := LoadSignups(ctx, q, planID, w.Start)
	if err != nil {
		t.Fatal(err)
	}
	// Sorted by user, then (as stored) by weekday and date.
	wantSignups := []matching.Signup{{User: uid(0), Pattern: custom.Pattern, Unavailable: custom.Unavailable}, {User: uid(1)}, {User: uid(2)}}
	if !reflect.DeepEqual(signups, wantSignups) {
		t.Fatalf("LoadSignups = %+v, want %+v", signups, wantSignups)
	}

	occasion := matching.Occasion{Date: day(13), Block: matching.Evening}
	ev := Event{
		PlanID: planID, Round: 1, ConfirmBy: day(11),
		Scheduled: matching.Scheduled{
			Occasion: occasion,
			Groups:   []matching.Group{{Invited: []matching.UserID{uid(0), uid(1)}, Quorum: 2, MinAttendance: 2, Score: 1.5}},
		},
	}
	if _, err := SaveEvent(ctx, pool, Event{PlanID: planID, Round: 2, Scheduled: ev.Scheduled, ConfirmBy: day(11)}); !errors.Is(err, ErrClosed) {
		t.Errorf("SaveEvent for a round that isn't current: %v, want ErrClosed", err)
	}
	if _, err := SaveEvent(ctx, pool, ev); err != nil {
		t.Fatal(err)
	}
	// Invited people stop waiting; the third stays for the next round.
	open, err = ListOpen(ctx, q, city)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].Signups != 1 {
		t.Errorf("after scheduling: %+v, want one plan with 1 signup", open)
	}

	// Signing up for another plan, the event already invited to is blocked out.
	other, err := Create(ctx, q, city, matching.Suggestion{Plan: matching.Plan{Area: area, Activity: act, Primary: "en"}, Audience: 6, Score: 1}, w)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveSignup(ctx, pool, other, matching.Signup{User: uid(0)}); err != nil {
		t.Fatal(err)
	}
	signups, err = LoadSignups(ctx, q, other, w.Start)
	if err != nil {
		t.Fatal(err)
	}
	if len(signups) != 1 || !reflect.DeepEqual(signups[0].Unavailable, []matching.Occasion{occasion}) {
		t.Errorf("busy occasions not added: %+v", signups)
	}

	next := Window{Start: day(21), Days: 14, Deadline: day(30)}
	if err := NextRound(ctx, q, planID, next); err != nil {
		t.Fatal(err)
	}
	if err := Close(ctx, q, other, ClosedNoGroup); err != nil {
		t.Fatal(err)
	}
	open, err = ListOpen(ctx, q, city)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].ID != planID || open[0].Round != 2 || !open[0].Window.Start.Equal(next.Start) {
		t.Errorf("after next round and close: %+v", open)
	}
	if err := SaveSignup(ctx, pool, other, matching.Signup{User: uid(1)}); !errors.Is(err, ErrClosed) {
		t.Errorf("signup for a closed plan: %v, want ErrClosed", err)
	}
	if err := Withdraw(ctx, q, planID, users[2]); err != nil {
		t.Fatal(err)
	}
	if signups, err := LoadSignups(ctx, q, planID, next.Start); err != nil || len(signups) != 0 {
		t.Errorf("after withdrawing: %+v, %v", signups, err)
	}
}

func equalOpen(a, b []Open) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x.ID != y.ID || x.OpenPlan != y.OpenPlan || x.Round != y.Round || x.Window.Days != y.Window.Days ||
			!x.Window.Start.Equal(y.Window.Start) || !x.Window.Deadline.Equal(y.Window.Deadline) {
			return false
		}
	}
	return true
}
