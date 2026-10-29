// A plan's life: a draft (in a city under review) is approved or rejected;
// an open plan runs rounds, each scheduled (or proposed, then approved) into
// an event whose invitations end the round; it closes as done, without a
// group, or withdrawn. Every transition is here or calls in here.

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

// Plan states (app.plans.status).
const (
	PlanDraft  = "draft" // waiting for an admin, in a city under review
	PlanOpen   = "open"
	PlanClosed = "closed"
)

// Close reasons (app.plans.closed_reason).
const (
	ClosedDone      = "done"      // scheduled, nobody left waiting
	ClosedNoGroup   = "no_group"  // the deadline passed and no group could form
	ClosedWithdrawn = "withdrawn" // taken down
	// ClosedRejected is a draft an admin turned down. It never had signups
	// (drafts aren't in the feed), so nobody is told and no screen shows it.
	ClosedRejected = "rejected"
)

// failedReasons are the closings that keep a plan from being suggested
// again during the cool-down (Timing.Cooldown).
var failedReasons = []string{ClosedNoGroup, ClosedRejected}

var (
	// ErrNotDraft means the plan isn't a draft waiting for review.
	ErrNotDraft = errors.New("not a draft")
	// ErrNotOpen means the plan isn't open.
	ErrNotOpen = errors.New("not an open plan")
)

// startRound sends a round's invitations, for the scheduler (SaveEvent) and
// an approved proposal alike: everyone invited stops waiting and is told;
// then the plan moves to its next round (next, asked for only then) if
// anyone matching would still place is waiting, or closes as done. closed
// says which. It runs in the caller's transaction.
func startRound(ctx context.Context, q *dbgen.Queries, planID, eventID uuid.UUID, invited []uuid.UUID, next func() (Window, error), now time.Time) (closed bool, err error) {
	if err := q.DeleteSignups(ctx, dbgen.DeleteSignupsParams{PlanID: planID, UserIds: invited}); err != nil {
		return false, err
	}
	if err := notify(ctx, q, NotifyInvited, eventID, invited); err != nil {
		return false, err
	}
	// People who left the city, deleted their account or were suspended
	// still have signup rows, but no round would place them.
	waiting, err := q.CountWaitingSignups(ctx, planID)
	if err != nil {
		return false, err
	}
	if waiting == 0 {
		return true, Close(ctx, q, planID, ClosedDone, now)
	}
	w, err := next()
	if err != nil {
		return false, err
	}
	return false, NextRound(ctx, q, planID, w)
}

// NextRound moves a plan to its next round, keeping whoever is still signed up.
func NextRound(ctx context.Context, q *dbgen.Queries, planID uuid.UUID, w Window) error {
	return q.NextPlanRound(ctx, dbgen.NextPlanRoundParams{
		ID:             planID,
		WindowStart:    date(w.Start),
		WindowDays:     int16(w.Days),
		SignupDeadline: pgtype.Timestamptz{Time: w.Deadline, Valid: true},
	})
}

// Close stops an open plan taking signups at the given time; reason is one
// of the Closed* constants. A plan that isn't open is left as it is.
func Close(ctx context.Context, q *dbgen.Queries, planID uuid.UUID, reason string, at time.Time) error {
	_, err := closePlan(ctx, q, planID, reason, at)
	return err
}

func closePlan(ctx context.Context, q *dbgen.Queries, planID uuid.UUID, reason string, at time.Time) (bool, error) {
	n, err := q.ClosePlan(ctx, dbgen.ClosePlanParams{ID: planID, Reason: reason, ClosedAt: pgtype.Timestamptz{Time: at, Valid: true}})
	return n > 0, err
}

// ApproveDraft opens a draft by an admin. A draft may have waited days, so
// its round starts now, as if just suggested.
func (s *Service) ApproveDraft(ctx context.Context, planID, by uuid.UUID, t Timing, now time.Time) error {
	q := dbgen.New(s.DB)
	p, err := q.GetReviewPlan(ctx, planID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	loc, err := location(ctx, q, p.CityID)
	if err != nil {
		return err
	}
	w := t.Round(now, loc)
	n, err := q.ApproveDraft(ctx, dbgen.ApproveDraftParams{
		ID: planID, WindowStart: date(w.Start), WindowDays: int16(w.Days),
		SignupDeadline: pgtype.Timestamptz{Time: w.Deadline, Valid: true},
		ReviewedBy:     pgtype.UUID{Bytes: by, Valid: true}, At: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotDraft
	}
	return nil
}

// RejectDraft closes a draft an admin turned down (ClosedRejected).
func (s *Service) RejectDraft(ctx context.Context, planID, by uuid.UUID, now time.Time) error {
	n, err := dbgen.New(s.DB).RejectDraft(ctx, dbgen.RejectDraftParams{
		ID: planID, Reason: ClosedRejected, ReviewedBy: pgtype.UUID{Bytes: by, Valid: true}, At: pgtype.Timestamptz{Time: now, Valid: true},
	})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotDraft
	}
	return nil
}

// WithdrawPlan takes an open plan down (an admin).
func (s *Service) WithdrawPlan(ctx context.Context, planID uuid.UUID, now time.Time) error {
	closed, err := closePlan(ctx, dbgen.New(s.DB), planID, ClosedWithdrawn, now)
	if err != nil {
		return err
	}
	if !closed {
		return ErrNotOpen
	}
	return nil
}
