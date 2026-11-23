// Command season plays the season (internal/season) and prints its numbers: the
// share of signups that got an outing, and the people who got none. It exits 1
// when an invariant broke or the log held a cast member's details, which is
// what CI's test of the season fails on too.
//
// It starts its own throwaway Postgres in Docker and removes it at the end, so
// it needs Docker and no database URL, and can't touch another database. Not in
// the production image.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/shrujancheruku/issho/backend/internal/season"
)

func main() { os.Exit(run()) }

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	boot, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	c, err := postgres.Run(boot, "postgres:17-alpine",
		postgres.WithDatabase("issho_season"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(time.Minute)),
	)
	if c != nil {
		defer func() { _ = testcontainers.TerminateContainer(c) }()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "season: start Postgres (is Docker running?):", err)
		return 2
	}
	dsn, err := c.ConnectionString(boot, "sslmode=disable")
	if err != nil {
		fmt.Fprintln(os.Stderr, "season:", err)
		return 2
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "season:", err)
		return 2
	}
	defer pool.Close()

	res, err := season.Run(ctx, pool, season.Options{})
	if err != nil {
		fmt.Fprintln(os.Stderr, "season:", err)
		return 2
	}
	fmt.Print(res.Scorecard)
	for _, v := range res.Violations {
		fmt.Fprintln(os.Stderr, "broken invariant:", v)
	}
	for _, l := range res.Leaks {
		fmt.Fprintln(os.Stderr, "the log holds", l)
	}
	if len(res.Violations) > 0 || len(res.Leaks) > 0 {
		return 1
	}
	return 0
}
