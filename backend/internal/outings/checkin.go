package outings

import (
	"context"
	"fmt"

	"github.com/shrujancheruku/issho/backend/internal/outings/store"
)

// CheckIn is a person going saying "I'm here" (docs/spec/outings.md, "On the
// day"): one tap with no proof of place, open from policy.CheckInOpensBeforeStart
// before the start to the plan's stated end, on the outing's city clock. The
// moment it opens counts and the moment of the end doesn't, as for an
// invitation's 48 hours. Only someone going can (ErrNotGoing); outside the
// window it is ErrCheckInClosed; no seat is ErrNotFound. Checking in again
// changes nothing, the instant included, and is allowed after the window since
// it records nothing new. It returns the seat as Mine shows it, so the person
// sees who else has checked in.
func (s *Service) CheckIn(ctx context.Context, accountID, outingID string) (Seat, error) {
	if !uuid.MatchString(outingID) {
		return Seat{}, ErrNotFound
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Seat{}, fmt.Errorf("outings: check in: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a rollback after commit is a no-op
	q := store.New(tx)
	o, seat, err := s.lockSeat(ctx, q, accountID, outingID)
	if err != nil {
		return Seat{}, err
	}
	if SeatState(seat.State) != SeatGoing {
		return Seat{}, ErrNotGoing
	}
	if seat.CheckedInAt != nil {
		return s.seat(ctx, accountID, outingID)
	}
	w, err := windowOf(o.Date, o.StartsAt, o.EndsAt, o.TimeZone)
	if err != nil {
		return Seat{}, fmt.Errorf("outings: check in: %w", err)
	}
	now := s.clock.Now()
	if now.Before(w.checkInOpens) || !now.Before(w.end) {
		return Seat{}, ErrCheckInClosed
	}
	if err := q.CheckInSeat(ctx, store.CheckInSeatParams{OutingID: outingID, AccountID: accountID, CheckedInAt: ptr(now)}); err != nil {
		return Seat{}, fmt.Errorf("outings: check in: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Seat{}, fmt.Errorf("outings: check in: %w", err)
	}
	return s.seat(ctx, accountID, outingID)
}
