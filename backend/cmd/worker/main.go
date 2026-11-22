package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/jobs"
	"github.com/shrujancheruku/issho/backend/internal/logging"
	"github.com/shrujancheruku/issho/backend/internal/outings"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/serve"
	"github.com/shrujancheruku/issho/backend/internal/staff"
)

// defaultCity is the Pilot's only listed city, which people.New wants for the
// accounts it makes; the worker makes none (it reads profiles), so the value
// is never used. cmd/api has the same constant and the same lookup to come.
const defaultCity = "da-nang-hoi-an"

// stopTimeout is how long a deploy's SIGTERM waits for running jobs to finish
// before River cancels them; a canceled job is retried by the next worker.
const stopTimeout = 30 * time.Second

func main() {
	// SIGTERM is what the host sends on a deploy; SIGINT is Ctrl-C locally.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log := logging.New(os.Stdout, "worker")
	if err := run(ctx, log); err != nil {
		log.Error("exited", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	// A direct or session-pooler connection, not the transaction pooler the API
	// uses: River listens for new jobs with LISTEN/NOTIFY.
	url := os.Getenv("WORKER_DATABASE_URL")
	if url == "" {
		return errors.New("WORKER_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	emails, err := newEmailSender(os.Getenv)
	if err != nil {
		return err
	}
	// No job sends email yet; the ones that do are handed emails below.
	_ = emails

	// The real clock is built here and only here (tools/clockcheck enforces it);
	// everything below takes it as a clock.Clock. Each domain package with a job
	// registers its queue kinds on kinds and its workers on workers here.
	clk := clock.Real{}
	kinds := staff.NewKinds()
	workers := jobs.NewWorkers(clk, pool, kinds)
	cs := cities.New(pool)
	ppl := people.New(clk, pool, cs, people.Config{DefaultCity: defaultCity})
	outings.AddWorker(workers, outings.New(clk, pool, plans.NewSignups(clk, pool, cs), ppl))
	client, err := jobs.New(pool, workers, log)
	if err != nil {
		return err
	}
	if err := client.Start(ctx); err != nil {
		return err
	}

	err = serve.Run(ctx, log)

	// ctx is done by now, so stopping needs a context of its own.
	stopCtx, cancel := context.WithTimeout(context.Background(), stopTimeout)
	defer cancel()
	return errors.Join(err, client.Stop(stopCtx))
}
