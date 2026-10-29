package plans

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

// RemoveFromUpcoming takes a suspended person out of everything they'd
// joined (ModerationService.SuspendPerson): their signups to open plans,
// outings that haven't started (an invitation is declined, a confirmed
// place cancelled, with the usual effect on the group). Outings already
// under way are left alone; the API and chat refuse the person anyway.
// Crews are crews.Service.RemoveFromAll.
//
// It's safe to run again: each step only touches what's still there. Each
// outing is its own transaction, like the person doing it themselves, so
// one failure leaves the rest done and a retry finishes. Nothing is put
// back when the suspension is lifted.
func (s *Service) RemoveFromUpcoming(ctx context.Context, userID uuid.UUID, now time.Time) error {
	q := dbgen.New(s.DB)
	plans, err := q.ListUserOpenSignups(ctx, userID)
	if err != nil {
		return err
	}
	for _, planID := range plans {
		if err := Withdraw(ctx, q, planID, userID); err != nil {
			return err
		}
	}

	events, err := q.ListUpcomingMemberships(ctx, dbgen.ListUpcomingMembershipsParams{
		UserID: userID, Now: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return err
	}
	for _, eventID := range events {
		if err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
			q := dbgen.New(tx)
			m, err := q.LockMembership(ctx, dbgen.LockMembershipParams{EventID: eventID, UserID: userID})
			if errors.Is(err, pgx.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			next, ok := lockedMembership(m).Leave(now)
			if !ok {
				return nil
			}
			return setResponse(ctx, q, m, userID, eventID, next, now)
		}); err != nil {
			return err
		}
	}
	return nil
}
