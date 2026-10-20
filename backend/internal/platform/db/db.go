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
	// Keep goose's bookkeeping table next to the app tables, not in public.
	goose.SetTableName("app_goose_db_version")
	return goose.RunContext(ctx, command, sqlDB, "migrations", args...)
}
