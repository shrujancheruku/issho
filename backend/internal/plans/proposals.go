package plans

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// Group review (00022): in a city under review the scheduler's round waits
// as a proposal (SaveEvent with Propose). These change and settle it.

var (
	ErrNoProposal      = errors.New("no such proposal")
	ErrNotProposal     = errors.New("not a proposal waiting for review")
	ErrProposalStale   = errors.New("the plan has closed or moved on since")
	ErrProposalTooLate = errors.New("too late: people need until the day before to answer; set a later start")
	ErrNoSuchGroup     = errors.New("no such group in this event")
	ErrNotInProposal   = errors.New("not in this proposal")
)

// MaxMeetingPoint bounds an admin's meeting-point text.
const MaxMeetingPoint = 300

// meetingPointContext tells the translator what a meeting point is.
const meetingPointContext = "where exactly a small group meets for an outing, e.g. which exit of a station"

// lockProposal locks a proposal and checks it's still waiting.
func lockProposal(ctx context.Context, q *dbgen.Queries, eventID uuid.UUID) (dbgen.LockProposalRow, error) {
	e, err := q.LockProposal(ctx, eventID)
	if errors.Is(err, pgx.ErrNoRows) {
		return e, ErrNoProposal
	}
	if err != nil {
		return e, err
	}
	if e.Status != EventProposed {
		return e, ErrNotProposal
	}
	return e, nil
}

// ApproveProposal sends a proposal's invitations, as if the scheduler had
// just scheduled it: people who withdrew since (or were suspended) are left
// out, everyone else is invited and told, their signups end, and the plan
// moves to its next round for whoever is still waiting (or closes as done).
func (s *Service) ApproveProposal(ctx context.Context, eventID, by uuid.UUID, t Timing, now time.Time) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		e, err := lockProposal(ctx, q, eventID)
		if err != nil {
			return err
		}
		round, err := q.LockOpenPlan(ctx, e.PlanID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && round != e.Round) {
			return ErrProposalStale
		}
		if err != nil {
			return err
		}
		ev, err := q.GetReviewEvent(ctx, eventID)
		if err != nil {
			return err
		}
		if !now.Before(ev.ConfirmBy.Time) {
			return ErrProposalTooLate
		}
		if err := q.DropMembersWithoutSignup(ctx, dbgen.DropMembersWithoutSignupParams{EventID: eventID, PlanID: e.PlanID}); err != nil {
			return err
		}
		if err := q.DeleteEmptyGroups(ctx, eventID); err != nil {
			return err
		}
		if err := checkGroupSizes(ctx, q, eventID); err != nil {
			return err
		}
		invited, err := q.ListEventMemberIDs(ctx, eventID)
		if err != nil {
			return err
		}
		if err := q.ApproveProposalEvent(ctx, dbgen.ApproveProposalEventParams{
			ID: eventID, ReviewedBy: pgtype.UUID{Bytes: by, Valid: true}, At: pgtype.Timestamptz{Time: now, Valid: true},
		}); err != nil {
			return err
		}
		_, err = startRound(ctx, q, e.PlanID, eventID, invited, func() (Window, error) {
			loc, err := time.LoadLocation(ev.Timezone)
			if err != nil {
				return Window{}, err
			}
			return t.Round(now, loc), nil
		}, now)
		return err
	})
}

// checkGroupSizes refuses a proposal with a group too small to go ahead
// (after moving people, or people leaving): fewer than its minimum.
func checkGroupSizes(ctx context.Context, q *dbgen.Queries, eventID uuid.UUID) error {
	groups, err := q.ListReviewEventGroups(ctx, eventID)
	if err != nil {
		return err
	}
	members, err := q.ListReviewEventMembers(ctx, eventID)
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		return ErrGroupTooSmall{Need: 2}
	}
	for _, g := range groups {
		n := 0
		for _, m := range members {
			if m.GroupID == g.ID {
				n++
			}
		}
		if n < int(g.MinAttendance) {
			return ErrGroupTooSmall{Have: n, Need: int(g.MinAttendance)}
		}
	}
	return nil
}

// ErrGroupTooSmall: a group has fewer people than it needs to go ahead.
type ErrGroupTooSmall struct{ Have, Need int }

func (e ErrGroupTooSmall) Error() string {
	return fmt.Sprintf("a group has %d people and needs at least %d: move someone in, or reject the proposal", e.Have, e.Need)
}

