// Command migrate applies or rolls back the database migrations in db/migrations
// with goose: `migrate up` applies every pending one, `migrate down` rolls back
// the latest. It connects to DATABASE_URL, which `make migrate-up` and
// `make migrate-down` point at the local Supabase database.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/db"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) != 1 || (args[0] != "up" && args[0] != "down") {
		return errors.New("usage: migrate up|down")
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	if args[0] == "up" {
		return db.Up(ctx, pool)
	}
	return db.Down(ctx, pool)
}
