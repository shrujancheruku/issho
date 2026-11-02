package crews

import "time"

// Membership is one person's place in a crew: their answer, the crew's
// state, and when the offer closes. It holds every rule about what they can
// do (see the crew, answer an offer, chat, leave, be removed), so the
// service only loads, locks and stores, as with plans.Membership.
type Membership struct {
	Status     string // Offered, Active, Lapsed or Closed
	Response   string // Undecided, Accepted, Declined, Left or Unanswered
	OfferUntil time.Time
}

// Visible is whether the crew is shown to them, and whether they are listed
// among its members: an offer while they're deciding or have accepted, and
// an active crew once they accepted. Declined, left and unanswered members
// aren't part of it, so nobody sits in an active crew's list as "still
// deciding". ListUserCrews and ListCrewMembers say the same in SQL.
func (m Membership) Visible() bool {
	switch m.Status {
	case Offered:
		return m.Response == Undecided || m.Response == Accepted
	case Active:
		return m.Response == Accepted
	}
	return false
}

// Respond is why they can't answer the offer now, or nil. The answer can be
// changed until the offer closes.
func (m Membership) Respond(now time.Time) error {
	if m.Status != Offered || !now.Before(m.OfferUntil) || m.Response == Unanswered {
		return ErrOver
	}
	return nil
}

// Chat is why they can't use the crew's chat, or nil. Reading and writing
// are the same permission: members of an active crew.
func (m Membership) Chat() error {
	if m.Response != Accepted {
		return ErrNotMember
	}
	if m.Status != Active {
		return ErrOver
	}
	return nil
}

// Leave is why they can't leave, or nil: only someone who accepted, from an
// offer or an active crew.
func (m Membership) Leave() error {
	if m.Response != Accepted || (m.Status != Active && m.Status != Offered) {
		return ErrNotMember
	}
	return nil
}

// Remove is the response that takes them out of a crew whatever they want
// (a suspension): an offer they haven't answered is declined, a place they
// accepted is left. ok is false when there's nothing to remove.
func (m Membership) Remove() (next string, ok bool) {
	switch {
	case m.Status == Offered && m.Response == Undecided:
		return Declined, true
	case (m.Status == Offered || m.Status == Active) && m.Response == Accepted:
		return Left, true
	}
	return "", false
}
