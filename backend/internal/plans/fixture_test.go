package plans

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

// fixture is a throwaway city with one area, one activity (no venue needed,
// groups of 2-6) and some app users, for integration tests. They're skipped
// unless TEST_DATABASE_URL is set (CI sets it).
type fixture struct {
	pool            *pgxpool.Pool
	q               *dbgen.Queries
	city, area, act string
	users           []uuid.UUID
}

// newFixture creates the city named id (and an activity and area named
// after it) in the given time zone, with n users, and deletes them all when
// the test ends. Leftovers from an interrupted run are deleted first.
func newFixture(t *testing.T, id, timezone string, n int) *fixture {
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
	f := &fixture{pool: pool, q: dbgen.New(pool), city: id, area: id + "-a", act: id}
	for i := range n {
		// Stable ids, so a rerun finds and deletes an interrupted run's users.
		f.users = append(f.users, uuid.NewSHA1(uuid.NameSpaceURL, fmt.Appendf(nil, "issho-test:%s/%d", id, i)))
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
		stmts := [][]any{{"delete from app.users where id = any($1)", f.users}}
		if supabase {
			stmts = append(stmts, []any{"delete from auth.users where id = any($1)", f.users})
		}
		stmts = append(stmts,
			[]any{"delete from app.cities where id = $1", f.city},
			[]any{"delete from app.activities where id = $1", f.act})
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
		  values ($1, '{"en":"Test"}', $2, 'USD', '{en}', '{walk}')`, f.city, timezone},
		{`insert into app.areas (id, city_id, name, centroid) values ($1, $2, '{"en":"A"}', 'POINT(0 0)')`, f.area, f.city},
		{`insert into app.activities (id, name, category, indoor, energy, cost_level, conversational, min_group_size, max_group_size)
		  values ($1, '{"en":"Test"}', 'test', true, 1, 0, true, 2, 6)`, f.act},
	}
	for _, id := range f.users {
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
