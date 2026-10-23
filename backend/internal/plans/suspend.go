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
// place cancelled, with the usual effect on the group), and crews (they
// leave an active one, which closes if one member is left, and decline
// open offers). Outings already under way are left alone; the API and
// chat refuse the person anyway.
//
// It's safe to run again: each step only touches what's still there. Each
// outing and crew is its own transaction, like the person doing it
// themselves, so one failure leaves the rest done and a retry finishes.
// Nothing is put back when the suspension is lifted.
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
			if m.EventStatus != "scheduled" || m.GroupStatus == GroupCancelled || (m.StartsAt.Valid && !now.Before(m.StartsAt.Time)) {
				return nil
			}
			switch m.Response {
			case Invited:
				return setResponse(ctx, q, m, userID, eventID, Declined, now)
			case Confirmed:
				return setResponse(ctx, q, m, userID, eventID, Dropped, now)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	crews, err := q.ListUserOpenCrewMemberships(ctx, userID)
	if err != nil {
		return err
	}
	for _, crewID := range crews {
		if err := pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
			return removeFromCrew(ctx, dbgen.New(tx), crewID, userID, now)
		}); err != nil {
			return err
		}
	}
	return nil
}

// removeFromCrew is LeaveCrew (or declining an offer) for someone
// suspended. No leave reason is stored: app.crew_leaves is why people
// chose to leave.
func removeFromCrew(ctx context.Context, q *dbgen.Queries, crewID, userID uuid.UUID, now time.Time) error {
	m, err := q.LockCrewMembership(ctx, dbgen.LockCrewMembershipParams{CrewID: crewID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	at := pgtype.Timestamptz{Time: now, Valid: true}
	switch {
	case m.Response == CrewOffered && m.Status == CrewOffered:
		// The offer is decided when it closes, with whoever accepted.
		return q.SetCrewResponse(ctx, dbgen.SetCrewResponseParams{CrewID: crewID, UserID: userID, Response: CrewDeclined, At: at})
	case m.Response == CrewAccepted && (m.Status == CrewOffered || m.Status == CrewActive):
		if err := q.SetCrewResponse(ctx, dbgen.SetCrewResponseParams{CrewID: crewID, UserID: userID, Response: CrewLeft, At: at}); err != nil {
			return err
		}
		if m.Status != CrewActive {
			return nil
		}
		left, err := q.CountCrewMembers(ctx, dbgen.CountCrewMembersParams{CrewID: crewID, Response: CrewAccepted})
		if err != nil || left >= 2 {
			return err
		}
		return q.SetCrewStatus(ctx, dbgen.SetCrewStatusParams{ID: crewID, Status: CrewClosed, At: at})
	}
	return nil
}
