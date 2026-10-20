// Package db opens the Postgres pool and runs migrations.
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/shrujancheruku/issho/backend/db"
)

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate runs a goose command ("up", "down", "status", ...) against the embedded migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool, command string, args ...string) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	return migrate(ctx, sqlDB, command, args...)
}

// goose keeps its settings in package-level globals, so concurrent Migrate calls
// with different settings are not safe. Migrations only run from cmd/tool and tests.
func migrate(ctx context.Context, sqlDB *sql.DB, command string, args ...string) error {
	goose.SetBaseFS(db.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	// goose's bookkeeping table lives in its own `goose` schema:
	//  - not `public`: Supabase's Data API exposes public tables to anonymous
	//    clients, who could insert rows and mark future migrations as applied;
	//  - not `app`: migration 00001's Down drops the app schema, which would
	//    delete the version table in the middle of the rollback.
	// (Until 2026-10-20 it was public.app_goose_db_version; see prepareVersionTable.)
	if err := prepareVersionTable(ctx, sqlDB); err != nil {
		return err
	}
	goose.SetTableName(versionTable)
	return goose.RunContext(ctx, command, sqlDB, "migrations", args...)
}

const versionTable = "goose.db_version"

// prepareVersionTable creates the goose schema (goose creates its table but not
// the schema) and moves a legacy public.app_goose_db_version table into place,
// keeping its history. Without the move, existing databases would look
// unmigrated and goose would try to re-run 00001. Both steps are no-ops once
// applied. On Supabase, a new schema has no grants for anon/authenticated.
func prepareVersionTable(ctx context.Context, sqlDB *sql.DB) error {
	_, err := sqlDB.ExecContext(ctx, `
		create schema if not exists goose;
		do $$
		begin
		  if to_regclass('public.app_goose_db_version') is not null
		     and to_regclass('goose.db_version') is null then
		    alter table public.app_goose_db_version set schema goose;
		    alter table goose.app_goose_db_version rename to db_version;
		  end if;
		end $$;`)
	if err != nil {
		return fmt.Errorf("prepare migration table: %w", err)
	}
	return nil
}
