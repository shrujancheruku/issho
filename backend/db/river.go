package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivermigrate"
)

// RiverSchema holds River's job tables (cmd/worker), apart from app's.
const RiverSchema = "river"

// RiverVersion is the River schema version migration 00002 applies. When an
// upgrade of River adds a schema migration, add a goose migration that
// migrates up to the new version, so goose keeps owning the order (and CI's
// up → down → up covers it).
const RiverVersion = 8

func init() {
	// A Go migration, because River ships its schema as its own versioned
	// migrations. No file named 00002_river.go exists in Migrations, so
	// goose takes the registered migration as it is. It runs outside a goose
	// transaction: River applies each of its migrations in its own (some
	// can't share one) and records them, so a rerun after a failure resumes.
	goose.AddNamedMigrationNoTxContext("00002_river.go", riverUp, riverDown)
}

// RollbackGuard refuses (raises) while app.users has rows, and passes when app
// is already gone. The checks are nested: PL/pgSQL plans a whole IF condition
// as one query, so `to_regclass(...) is not null and exists (select ... from
// app.users)` fails to resolve app.users when it doesn't exist.
const RollbackGuard = `do $$ begin
  if to_regclass('app.users') is not null then
    if exists (select 1 from app.users) then
      raise exception 'refusing to roll back: app.users has rows (production only migrates forward)';
    end if;
  end if;
end $$`

func riverUp(ctx context.Context, db *sql.DB) error {
	// Revoked as in 00001_baseline: Supabase's default privileges would otherwise
	// let anon/authenticated reach the job tables through the Data API.
	stmts := []string{
		"create schema if not exists " + RiverSchema,
		`do $$ begin
		   if exists (select 1 from pg_roles where rolname = 'anon') then
		     revoke all on schema ` + RiverSchema + ` from anon, authenticated;
		   end if;
		 end $$`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return err
		}
	}
	m, err := rivermigrate.New(riverdatabasesql.New(db), &rivermigrate.Config{Schema: RiverSchema})
	if err != nil {
		return err
	}
	if _, err := m.Migrate(ctx, rivermigrate.DirectionUp, &rivermigrate.MigrateOpts{TargetVersion: RiverVersion}); err != nil {
		return fmt.Errorf("river migrations: %w", err)
	}
	return nil
}

func riverDown(ctx context.Context, db *sql.DB) error {
	// The baseline's Down refuses when accounts exist, but this Down runs
	// first, and dropping the schema can't be undone by a failure after it:
	// make the same check here. Jobs are recreated by the next up, accounts
	// aren't.
	if _, err := db.ExecContext(ctx, RollbackGuard); err != nil {
		return err
	}
	// Dropping the schema removes River's tables and its own version table,
	// along with any jobs: they're all recreated by the next up.
	_, err := db.ExecContext(ctx, "drop schema if exists "+RiverSchema+" cascade")
	return err
}
