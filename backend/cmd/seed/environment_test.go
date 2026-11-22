package main

import (
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

func markDev(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `do $$ begin execute format('alter database %I set issho.environment = %L', current_database(), 'dev'); end $$`); err != nil {
		t.Fatal(err)
	}
}

// The marker is the database's stored setting, which a session can't change.
// A session `SET` (what a URL's `options` or PGOPTIONS does at connect) on an
// unmarked database must not read as dev, and on a marked one it must not
// read as anything else.
func TestCheckEnvironmentReadsTheStoredSettingNotTheSession(t *testing.T) {
	ctx := context.Background()

	t.Run("a session SET on an unmarked database is refused", func(t *testing.T) {
		pool := testdb.NewDatabase(t)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx) //nolint:errcheck
		if _, err := tx.Exec(ctx, `set issho.environment = 'dev'`); err != nil {
			t.Fatal(err)
		}
		if err := checkEnvironment(ctx, tx); err == nil {
			t.Fatal("a session setting passed for the database's marker")
		}
	})

	t.Run("a session SET on a database marked dev changes nothing", func(t *testing.T) {
		pool := testdb.NewDatabase(t)
		markDev(t, pool)
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx) //nolint:errcheck
		if _, err := tx.Exec(ctx, `set issho.environment = 'prod'`); err != nil {
			t.Fatal(err)
		}
		if err := checkEnvironment(ctx, tx); err != nil {
			t.Fatalf("the stored marker was masked by a session setting: %v", err)
		}
	})

	t.Run("a setting for one role is not the database's marker", func(t *testing.T) {
		pool := testdb.NewDatabase(t)
		if _, err := pool.Exec(ctx, `do $$ begin execute format('alter role postgres in database %I set issho.environment = %L', current_database(), 'dev'); end $$`); err != nil {
			t.Fatal(err)
		}
		if err := checkEnvironment(ctx, pool); err == nil {
			t.Fatal("a per-role setting passed for the database's marker")
		}
	})
}

// Hosted dev connects as a role that is not a superuser, so the stored setting
// has to be readable by one. If it weren't, the check could only fail closed.
func TestCheckEnvironmentIsReadableByANonSuperuser(t *testing.T) {
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	markDev(t, pool)
	const role = "seed_reader_test"
	if _, err := pool.Exec(ctx, fmt.Sprintf(`create role %s login password 'reader'`, role)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `drop role `+role) })

	u, err := url.Parse(testdb.URL(pool))
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.UserPassword(role, "reader")
	reader, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reader.Close)

	var super bool
	if err := reader.QueryRow(ctx, `select rolsuper from pg_roles where rolname = current_user`).Scan(&super); err != nil || super {
		t.Fatalf("the reader role is a superuser (%v) or unreadable: %v", super, err)
	}
	if err := checkEnvironment(ctx, reader); err != nil {
		t.Fatalf("a non-superuser could not read the marker: %v", err)
	}
}
