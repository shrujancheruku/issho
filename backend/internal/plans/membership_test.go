package plans

import (
	"errors"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/chat"
)

// The membership rules without a database: each case is one person's
// answer, the group's and the event's states, and a moment.
func TestMembership(t *testing.T) {
	starts := time.Date(2030, 1, 12, 14, 0, 0, 0, time.UTC)
	confirmBy := time.Date(2030, 1, 11, 0, 0, 0, 0, time.UTC)
	mk := func(response, group, event string) Membership {
		return Membership{Response: response, GroupStatus: group, EventStatus: event, StartsAt: starts, ConfirmBy: confirmBy}
	}
	going := mk(Confirmed, GroupConfirmed, EventScheduled)
	before, after := confirmBy.Add(-time.Hour), confirmBy.Add(time.Hour)

	t.Run("answer", func(t *testing.T) {
		for _, c := range []struct {
			name string
			m    Membership
			a    Action
			now  time.Time
			next string
			err  error
		}{
			{"confirm an invitation", mk(Invited, GroupInviting, EventScheduled), Confirm, before, Confirmed, nil},
			{"decline an invitation", mk(Invited, GroupInviting, EventScheduled), Decline, before, Declined, nil},
			{"change a decline", mk(Declined, GroupInviting, EventScheduled), Confirm, before, Confirmed, nil},
			{"decline twice", mk(Declined, GroupInviting, EventScheduled), Decline, before, "", ErrNotNow},
			{"confirm after the deadline", mk(Invited, GroupInviting, EventScheduled), Confirm, confirmBy, "", ErrTooLate},
			{"cancel after the deadline", going, Cancel, after, Dropped, nil},
			{"cancel at the start", going, Cancel, starts, "", ErrTooLate},
			{"cancel an invitation", mk(Invited, GroupInviting, EventScheduled), Cancel, before, "", ErrNotNow},
			{"group cancelled", mk(Invited, GroupCancelled, EventScheduled), Confirm, before, "", ErrEventOver},
			{"event done", mk(Confirmed, GroupConfirmed, EventDone), Cancel, before, "", ErrEventOver},
		} {
			next, err := c.m.Answer(c.a, c.now)
			if next != c.next || !errors.Is(err, c.err) {
				t.Errorf("%s: got %q, %v; want %q, %v", c.name, next, err, c.next, c.err)
			}
		}
		if _, err := going.Answer(Action(9), before); err == nil {
			t.Error("an unknown action was accepted")
		}
	})

	t.Run("leave", func(t *testing.T) {
		for _, c := range []struct {
			name string
			m    Membership
			now  time.Time
			next string
		}{
			{"invited, after the deadline", mk(Invited, GroupInviting, EventScheduled), after, Declined},
			{"going", going, after, Dropped},
			{"already declined", mk(Declined, GroupInviting, EventScheduled), before, ""},
			{"started", going, starts, ""},
			{"group cancelled", mk(Confirmed, GroupCancelled, EventScheduled), before, ""},
		} {
			next, ok := c.m.Leave(c.now)
			if next != c.next || ok != (c.next != "") {
				t.Errorf("%s: got %q, %v; want %q", c.name, next, ok, c.next)
			}
		}
	})

	t.Run("windows", func(t *testing.T) {
		for _, c := range []struct {
			name string
			m    Membership
			now  time.Time
			// What's allowed at now: see the meeting point, check in, write
			// in the chat, give feedback, report.
			reveal                          bool
			checkIn, chat, feedback, report error
		}{
			{"going, a week before", going, starts.AddDate(0, 0, -7), false, ErrCheckInClosed, nil, ErrFeedbackClosed, nil},
			{"going, a day before", going, starts.Add(-RevealBefore), true, ErrCheckInClosed, nil, ErrFeedbackClosed, nil},
			{"going, at check-in", going, starts.Add(-CheckInBefore), true, nil, nil, ErrFeedbackClosed, nil},
			{"going, check-in's last moment", going, starts.Add(CheckInAfter), true, nil, nil, nil, nil},
			{"going, after two weeks", going, starts.Add(FeedbackFor), true, ErrCheckInClosed, nil, ErrFeedbackClosed, nil},
			{"going, after 30 days", going, starts.Add(ReportFor + time.Second), true, ErrCheckInClosed, nil, ErrFeedbackClosed, ErrTooLate},
			{"waiting for the group", mk(Confirmed, GroupInviting, EventScheduled), starts, true, nil, nil, ErrEventOver, nil},
			{"cancelled since", mk(Dropped, GroupConfirmed, EventScheduled), starts, false, ErrNotGoing, ErrNotGoing, ErrNotGoing, nil},
			{"declined", mk(Declined, GroupConfirmed, EventScheduled), starts, false, ErrNotGoing, ErrNotGoing, ErrNotGoing, ErrNotGoing},
			{"group cancelled", mk(Confirmed, GroupCancelled, EventScheduled), starts, false, ErrEventOver, ErrEventOver, ErrEventOver, nil},
			{"event cancelled", mk(Confirmed, GroupConfirmed, EventCancelled), starts, false, ErrEventOver, ErrEventOver, ErrEventOver, nil},
			{"event done", mk(Confirmed, GroupConfirmed, EventDone), starts.Add(FeedbackAfter), true, nil, nil, nil, nil},
		} {
			_, _, _, chatErr := c.m.Chat(c.now)
			if got := c.m.Revealed(c.now); got != c.reveal {
				t.Errorf("%s: revealed %v", c.name, got)
			}
			for _, e := range []struct {
				what      string
				got, want error
			}{
				{"check in", c.m.CheckIn(c.now), c.checkIn},
				{"chat", chatErr, c.chat},
				{"feedback", c.m.Feedback(c.now), c.feedback},
				{"report", c.m.Report(c.now), c.report},
			} {
				if !errors.Is(e.got, e.want) {
					t.Errorf("%s: %s: got %v, want %v", c.name, e.what, e.got, e.want)
				}
			}
		}
	})

	t.Run("chat opens and closes", func(t *testing.T) {
		opens, closes, open, err := going.Chat(starts.Add(-chat.OpensBefore))
		if err != nil || !open || !opens.Equal(starts.Add(-chat.OpensBefore)) || !closes.Equal(starts.Add(chat.ClosesAfter)) {
			t.Errorf("at opening: %v %v %v %v", opens, closes, open, err)
		}
		if _, _, open, _ := going.Chat(starts.Add(chat.ClosesAfter)); open {
			t.Error("open at closing time")
		}
	})

	t.Run("no start time", func(t *testing.T) {
		m := going
		m.StartsAt = time.Time{}
		later := starts.AddDate(1, 0, 0)
		if from, until := m.CheckInWindow(); !from.IsZero() || !until.IsZero() || m.CheckIn(starts) != ErrCheckInClosed {
			t.Error("check-in without a start")
		}
		if _, _, err := m.FeedbackWindow(); err != ErrFeedbackClosed {
			t.Errorf("feedback without a start: %v", err)
		}
		if m.Revealed(later) || !m.RevealAt().IsZero() || !m.ReportUntil().IsZero() || m.Report(later) != nil {
			t.Error("reveal or report without a start")
		}
		if _, _, open, err := m.Chat(starts); open || err != nil {
			t.Errorf("chat without a start: %v %v", open, err)
		}
		if _, err := m.Answer(Cancel, later); err != nil {
			t.Errorf("cancel without a start: %v", err)
		}
	})
}
