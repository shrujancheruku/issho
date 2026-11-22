package db_test

import (
	"context"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

func schemas(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(),
		`select nspname from pg_namespace where nspname in ('app', 'river', 'goose', 'public') order by 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

// objectsIn lists the tables, views, sequences, types and functions in a
// schema, so "nothing in public" covers everything a migration could create.
func objectsIn(t *testing.T, pool *pgxpool.Pool, schema string) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
		select c.relname from pg_class c join pg_namespace n on n.oid = c.relnamespace where n.nspname = $1
		union all
		select t.typname from pg_type t join pg_namespace n on n.oid = t.typnamespace
		  where n.nspname = $1 and t.typtype in ('e', 'd', 'c') and t.typrelid = 0
		union all
		select p.proname from pg_proc p join pg_namespace n on n.oid = p.pronamespace where n.nspname = $1
		order by 1`, schema)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return out
}

func TestUpCreatesAppAndRiverAndKeepsPublicEmpty(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)

	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}

	got := schemas(t, pool)
	for _, want := range []string{"app", "river", "goose"} {
		if !slices.Contains(got, want) {
			t.Errorf("schema %q missing after up; have %v", want, got)
		}
	}
	if got := objectsIn(t, pool, "public"); len(got) != 0 {
		t.Errorf("public holds %v, want nothing", got)
	}
	if !slices.Contains(objectsIn(t, pool, "goose"), "goose_db_version") {
		t.Errorf("goose's version table is not in the goose schema: %v", objectsIn(t, pool, "goose"))
	}
	if !slices.Contains(objectsIn(t, pool, "river"), "river_job") {
		t.Errorf("river_job is not in the river schema: %v", objectsIn(t, pool, "river"))
	}
}

// Down must undo the whole baseline so that CI's up -> down -> up (and a
// developer's local reset) works.
func TestDownThenUpAgain(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)

	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := db.DownAll(ctx, pool); err != nil {
		t.Fatal(err)
	}
	got := schemas(t, pool)
	for _, gone := range []string{"app", "river"} {
		if slices.Contains(got, gone) {
			t.Errorf("schema %q still there after down; have %v", gone, got)
		}
	}
	if got := objectsIn(t, pool, "public"); len(got) != 0 {
		t.Errorf("public holds %v after down, want nothing", got)
	}

	if err := db.Up(ctx, pool); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}

// downPast rolls back every migration numbered above n, so a test of
// migration n holds however many later ones there are.
func downPast(t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()
	files, err := os.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		num, _, _ := strings.Cut(f.Name(), "_")
		if v, err := strconv.Atoi(num); err == nil && v > n {
			if err := db.Down(context.Background(), pool); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// The queue's migration (00002) is undone by one step of Down, which leaves the
// baseline in place, and goes up again.
func TestQueueMigrationDownThenUp(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)

	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(objectsIn(t, pool, "app"), "queue_items") {
		t.Fatalf("app.queue_items missing after up: %v", objectsIn(t, pool, "app"))
	}

	downPast(t, pool, 2)
	if err := db.Down(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if got := objectsIn(t, pool, "app"); len(got) != 0 {
		t.Errorf("app holds %v after one down, want nothing (queue_items and its index gone)", got)
	}
	if !slices.Contains(schemas(t, pool), "app") {
		t.Error("one down removed the app schema, which is the baseline's")
	}

	if err := db.Up(ctx, pool); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if !slices.Contains(objectsIn(t, pool, "app"), "queue_items") {
		t.Error("app.queue_items missing after up, down, up")
	}
}

// The accounts migration (00003) is undone by its Down, which leaves the queue
// and the baseline in place, and goes up again.
func TestAccountsMigrationDownThenUp(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)

	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(objectsIn(t, pool, "app"), "accounts") {
		t.Fatalf("app.accounts missing after up: %v", objectsIn(t, pool, "app"))
	}

	downPast(t, pool, 3)
	if err := db.Down(ctx, pool); err != nil {
		t.Fatal(err)
	}
	got := objectsIn(t, pool, "app")
	if slices.Contains(got, "accounts") || slices.Contains(got, "accounts_email_key") {
		t.Errorf("app holds %v after down, want accounts and its index gone", got)
	}
	if !slices.Contains(got, "queue_items") {
		t.Errorf("down of accounts took the queue with it: %v", got)
	}

	if err := db.Up(ctx, pool); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if !slices.Contains(objectsIn(t, pool, "app"), "accounts") {
		t.Error("app.accounts missing after up, down, up")
	}
}

// The signups migration (00006) is undone by its Down, which leaves the plans
// and everything before it in place, and goes up again.
func TestSignupsMigrationDownThenUp(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)

	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"plan_rounds", "signups", "signup_ticks"} {
		if !slices.Contains(objectsIn(t, pool, "app"), table) {
			t.Fatalf("app.%s missing after up", table)
		}
	}

	downPast(t, pool, 6)
	if err := db.Down(ctx, pool); err != nil {
		t.Fatal(err)
	}
	got := objectsIn(t, pool, "app")
	for _, gone := range []string{"plan_rounds", "signups", "signup_ticks", "signups_account_id_idx"} {
		if slices.Contains(got, gone) {
			t.Errorf("app holds %v after down, want %s gone", got, gone)
		}
	}
	if !slices.Contains(got, "plans") || !slices.Contains(got, "profiles") {
		t.Errorf("down of signups took the plans or profiles with it: %v", got)
	}
	var hasOpensOn bool
	if err := pool.QueryRow(ctx, `select exists (select 1 from information_schema.columns
		where table_schema = 'app' and table_name = 'plans' and column_name = 'opens_on')`).Scan(&hasOpensOn); err != nil {
		t.Fatal(err)
	}
	if hasOpensOn {
		t.Error("app.plans.opens_on is still there after down")
	}

	if err := db.Up(ctx, pool); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if !slices.Contains(objectsIn(t, pool, "app"), "signups") {
		t.Error("app.signups missing after up, down, up")
	}
}

