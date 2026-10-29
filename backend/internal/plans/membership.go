package plans

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/internal/chat"
)

// Membership is one person's place in an event: their answer, their group's
// and the event's state, and the event's times. It holds every rule about
// what they can do and when (answer, cancel, see the meeting point, check
// in, chat, give feedback, report), so the service only loads, locks and
// stores, and the API sends the resulting windows to the app.
type Membership struct {
	Response    string // Invited, Confirmed, Declined or Dropped
	GroupStatus string // GroupInviting, GroupConfirmed or GroupCancelled
	EventStatus string // EventScheduled, EventDone or EventCancelled
	StartsAt    time.Time
	ConfirmBy   time.Time
}

// newMembership reads a membership from a query's columns.
func newMembership(response, groupStatus, eventStatus string, startsAt, confirmBy pgtype.Timestamptz) Membership {
	return Membership{
		Response: response, GroupStatus: groupStatus, EventStatus: eventStatus,
		StartsAt: startsAt.Time, ConfirmBy: confirmBy.Time,
	}
}

// Answer is the response an action leads to: confirm (or change a decline)
// and decline until ConfirmBy; cancel a confirmed place until the start.
func (m Membership) Answer(a Action, now time.Time) (string, error) {
	if m.EventStatus != EventScheduled || m.GroupStatus == GroupCancelled {
		return "", ErrEventOver
	}
	switch a {
	case Confirm, Decline:
		if !now.Before(m.ConfirmBy) {
			return "", ErrTooLate
		}
		if a == Confirm && (m.Response == Invited || m.Response == Declined) {
			return Confirmed, nil
		}
		if a == Decline && m.Response == Invited {
			return Declined, nil
		}
		return "", ErrNotNow
	case Cancel:
		if m.started(now) {
			return "", ErrTooLate
		}
		if m.Response != Confirmed {
			return "", ErrNotNow
		}
		return Dropped, nil
	}
	return "", fmt.Errorf("unknown action %d", a)
}

// Leave is the response that takes someone out of an outing that hasn't
// started, whatever the deadline (a suspension): an invitation is declined,
// a confirmed place cancelled. ok is false when there's nothing to leave.
func (m Membership) Leave(now time.Time) (next string, ok bool) {
	if m.EventStatus != EventScheduled || m.GroupStatus == GroupCancelled || m.started(now) {
		return "", false
	}
	switch m.Response {
	case Invited:
		return Declined, true
	case Confirmed:
		return Dropped, true
	}
	return "", false
}

func (m Membership) started(now time.Time) bool {
	return !m.StartsAt.IsZero() && !now.Before(m.StartsAt)
}

// going is whether they confirmed and their group and event weren't
// cancelled: who may check in, chat and see the meeting point.
func (m Membership) going() error {
	if m.Response != Confirmed {
		return ErrNotGoing
	}
	if m.EventStatus == EventCancelled || m.GroupStatus == GroupCancelled {
		return ErrEventOver
	}
	return nil
}

// RevealAt is when the meeting point is shown (zero without a start time).
func (m Membership) RevealAt() time.Time {
	if m.StartsAt.IsZero() {
		return time.Time{}
	}
	return m.StartsAt.Add(-RevealBefore)
}

// Revealed is whether they may see the meeting point now: they're going,
// and it's at most RevealBefore until the start.
func (m Membership) Revealed(now time.Time) bool {
	return m.going() == nil && !m.StartsAt.IsZero() && !now.Before(m.RevealAt())
}

// CheckInWindow is when check-in is open, both ends included (zero without
// a start time).
func (m Membership) CheckInWindow() (from, until time.Time) {
	if m.StartsAt.IsZero() {
		return time.Time{}, time.Time{}
	}
	return m.StartsAt.Add(-CheckInBefore), m.StartsAt.Add(CheckInAfter)
}

// CheckIn is why they can't check in now, or nil.
func (m Membership) CheckIn(now time.Time) error {
	if err := m.going(); err != nil {
		return err
	}
	from, until := m.CheckInWindow()
	if from.IsZero() || now.Before(from) || now.After(until) {
		return ErrCheckInClosed
	}
	return nil
}

// Chat is when the group chat is open for writing (zero times without a
// start), or why they can't read it at all.
func (m Membership) Chat(now time.Time) (opensAt, closesAt time.Time, open bool, err error) {
	if err := m.going(); err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	if m.StartsAt.IsZero() {
		return time.Time{}, time.Time{}, false, nil
	}
	opensAt, closesAt = m.StartsAt.Add(-chat.OpensBefore), m.StartsAt.Add(chat.ClosesAfter)
	return opensAt, closesAt, !now.Before(opensAt) && now.Before(closesAt), nil
}

// FeedbackWindow is when they can give feedback, from FeedbackAfter to
// FeedbackFor after the start (the end excluded), or why they can't at
// all: only someone who went ahead with their group can.
func (m Membership) FeedbackWindow() (from, until time.Time, err error) {
	switch {
	case m.Response != Confirmed:
		return from, until, ErrNotGoing
	case m.GroupStatus != GroupConfirmed || m.EventStatus == EventCancelled:
		return from, until, ErrEventOver
	case m.StartsAt.IsZero():
		return from, until, ErrFeedbackClosed
	}
	return m.StartsAt.Add(FeedbackAfter), m.StartsAt.Add(FeedbackFor), nil
}

// Feedback is why they can't give feedback now, or nil.
func (m Membership) Feedback(now time.Time) error {
	from, until, err := m.FeedbackWindow()
	if err != nil {
		return err
	}
	if now.Before(from) || !now.Before(until) {
		return ErrFeedbackClosed
	}
	return nil
}

// confirmedOnce is whether they confirmed, even if they cancelled since:
// who sees the feedback form and can report someone.
func (m Membership) confirmedOnce() bool {
	return m.Response == Confirmed || m.Response == Dropped
}

// ReportUntil is the last moment they can report someone in their group
// (zero: no limit, as the event has no start time).
func (m Membership) ReportUntil() time.Time {
	if m.StartsAt.IsZero() {
		return time.Time{}
	}
	return m.StartsAt.Add(ReportFor)
}

// Report is why they can't report someone in their group now, or nil.
func (m Membership) Report(now time.Time) error {
	if !m.confirmedOnce() {
		return ErrNotGoing
	}
	if until := m.ReportUntil(); !until.IsZero() && now.After(until) {
		return ErrTooLate
	}
	return nil
}
