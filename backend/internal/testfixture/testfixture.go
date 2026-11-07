// Package testfixture builds throwaway cities and users in a real database
// for integration tests in any package. Tests using it are skipped unless
// TEST_DATABASE_URL is set (CI sets it). When CI=true, a missing URL fails them
// instead, so a lost env var can't pass silently.
//
// One database per package: TEST_DATABASE_URL names the server and a migrated,
// seeded template database (scripts/db-checks.sh builds it). The first test in
// a package that needs the database clones the template into a database of
// its own, and Run (called from the package's TestMain) drops it when the
// package's tests finish. Packages run in parallel without seeing each
// other's rows; within a package tests share the database, so they stay serial
// unless they use unique ids and t.Parallel. Tests never connect to the
// template itself: Postgres refuses to clone a database that has sessions.
package testfixture

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// DatabasePrefix starts the name of every per-package database, so a sweep
// (scripts/db-checks.sh) can drop the ones a killed run left behind.
const DatabasePrefix = "issho_test_"

// adminDatabase is where CREATE/DROP DATABASE run from: never the template,
// which must have no sessions while it is cloned.
const adminDatabase = "template1"

var (
	pkgMu  sync.Mutex
	pkgURL string // this package's database, once created
	pkgErr error
)

// withDatabase returns rawURL pointing at the database name, whatever it
// pointed at before.
func withDatabase(rawURL, name string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.New("TEST_DATABASE_URL is not a URL")
	}
	u.Path = "/" + name
	u.RawPath = ""
	return u.String(), nil
}

// databaseName is the database a connection string points at.
func databaseName(rawURL string) (string, error) {
	cfg, err := pgx.ParseConfig(rawURL)
	if err != nil {
		return "", errors.New("TEST_DATABASE_URL is not a valid connection string")
	}
	return cfg.Database, nil
}

// createDatabase clones the template named in templateURL into a new database
// and returns its URL.
func createDatabase(ctx context.Context, templateURL string) (string, error) {
	tmpl, err := databaseName(templateURL)
	if err != nil {
		return "", err
	}
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s%d_%s", DatabasePrefix, os.Getpid(), hex.EncodeToString(b[:]))
	adminURL, err := withDatabase(templateURL, adminDatabase)
	if err != nil {
		return "", err
	}
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return "", fmt.Errorf("connect to create the package database: %w", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	// Identifiers can't be bound; both are ours or come from the operator's own URL.
	q := fmt.Sprintf("create database %s template %s", pgx.Identifier{name}.Sanitize(), pgx.Identifier{tmpl}.Sanitize())
	// Another session on the template (a leftover connection, or a package
	// cloning at this very moment) makes Postgres answer 55006; retry briefly.
	for attempt := 0; ; attempt++ {
		_, err = conn.Exec(ctx, q)
		var pgErr *pgconn.PgError
		if err == nil || !errors.As(err, &pgErr) || pgErr.Code != "55006" || attempt >= 20 {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if err != nil {
		return "", fmt.Errorf("create the package database from template %q (the role needs CREATEDB): %w", tmpl, err)
	}
	return withDatabase(templateURL, name)
}

// DropDatabase drops the database rawURL points at, closing any session left
// on it. It is a no-op when the database is already gone.
func DropDatabase(ctx context.Context, rawURL string) error {
	name, err := databaseName(rawURL)
	if err != nil {
		return err
	}
	adminURL, err := withDatabase(rawURL, adminDatabase)
	if err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close(ctx) }()
	_, err = conn.Exec(ctx, "drop database if exists "+pgx.Identifier{name}.Sanitize()+" with (force)")
	return err
}

// Run is a package's TestMain body: it runs the tests, then drops the
// package's database if any test created one. Packages that use this package
// call it from TestMain (`func TestMain(m *testing.M) { testfixture.Run(m) }`);
// a package that forgets leaks its database until the next db-checks.sh sweep.
func Run(m *testing.M) {
	code := m.Run()
	pkgMu.Lock()
	u := pkgURL
	pkgMu.Unlock()
	if u != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		if err := DropDatabase(ctx, u); err != nil {
			fmt.Fprintln(os.Stderr, "testfixture: dropping the package database:", err)
		}
		cancel()
	}
	os.Exit(code)
}

// DatabaseURL returns the URL of this package's own database for an
// integration test, creating it from the template TEST_DATABASE_URL names on
// first use. Unset, the test is skipped locally, but fails when CI=true: a
// dropped env var in the workflow would otherwise turn every database test
// into a silent pass.
func DatabaseURL(t *testing.T) string {
	t.Helper()
	templateURL := os.Getenv("TEST_DATABASE_URL")
	if templateURL == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("TEST_DATABASE_URL not set, but CI=true: integration tests must not skip in CI")
		}
		t.Skip("TEST_DATABASE_URL not set")
	}
	pkgMu.Lock()
	defer pkgMu.Unlock()
	if pkgURL == "" && pkgErr == nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		pkgURL, pkgErr = createDatabase(ctx, templateURL)
	}
	if pkgErr != nil {
		t.Fatal(pkgErr)
	}
	return pkgURL
}

// Pool opens a pool on the test database and closes it when the test ends. It
// is skipped (failed in CI) like DatabaseURL. Because t.Cleanup runs in reverse
// order, cleanups the test registers after calling it still find it open.
// Tests that need their own bespoke rows use this; ones that fit a throwaway
// city should use New.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// ClosedPool is a pool whose database is gone: every query on it fails, for
// tests of how a caller copes with the database being down.
func ClosedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), DatabaseURL(t))
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	return pool
}

// New creates the city named id (and an activity and area named
// after it) in the given time zone, with n users, and deletes them all when
// the test ends. Leftovers from an interrupted run are deleted first.
func New(t *testing.T, id, timezone string, n int) *Fixture {
	t.Helper()
	ctx := context.Background()
	// Pool registers pool.Close with t.Cleanup, not defer: cleanups run in
	// reverse order, so the pool is still open for the deletes registered below.
	pool := Pool(t)
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