// The outings migration (00007) is undone by its Down, which leaves the
// signups and everything before it in place, and goes up again.
func TestOutingsMigrationDownThenUp(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)

	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"outings", "seats"} {
		if !slices.Contains(objectsIn(t, pool, "app"), table) {
			t.Fatalf("app.%s missing after up", table)
		}
	}

	downPast(t, pool, 7)
	if err := db.Down(ctx, pool); err != nil {
		t.Fatal(err)
	}
	got := objectsIn(t, pool, "app")
	for _, gone := range []string{"outings", "seats", "seats_account_id_idx"} {
		if slices.Contains(got, gone) {
			t.Errorf("app holds %v after down, want %s gone", got, gone)
		}
	}
	if !slices.Contains(got, "signups") || !slices.Contains(got, "plan_rounds") {
		t.Errorf("down of outings took the signups or rounds with it: %v", got)
	}

	if err := db.Up(ctx, pool); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if !slices.Contains(objectsIn(t, pool, "app"), "seats") {
		t.Error("app.seats missing after up, down, up")
	}
}

// The seat answers migration (00008) lets a seat be accepted (a yes that waits
// for the outing to be set). Its Down turns such a seat back into an invitation
// and the check refuses the state again; going up again allows it.
func TestSeatAnswersMigrationDownThenUp(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		insert into app.cities (slug, state, time_zone, currency) values ('x', 'open', 'UTC', 'USD');
		insert into app.accounts (id, email, city, display_language, created_at)
		values ('00000000-0000-4000-8000-000000000001', 'a@example.com', 'x', 'en', now());
		insert into app.plans (id, city, price, meeting_address, parts_of_day, starts_at, ends_at, opens_on)
		values ('00000000-0000-4000-8000-0000000000a1', 'x', 1, 'm', '{evening}', '17:00', '18:00', '2026-01-05');
		insert into app.plan_rounds (id, plan_id, number, starts_on, ends_on)
		values ('00000000-0000-4000-8000-0000000000b1', '00000000-0000-4000-8000-0000000000a1', 0, '2026-01-05', '2026-01-11');
		insert into app.outings (id, plan_id, round_id, date, part, starts_at, ends_at, time_zone, languages, state, created_at)
		values ('00000000-0000-4000-8000-0000000000c1', '00000000-0000-4000-8000-0000000000a1', '00000000-0000-4000-8000-0000000000b1',
		        '2026-01-09', 'evening', '17:00', '18:00', 'UTC', '{en}', 'offered', now());
		insert into app.seats (outing_id, round_id, account_id, state, invited_at, invitation_ends_at)
		values ('00000000-0000-4000-8000-0000000000c1', '00000000-0000-4000-8000-0000000000b1',
		        '00000000-0000-4000-8000-000000000001', 'invited', now(), now() + interval '2 days');`); err != nil {
		t.Fatal(err)
	}
	update := func(state string) error {
		_, err := pool.Exec(ctx, `update app.seats set state = $1`, state)
		return err
	}
	if err := update("accepted"); err != nil {
		t.Fatalf("a seat can't be accepted after up: %v", err)
	}

	downPast(t, pool, 8)
	if err := db.Down(ctx, pool); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := pool.QueryRow(ctx, `select state from app.seats`).Scan(&state); err != nil || state != "invited" {
		t.Fatalf("after down the accepted seat is %q, %v; want invited", state, err)
	}
	if err := update("accepted"); err == nil {
		t.Fatal("a seat can be accepted after down, want the check back")
	}

	if err := db.Up(ctx, pool); err != nil {
		t.Fatalf("up after down: %v", err)
	}
	if err := update("accepted"); err != nil {
		t.Fatalf("a seat can't be accepted after up, down, up: %v", err)
	}
}

// The join links migration (00009) makes the links and admissions tables. A
// link ends one way (used or cancelled, not both) and has one code, and a link
// admits one account. Its Down drops both and goes up again.
func TestJoinLinksMigrationDownThenUp(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"join_links", "admissions"} {
		if !slices.Contains(objectsIn(t, pool, "app"), table) {
			t.Fatalf("app.%s missing after up", table)
		}
	}
	if _, err := pool.Exec(ctx, `
		insert into app.cities (slug, state, time_zone, currency) values ('x', 'open', 'UTC', 'USD');
		insert into app.accounts (id, email, city, display_language, created_at) values
		  ('00000000-0000-4000-8000-000000000001', 'a@example.com', 'x', 'en', now()),
		  ('00000000-0000-4000-8000-000000000002', 'b@example.com', 'x', 'en', now()),
		  ('00000000-0000-4000-8000-000000000003', 'c@example.com', 'x', 'en', now());
		insert into app.join_links (id, member_id, code, created_at) values
		  ('00000000-0000-4000-8000-0000000000d1', '00000000-0000-4000-8000-000000000001', 'code-1', now());`); err != nil {
		t.Fatal(err)
	}
	for name, stmt := range map[string]string{
		"a code is used by one link":     `insert into app.join_links (member_id, code, created_at) values ('00000000-0000-4000-8000-000000000001', 'code-1', now())`,
		"a link ends one way":            `update app.join_links set cancelled_at = now(), redeemed_at = now()`,
		"via is a join link or the seed": `insert into app.admissions (account_id, admitted_at, via) values ('00000000-0000-4000-8000-000000000002', now(), 'other')`,
	} {
		if _, err := pool.Exec(ctx, stmt); err == nil {
			t.Errorf("%s: the database accepted it", name)
		}
	}
	// A link admits one account.
	if _, err := pool.Exec(ctx, `insert into app.admissions (account_id, admitted_at, via, brought_by, link_id)
		values ('00000000-0000-4000-8000-000000000002', now(), 'join-link', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-0000000000d1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into app.admissions (account_id, admitted_at, via, brought_by, link_id)
		values ('00000000-0000-4000-8000-000000000003', now(), 'join-link', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-0000000000d1')`); err == nil {
		t.Error("one link admitted two accounts")
	}

	downPast(t, pool, 9)
	if err := db.Down(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"join_links", "admissions"} {
		if slices.Contains(objectsIn(t, pool, "app"), table) {
			t.Errorf("app.%s remains after down", table)
		}
	}
	if err := db.Up(ctx, pool); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}

// The check-in and feedback migration (00010) keeps each outing's end and time
// zone, a seat's history, check-in and handle, and makes the feedback, unticks
// and picks tables. Outings and seats that were there before it get the plan's
// end and the city's zone, and the history their state implies. Its Down drops
// all of it and goes up again.
func TestCheckInAndFeedbackMigrationDownThenUp(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	downPast(t, pool, 9)
	if _, err := pool.Exec(ctx, `
		insert into app.cities (slug, state, time_zone, currency) values ('x', 'open', 'Asia/Ho_Chi_Minh', 'USD');
		insert into app.accounts (id, email, city, display_language, created_at) values
		  ('00000000-0000-4000-8000-000000000001', 'a@example.com', 'x', 'en', now()),
		  ('00000000-0000-4000-8000-000000000002', 'b@example.com', 'x', 'en', now()),
		  ('00000000-0000-4000-8000-000000000003', 'c@example.com', 'x', 'en', now());
		insert into app.plans (id, city, price, meeting_address, parts_of_day, starts_at, ends_at, opens_on)
		values ('00000000-0000-4000-8000-0000000000a1', 'x', 1, 'm', '{evening}', '17:00', '20:30', '2026-01-05');
		insert into app.plan_rounds (id, plan_id, number, starts_on, ends_on)
		values ('00000000-0000-4000-8000-0000000000b1', '00000000-0000-4000-8000-0000000000a1', 0, '2026-01-05', '2026-01-11');
		insert into app.outings (id, plan_id, round_id, date, part, starts_at, languages, state, created_at)
		values ('00000000-0000-4000-8000-0000000000c1', '00000000-0000-4000-8000-0000000000a1', '00000000-0000-4000-8000-0000000000b1',
		        '2026-01-09', 'evening', '17:00', '{en}', 'set', now());
		insert into app.seats (outing_id, round_id, account_id, state, invited_at, invitation_ends_at) values
		  ('00000000-0000-4000-8000-0000000000c1', '00000000-0000-4000-8000-0000000000b1', '00000000-0000-4000-8000-000000000001', 'going', '2026-01-05 09:00+00', '2026-01-07 09:00+00'),
		  ('00000000-0000-4000-8000-0000000000c1', '00000000-0000-4000-8000-0000000000b1', '00000000-0000-4000-8000-000000000002', 'cancelled', '2026-01-05 09:00+00', '2026-01-07 09:00+00'),
		  ('00000000-0000-4000-8000-0000000000c1', '00000000-0000-4000-8000-0000000000b1', '00000000-0000-4000-8000-000000000003', 'invited', '2026-01-05 09:00+00', '2026-01-07 09:00+00');`); err != nil {
		t.Fatal(err)
	}

	if err := db.Up(ctx, pool); err != nil {
		t.Fatalf("up over existing outings: %v", err)
	}

	var endsAt, zone string
	if err := pool.QueryRow(ctx, `select ends_at, time_zone from app.outings`).Scan(&endsAt, &zone); err != nil || endsAt != "20:30" || zone != "Asia/Ho_Chi_Minh" {
		t.Errorf("the outing got end %q and zone %q, %v; want the plan's and the city's", endsAt, zone, err)
	}
	history := func(account string) (going, left bool) {
		if err := pool.QueryRow(ctx, `select going_at is not null, left_going_at is not null from app.seats where account_id = $1`, account).Scan(&going, &left); err != nil {
			t.Fatal(err)
		}
		return going, left
	}
	for account, want := range map[string][2]bool{
		"00000000-0000-4000-8000-000000000001": {true, false}, // going: became going, still is
		"00000000-0000-4000-8000-000000000002": {true, true},  // cancelled: was going and left
		"00000000-0000-4000-8000-000000000003": {false, false},
	} {
		if going, left := history(account); going != want[0] || left != want[1] {
			t.Errorf("%s history = going %v, left %v; want %v", account, going, left, want)
		}
	}
	for name, stmt := range map[string]string{
		"checking in without ever going":      `update app.seats set checked_in_at = now() where account_id = '00000000-0000-4000-8000-000000000003'`,
		"leaving going without ever going":    `update app.seats set left_going_at = now() where account_id = '00000000-0000-4000-8000-000000000003'`,
		"a feedback row for no seat":          `insert into app.feedback (outing_id, account_id, went, answered_at, updated_at) values ('00000000-0000-4000-8000-0000000000c1', '00000000-0000-4000-8000-0000000000ff', true, now(), now())`,
		"a pick with no feedback by the pick": `insert into app.picks (outing_id, picker_id, picked_id, created_at) values ('00000000-0000-4000-8000-0000000000c1', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000002', now())`,
	} {
		if _, err := pool.Exec(ctx, stmt); err == nil {
			t.Errorf("%s: the database accepted it", name)
		}
	}
	if _, err := pool.Exec(ctx, `insert into app.feedback (outing_id, account_id, went, answered_at, updated_at) values ('00000000-0000-4000-8000-0000000000c1', '00000000-0000-4000-8000-000000000001', true, now(), now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `insert into app.picks (outing_id, picker_id, picked_id, created_at) values ('00000000-0000-4000-8000-0000000000c1', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000001', now())`); err == nil {
		t.Error("a person picked themselves")
	}
	// A person who leaves takes what they picked and what was said of them.
	if _, err := pool.Exec(ctx, `insert into app.picks (outing_id, picker_id, picked_id, created_at) values ('00000000-0000-4000-8000-0000000000c1', '00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000002', now())`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `delete from app.accounts where id = '00000000-0000-4000-8000-000000000002'`); err != nil {
		t.Fatal(err)
	}
	var picks int
	if err := pool.QueryRow(ctx, `select count(*) from app.picks`).Scan(&picks); err != nil || picks != 0 {
		t.Errorf("%d picks remain after the person picked was deleted, %v; want none", picks, err)
	}

	downPast(t, pool, 9)
	for _, table := range []string{"feedback", "feedback_unticks", "picks"} {
		if slices.Contains(objectsIn(t, pool, "app"), table) {
			t.Errorf("app.%s remains after down", table)
		}
	}
	if err := db.Up(ctx, pool); err != nil {
		t.Fatalf("up after down: %v", err)
	}
}
