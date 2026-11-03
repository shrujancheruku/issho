package moderation

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/platform/errs"
	"github.com/shrujancheruku/issho/backend/internal/platform/pgconv"
)

// TxInserter queues a job inside the caller's transaction (*river.Client).
type TxInserter interface {
	InsertTx(ctx context.Context, tx pgx.Tx, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
}

// Service suspends and lifts. Like the other services, its methods return
// Connect errors for the caller's mistakes; anything else is ours.
type Service struct {
	DB *pgxpool.Pool
	// Jobs queues the cleanup in the suspension's transaction (the API's
	// insert-only River client).
	Jobs TxInserter
	// Suspensions is the API's check, told once a change has committed.
	Suspensions *Suspensions
}

func (s *Service) invalidate() {
	if s.Suspensions != nil {
		s.Suspensions.Invalidate()
	}
}

// Suspend suspends the person and queues remove_suspended in one
// transaction, so a suspension never exists without its cleanup being
// queued, nor the reverse. Suspending someone already suspended changes
// nothing but queues the job again (a no-op while one is live: the job is
// unique by person), so an admin can always finish a stuck cleanup.
func (s *Service) Suspend(ctx context.Context, by, userID uuid.UUID, reason string, now time.Time) error {
	err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if _, err := q.GetPerson(ctx, userID); errors.Is(err, pgx.ErrNoRows) {
			return errs.NotFound("no such person")
		} else if err != nil {
			return err
		}
		if _, err := q.InsertSuspension(ctx, dbgen.InsertSuspensionParams{
			UserID: userID, Reason: reason, CreatedBy: by, At: pgconv.Time(now),
		}); err != nil {
			return err
		}
		_, err := s.Jobs.InsertTx(ctx, tx, RemoveSuspendedArgs{UserID: userID, At: now}, nil)
		return err
	})
	if err != nil {
		return err
	}
	// After the commit: invalidating earlier would let a concurrent check
	// reload the list before the suspension is visible and keep that answer.
	s.invalidate()
	return nil
}

// Lift ends the person's open suspension. Nothing they were removed from is
// put back.
func (s *Service) Lift(ctx context.Context, by, userID uuid.UUID, note string, now time.Time) error {
	n, err := dbgen.New(s.DB).LiftSuspension(ctx, dbgen.LiftSuspensionParams{
		UserID: userID, LiftedBy: pgUUID(by), Note: note, At: pgconv.Time(now),
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return errs.Refused("not suspended")
	}
	s.invalidate()
	return nil
}
