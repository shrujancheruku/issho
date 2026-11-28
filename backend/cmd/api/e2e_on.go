//go:build e2e

package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/clock"
	"github.com/shrujancheruku/issho/backend/internal/e2eclock"
	"github.com/shrujancheruku/issho/backend/internal/jobs"
	"github.com/shrujancheruku/issho/backend/internal/outings"
	"github.com/shrujancheruku/issho/backend/internal/rpc"
	"github.com/shrujancheruku/issho/backend/internal/staff"
)

// The end-to-end build (`go build -tags e2e`, by scripts/e2e.sh only): the API
// with a fake clock a test moves, a worker running in the same process on that
// clock, and the test-only RPCs that move it and trigger forming
// (internal/e2eclock). One process, because one fake clock can only be shared
// in one: a separate worker would need its own way to read the API's time.
// River's own scheduling (a job's scheduled_at) still reads the database's
// clock, so a job scheduled for a later instant, such as an invitation's
// expiry, does not run when the fake clock passes it; the journey answers every
// invitation, so none is waited on.

// newClock starts the fake at the system's time; a test sets its own.
func newClock() clock.Clock { return clock.NewFake(clock.Real{}.Now()) }

func e2eServices(ctx context.Context, log *slog.Logger, clk clock.Clock, acc *accounts) ([]rpc.Mount, func(), error) {
	if acc == nil {
		return nil, nil, errors.New("the e2e build needs DATABASE_URL and SUPABASE_AUTH_URL: its worker and its forming trigger run on the database")
	}
	workers := jobs.NewWorkers(clk, acc.pool, staff.NewKinds())
	outings.AddWorker(workers, acc.outings)
	client, err := jobs.New(acc.pool, workers, log)
	if err != nil {
		return nil, nil, err
	}
	if err := client.Start(ctx); err != nil {
		return nil, nil, err
	}
	stop := func() {
		// ctx is done by now (this runs as the API shuts down), so stopping needs its own.
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = client.Stop(stopCtx)
	}
	return e2eclock.Mounts(log, clk.(*clock.Fake), e2eclock.NewRiverFormer(client)), stop, nil
}
