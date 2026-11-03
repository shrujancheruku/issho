package moderation

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/adminjobs"
	"github.com/shrujancheruku/issho/backend/internal/crews"
	"github.com/shrujancheruku/issho/backend/internal/plans"
)

// KindRemoveSuspended is the job that takes a suspended person out of what
// they'd joined.
const KindRemoveSuspended = "remove_suspended"

const (
	// RemoveSuspendedQueue is the worker's maintenance queue (one worker, so
	// two people's cleanups never overlap).
	RemoveSuspendedQueue = adminjobs.Queue
	// RemoveSuspendedMaxAttempts is generous because a failed attempt leaves
	// a suspended person half-removed, and every step is safe to repeat.
	RemoveSuspendedMaxAttempts = 10
	// RemoveSuspendedTimeout: a few queries and a transaction per outing and
	// crew the person is in.
	RemoveSuspendedTimeout = 2 * time.Minute
)

// RemoveSuspendedArgs: At is when the suspension was made, the time the
// removal is done as. Unique by person only (river:"unique"): suspending
// someone twice while the job is live queues one job, not two.
type RemoveSuspendedArgs struct {
	UserID uuid.UUID `json:"user_id" river:"unique"`
	At     time.Time `json:"at"`
}

func (RemoveSuspendedArgs) Kind() string { return KindRemoveSuspended }

func (RemoveSuspendedArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: RemoveSuspendedQueue, MaxAttempts: RemoveSuspendedMaxAttempts,
		UniqueOpts: river.UniqueOpts{ByArgs: true, ByState: adminjobs.LiveStates}}
}

// Remover runs remove_suspended.
type Remover struct {
	Q     *dbgen.Queries
	Plans *plans.Service
	Crews *crews.Service
}

// Run takes the person out of their open signups, outings that haven't
// started and crews. It's safe to run again (both steps only touch what's
// still there), so River's retries finish a partial run. If the suspension
// was lifted before the job ran, it does nothing: the person is back in good
// standing, and nothing is removed after the fact.
func (r *Remover) Run(ctx context.Context, args RemoveSuspendedArgs) error {
	suspended, err := r.Q.IsSuspended(ctx, args.UserID)
	if err != nil {
		return err
	}
	if !suspended {
		return nil
	}
	if err := r.Plans.RemoveFromUpcoming(ctx, args.UserID, args.At); err != nil {
		return err
	}
	return r.Crews.RemoveFromAll(ctx, args.UserID, args.At)
}
