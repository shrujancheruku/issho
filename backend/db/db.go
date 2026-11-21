// Package db owns the schema: the goose migrations, embedded so any binary can
// apply them, and the Up and Down that run them.
package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// versionTable keeps goose's bookkeeping out of `public` and out of `app`.
const versionTable = "goose.goose_db_version"

func provider(ctx context.Context, pool *pgxpool.Pool) (*goose.Provider, error) {
	// goose creates its version table but not the schema it is in.
	if _, err := pool.Exec(ctx, "create schema if not exists goose"); err != nil {
		return nil, fmt.Errorf("create schema goose: %w", err)
	}
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return nil, err
	}
	return goose.NewProvider(goose.DialectPostgres, stdlib.OpenDBFromPool(pool), sub,
		goose.WithTableName(versionTable))
}

// Up applies every migration that is pending.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	p, err := provider(ctx, pool)
	if err != nil {
		return err
	}
	_, err = p.Up(ctx)
	return err
}

// Down rolls back the latest migration, as `goose down` does.
func Down(ctx context.Context, pool *pgxpool.Pool) error {
	p, err := provider(ctx, pool)
	if err != nil {
		return err
	}
	_, err = p.Down(ctx)
	return err
}

// DownAll rolls back every migration.
func DownAll(ctx context.Context, pool *pgxpool.Pool) error {
	p, err := provider(ctx, pool)
	if err != nil {
		return err
	}
	_, err = p.DownTo(ctx, 0)
	return err
}
