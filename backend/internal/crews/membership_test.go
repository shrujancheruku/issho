package crews

import (
	"errors"
	"testing"
	"time"
)

var (
	statuses  = []string{Offered, Active, Lapsed, Closed}
	responses = []string{Undecided, Accepted, Declined, Left, Unanswered}
)

// eachMembership calls f for every status and response.
func eachMembership(f func(m Membership)) {
	for _, s := range statuses {
		for _, r := range responses {
			f(Membership{Status: s, Response: r, OfferUntil: day(10)})
		}
	}
}

func TestMembershipVisible(t *testing.T) {
	want := map[[2]string]bool{
		{Offered, Undecided}: true,
		{Offered, Accepted}:  true,
		{Active, Accepted}:   true,
	}
	eachMembership(func(m Membership) {
		if got := m.Visible(); got != want[[2]string{m.Status, m.Response}] {
			t.Errorf("%s crew, %s: Visible = %v", m.Status, m.Response, got)
		}
	})
}

func TestMembershipRespond(t *testing.T) {
	before, after := day(9), day(10)
	eachMembership(func(m Membership) {
		open := m.Status == Offered && m.Response != Unanswered
		if err := m.Respond(before); (err == nil) != open || (err != nil && !errors.Is(err, ErrOver)) {
			t.Errorf("%s crew, %s, before the deadline: %v", m.Status, m.Response, err)
		}
		if err := m.Respond(after); !errors.Is(err, ErrOver) {
			t.Errorf("%s crew, %s, at the deadline: %v, want ErrOver", m.Status, m.Response, err)
		}
	})
	// A changed mind is fine until then.
	for _, r := range []string{Undecided, Accepted, Declined} {
		if err := (Membership{Status: Offered, Response: r, OfferUntil: day(10)}).Respond(before.Add(time.Hour)); err != nil {
			t.Errorf("%s changing their answer: %v", r, err)
		}
	}
}

func TestMembershipChat(t *testing.T) {
	eachMembership(func(m Membership) {
		var want error
		switch {
		case m.Response != Accepted:
			want = ErrNotMember
		case m.Status != Active:
			want = ErrOver
		}
		if got := m.Chat(); !errors.Is(got, want) || (want == nil) != (got == nil) {
			t.Errorf("%s crew, %s: Chat = %v, want %v", m.Status, m.Response, got, want)
		}
	})
}

func TestMembershipLeaveAndRemove(t *testing.T) {
	eachMembership(func(m Membership) {
		accepted := m.Response == Accepted && (m.Status == Offered || m.Status == Active)
		if err := m.Leave(); (err == nil) != accepted || (err != nil && !errors.Is(err, ErrNotMember)) {
			t.Errorf("%s crew, %s: Leave = %v", m.Status, m.Response, err)
		}
		next, ok := m.Remove()
		switch {
		case accepted:
			if !ok || next != Left {
				t.Errorf("%s crew, %s: Remove = %q, %v; want left", m.Status, m.Response, next, ok)
			}
		case m.Status == Offered && m.Response == Undecided:
			if !ok || next != Declined {
				t.Errorf("%s crew, %s: Remove = %q, %v; want declined", m.Status, m.Response, next, ok)
			}
		default:
			if ok {
				t.Errorf("%s crew, %s: Remove = %q, nothing to remove", m.Status, m.Response, next)
			}
		}
	})
}