// RejectProposal deletes a proposal. Everyone stays signed up, and the
// scheduler leaves the plan alone for ProposalRest.
func (s *Service) RejectProposal(ctx context.Context, eventID uuid.UUID, now time.Time) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		e, err := lockProposal(ctx, q, eventID)
		if err != nil {
			return err
		}
		if err := q.DeleteProposal(ctx, eventID); err != nil {
			return err
		}
		return q.SetProposalRejected(ctx, dbgen.SetProposalRejectedParams{ID: e.PlanID, At: pgtype.Timestamptz{Time: now, Valid: true}})
	})
}

// MoveProposalMember moves someone to another group of the proposal.
func (s *Service) MoveProposalMember(ctx context.Context, eventID, userID, groupID uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if _, err := lockProposal(ctx, q, eventID); err != nil {
			return err
		}
		groups, err := q.ListReviewEventGroups(ctx, eventID)
		if err != nil {
			return err
		}
		found := false
		for _, g := range groups {
			found = found || g.ID == groupID
		}
		if !found {
			return ErrNoSuchGroup
		}
		n, err := q.MoveProposalMember(ctx, dbgen.MoveProposalMemberParams{EventID: eventID, UserID: userID, GroupID: groupID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotInProposal
		}
		return nil
	})
}

// RemoveProposalMember takes someone out of the proposal. They stay signed
// up, for the plan's next round.
func (s *Service) RemoveProposalMember(ctx context.Context, eventID, userID uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if _, err := lockProposal(ctx, q, eventID); err != nil {
			return err
		}
		n, err := q.DeleteProposalMember(ctx, dbgen.DeleteProposalMemberParams{EventID: eventID, UserID: userID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotInProposal
		}
		return q.DeleteEmptyGroups(ctx, eventID)
	})
}

// SetProposalStart moves a proposal to another local date and time in the
// city. Its part of the day follows the time, and people have until the
// start of the day before to answer, which must still be ahead.
func (s *Service) SetProposalStart(ctx context.Context, eventID uuid.UUID, day, clock string, now time.Time) error {
	return pgx.BeginFunc(ctx, s.DB, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if _, err := lockProposal(ctx, q, eventID); err != nil {
			return err
		}
		ev, err := q.GetReviewEvent(ctx, eventID)
		if err != nil {
			return err
		}
		loc, err := time.LoadLocation(ev.Timezone)
		if err != nil {
			return err
		}
		start, err := time.ParseInLocation(time.DateOnly+" 15:04", day+" "+clock, loc)
		if err != nil {
			return ErrBadStart
		}
		o := matching.Occasion{Date: time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC), Block: blockAt(start)}
		by := confirmBy(o, loc)
		if !now.Before(by) {
			return ErrProposalTooLate
		}
		return q.SetProposalStart(ctx, dbgen.SetProposalStartParams{
			ID: eventID, Date: date(o.Date), Block: string(o.Block),
			StartsAt:  pgtype.Timestamptz{Time: start, Valid: true},
			ConfirmBy: pgtype.Timestamptz{Time: by, Valid: true},
		})
	})
}

// ErrBadStart: a start that isn't a date (YYYY-MM-DD) and time (HH:MM).
var ErrBadStart = errors.New("start: a date YYYY-MM-DD and a time HH:MM")

// blockAt is the part of the day a local start time falls in.
func blockAt(t time.Time) matching.Block {
	switch h := t.Hour(); {
	case h < 12:
		return matching.Morning
	case h < 17:
		return matching.Afternoon
	default:
		return matching.Evening
	}
}

// SetMeetingPoint sets where exactly a group meets (English; "" clears it),
// for a proposal or a scheduled event. It's stored in the event's own
// languages too when a translator is configured; otherwise people see the
// English. Members who confirmed see it from 24 h before the start.
func (s *Service) SetMeetingPoint(ctx context.Context, groupID uuid.UUID, text string) error {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) > MaxMeetingPoint {
		return ErrMeetingPointTooLong
	}
	q := dbgen.New(s.DB)
	g, err := q.GetGroupEventStatus(ctx, groupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNoSuchGroup
	}
	if err != nil {
		return err
	}
	if g.Status != EventProposed && g.Status != EventScheduled {
		return ErrNotProposal
	}
	var raw []byte
	if text != "" {
		raw = english(text)
		if s.Translations != nil && s.Translations.Translator != nil {
			if filled, err := s.Translations.Prefill(ctx, text, meetingPointContext, []string{g.PrimaryLanguage, deref(g.SecondaryLanguage)}); err == nil {
				raw = filled
			}
		}
	}
	n, err := q.SetGroupMeetingPoint(ctx, dbgen.SetGroupMeetingPointParams{ID: groupID, MeetingPoint: raw})
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotProposal
	}
	return nil
}

// ErrMeetingPointTooLong: over MaxMeetingPoint characters.
var ErrMeetingPointTooLong = errors.New("meeting point: at most 300 characters")
