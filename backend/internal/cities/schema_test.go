package cities_test

import (
	"context"
	"strings"
	"testing"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// The baseline migration's schema, checked against the database CI and
// `make test-db` migrate (#160). Lives here because the packages that run
// against a database are listed in scripts/db-checks.sh and this one owns the
// tables the baseline touched most (cities, areas).
func TestIntegrationBaselineSchema(t *testing.T) {
	f := testfixture.New(t, "schematest", "UTC", 1) // one app.users row
	ctx := context.Background()

	t.Run("columns left out of the baseline stay out", func(t *testing.T) {
		for _, c := range [][2]string{
			{"profiles", "reliability"}, {"users", "deleted_at"}, {"users", "verified_at"},
			{"venues", "tags"}, {"venues", "discovered_at"}, {"cities", "matching_weights"},
		} {
			var n int
			if err := f.Pool.QueryRow(ctx,
				"select count(*) from information_schema.columns where table_schema = 'app' and table_name = $1 and column_name = $2",
				c[0], c[1]).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 0 {
				t.Errorf("app.%s.%s exists", c[0], c[1])
			}
		}
		for _, c := range [][2]string{{"cities", "matching"}, {"cities", "meeting_places"}} {
			var n int
			if err := f.Pool.QueryRow(ctx,
				"select count(*) from information_schema.columns where table_schema = 'app' and table_name = $1 and column_name = $2",
				c[0], c[1]).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Errorf("app.%s.%s is missing", c[0], c[1])
			}
		}
	})

	t.Run("PostGIS lives in extensions, not public", func(t *testing.T) {
		var schema string
		if err := f.Pool.QueryRow(ctx,
			"select n.nspname from pg_extension e join pg_namespace n on n.oid = e.extnamespace where e.extname = 'postgis'").Scan(&schema); err != nil {
			t.Fatal(err)
		}
		if schema != "extensions" {
			t.Errorf("postgis is in schema %q, want extensions", schema)
		}
	})

	t.Run("the pending index drops notifications that ran out of attempts", func(t *testing.T) {
		var def string
		if err := f.Pool.QueryRow(ctx,
			"select indexdef from pg_indexes where schemaname = 'app' and indexname = 'notifications_pending_idx'").Scan(&def); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(def, "attempts < 5") {
			t.Errorf("notifications_pending_idx = %s", def)
		}
	})

	t.Run("River's Down guard refuses with accounts and passes once app is gone", func(t *testing.T) {
		tx, err := f.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx) //nolint:errcheck // always rolled back
		if _, err := tx.Exec(ctx, db.RollbackGuard); err == nil || !strings.Contains(err.Error(), "refusing") {
			t.Fatalf("guard with app.users rows: err = %v, want a refusal", err)
		}
		// The failed statement aborted the transaction; use a fresh one for the
		// state where app doesn't exist (as after the baseline's Down).
		_ = tx.Rollback(ctx)
		tx, err = f.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx) //nolint:errcheck // always rolled back
		if _, err := tx.Exec(ctx, "drop schema app cascade"); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, db.RollbackGuard); err != nil {
			t.Errorf("guard without the app schema: %v", err)
		}
	})

	t.Run("Down refuses while accounts exist", func(t *testing.T) {
		src, err := db.Migrations.ReadFile("migrations/00001_baseline.sql")
		if err != nil {
			t.Fatal(err)
		}
		_, down, ok := strings.Cut(string(src), "-- +goose Down")
		if !ok {
			t.Fatal("no Down section")
		}
		// In a transaction that is always rolled back: DDL is transactional, so
		// even a Down that wrongly went ahead can't drop the schema under the
		// other tests.
		tx, err := f.Pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)                                //nolint:errcheck // the rollback is the point
		_, err = tx.Conn().PgConn().Exec(ctx, down).ReadAll() // several statements: simple protocol
		if err == nil || !strings.Contains(err.Error(), "refusing") {
			t.Errorf("Down with app.users rows: err = %v, want a refusal", err)
		}
	})
}
