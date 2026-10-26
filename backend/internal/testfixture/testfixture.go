// Package testfixture builds throwaway cities and users in a real database
// for integration tests in any package. Tests using it are skipped unless
// TEST_DATABASE_URL is set (CI sets it).
package testfixture

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

// Fixture is a throwaway city with one area, one activity (no venue needed,
// groups of 2-6) and some app users.
type Fixture struct {
	Pool            *pgxpool.Pool
	Q               *dbgen.Queries
	City, Area, Act string
	Users           []uuid.UUID
}

// New creates the city named id (and an activity and area named
// after it) in the given time zone, with n users, and deletes them all when
// the test ends. Leftovers from an interrupted run are deleted first.
func New(t *testing.T, id, timezone string, n int) *Fixture {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	// t.Cleanup, not defer: cleanups run in reverse order, so the pool is
	// still open for the deletes registered below.
	t.Cleanup(pool.Close)
	f := &Fixture{Pool: pool, Q: dbgen.New(pool), City: id, Area: id + "-a", Act: id}
	for i := range n {
		// Stable ids, so a rerun finds and deletes an interrupted run's users.
		f.Users = append(f.Users, uuid.NewSHA1(uuid.NameSpaceURL, fmt.Appendf(nil, "issho-test:%s/%d", id, i)))
	}
	// On Supabase, app.users references auth.users; plain Postgres (CI) has
	// no auth schema.
	var supabase bool
	if err := pool.QueryRow(ctx, "select exists (select 1 from pg_namespace where nspname = 'auth')").Scan(&supabase); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		// Users first (their profiles use the area), then the city, which
		// cascades to its plans, which use the activity.
		stmts := [][]any{{"delete from app.users where id = any($1)", f.Users}}
		if supabase {
			stmts = append(stmts, []any{"delete from auth.users where id = any($1)", f.Users})
		}
		stmts = append(stmts,
			[]any{"delete from app.cities where id = $1", f.City},
			[]any{"delete from app.activities where id = $1", f.Act})
		for _, s := range stmts {
			if _, err := pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	setup := [][]any{
		{`insert into app.cities (id, name, timezone, currency, languages, travel_modes)
		  values ($1, '{"en":"Test"}', $2, 'USD', '{en}', '{walk}')`, f.City, timezone},
		{`insert into app.areas (id, city_id, name, centroid) values ($1, $2, '{"en":"A"}', 'POINT(0 0)')`, f.Area, f.City},
		{`insert into app.activities (id, name, category, indoor, energy, cost_level, conversational, min_group_size, max_group_size)
		  values ($1, '{"en":"Test"}', 'test', true, 1, 0, true, 2, 6)`, f.Act},
	}
	for _, id := range f.Users {
		if supabase {
			setup = append(setup, []any{"insert into auth.users (id, email) values ($1, $2)", id, id.String() + "@example.com"})
		}
		setup = append(setup, []any{"insert into app.users (id) values ($1)", id})
	}
	for _, s := range setup {
		if _, err := pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

// AddProfile gives a fixture user a profile in the fixture's area: fluent in
// lang, free on Saturday afternoons, loving the fixture's activity.
func (f *Fixture) AddProfile(t *testing.T, id uuid.UUID, lang string) {
	t.Helper()
	ctx := context.Background()
	stmts := [][]any{
		{`insert into app.profiles (user_id, display_name, birth_year, gender, home_area_id, max_travel_min,
		   energy, budget, size_pref, primary_language, conduct_version)
		  values ($1, 'Test', 1995, 'unspecified', $2, 30, 2, 1, 'medium', $3, 1)`, id, f.Area, lang},
		{"insert into app.user_languages (user_id, lang, fluent) values ($1, $2, true)", id, lang},
		{"insert into app.availability_slots (user_id, weekday, block) values ($1, 6, 'afternoon')", id},
		{"insert into app.user_activity_prefs (user_id, activity_id, weight) values ($1, $2, 2)", id, f.Act},
	}
	for _, s := range stmts {
		if _, err := f.Pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
			t.Fatal(err)
		}
	}
}
