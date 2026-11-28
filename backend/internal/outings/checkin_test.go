package outings_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/outings"
	"github.com/shrujancheruku/issho/backend/internal/policy"
)

// The lanterns plan runs 17:00 to 20:30 on the city's clock (UTC+7), so the
// outing of the 9th starts at 10:00 UTC and ends at 13:30 UTC.
var (
	lanternsStart = time.Date(2026, 1, 9, 10, 0, 0, 0, time.UTC)
	lanternsEnd   = time.Date(2026, 1, 9, 13, 30, 0, 0, time.UTC)
)

// going is an outing of n people who have all said yes, so all are going, on
// the clock at the moment the third said it.
func (r *rig) going(t *testing.T, n int) (ids []string, outing string) {
	t.Helper()
	ids, outing = r.formed(t, n)
	for _, id := range ids {
		r.answer(t, id, outing, outings.Yes)
	}
	if got := r.seatOf(t, ids[0]).State; got != outings.SeatGoing {
		t.Fatalf("seat = %q, want going", got)
	}
	return ids, outing
}

func (r *rig) checkIn(t *testing.T, id, outing string) outings.Seat {
	t.Helper()
	seat, err := r.svc.CheckIn(context.Background(), id, outing)
	if err != nil {
		t.Fatalf("%s checks in: %v", id, err)
	}
	return seat
}

// Check-in opens 30 minutes before the start, on the city's clock, and not a
// second sooner.
func TestCheckInOpensThirtyMinutesBeforeTheStart(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 3)
	opens := lanternsStart.Add(-policy.CheckInOpensBeforeStart)

	r.clk.Set(opens.Add(-time.Second))
	_, early := r.svc.CheckIn(context.Background(), ids[0], outing)
	r.clk.Set(opens)
	seat, err := r.svc.CheckIn(context.Background(), ids[0], outing)

	if !errors.Is(early, outings.ErrCheckInClosed) {
		t.Errorf("a second before it opens: err = %v, want ErrCheckInClosed", early)
	}
	if err != nil || !seat.CheckedIn {
		t.Fatalf("at the opening: %+v, %v; want checked in", seat, err)
	}
	if !seat.CheckInOpensAt.Equal(opens) || !seat.CheckInClosesAt.Equal(lanternsEnd) {
		t.Errorf("window = %v to %v, want %v to %v", seat.CheckInOpensAt, seat.CheckInClosesAt, opens, lanternsEnd)
	}
}

// Check-in stays open to the plan's stated end: the last second counts, the end
// does not.
func TestCheckInStaysOpenToThePlansEnd(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 3)

	r.clk.Set(lanternsEnd.Add(-time.Second))
	seat, err := r.svc.CheckIn(context.Background(), ids[0], outing)
	r.clk.Set(lanternsEnd)
	_, late := r.svc.CheckIn(context.Background(), ids[1], outing)

	if err != nil || !seat.CheckedIn {
		t.Fatalf("a second before the end: %+v, %v; want checked in", seat, err)
	}
	if !errors.Is(late, outings.ErrCheckInClosed) {
		t.Errorf("at the end: err = %v, want ErrCheckInClosed", late)
	}
	if r.seatOf(t, ids[1]).CheckedIn {
		t.Error("a refused check-in was recorded")
	}
}

// Only a person going can check in: not one still invited, not one who
// cancelled, and not one with no seat, who finds no outing.
func TestOnlyAPersonGoingCanCheckIn(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 4)
	r.answer(t, ids[0], outing, outings.Yes)
	r.answer(t, ids[1], outing, outings.Yes)
	r.answer(t, ids[2], outing, outings.Yes) // sets it; ids[3] is still invited
	r.answer(t, ids[2], outing, outings.Yes)
	if _, err := r.svc.Cancel(context.Background(), ids[2], outing); err != nil {
		t.Fatal(err)
	}
	outsider := r.member(t, 50, person{})
	r.clk.Set(lanternsStart)

	_, invited := r.svc.CheckIn(context.Background(), ids[3], outing)
	_, cancelled := r.svc.CheckIn(context.Background(), ids[2], outing)
	_, stranger := r.svc.CheckIn(context.Background(), outsider, outing)

	if !errors.Is(invited, outings.ErrNotGoing) {
		t.Errorf("an invited person: err = %v, want ErrNotGoing", invited)
	}
	if !errors.Is(cancelled, outings.ErrNotGoing) {
		t.Errorf("a cancelled seat: err = %v, want ErrNotGoing", cancelled)
	}
	if !errors.Is(stranger, outings.ErrNotFound) {
		t.Errorf("no seat: err = %v, want ErrNotFound", stranger)
	}
}

// Checking in twice changes nothing, including the time it was done.
func TestCheckingInAgainChangesNothing(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 3)
	r.clk.Set(lanternsStart)
	first := r.checkIn(t, ids[0], outing)
	r.clk.Advance(time.Hour)

	again := r.checkIn(t, ids[0], outing)

	if !again.CheckedIn || !again.CheckedInAt.Equal(first.CheckedInAt) || !first.CheckedInAt.Equal(lanternsStart) {
		t.Errorf("checked in at %v, then %v; want %v both times", first.CheckedInAt, again.CheckedInAt, lanternsStart)
	}
}

// People going see who has checked in, by first name: the others, not
// themselves, and nobody who hasn't.
func TestPeopleGoingSeeWhoHasCheckedIn(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 3)
	r.clk.Set(lanternsStart)
	r.checkIn(t, ids[1], outing)

	first := r.seatOf(t, ids[0])
	second := r.seatOf(t, ids[1])
	r.checkIn(t, ids[2], outing)
	first = r.seatOf(t, ids[0])

	if got := first.CheckedInFirstNames; len(got) != 2 || got[0] != "Person" || got[1] != "Person" {
		t.Errorf("the first sees %v checked in, want the other two by first name", got)
	}
	if len(second.CheckedInFirstNames) != 0 {
		t.Errorf("the second sees %v checked in, want none (only themselves)", second.CheckedInFirstNames)
	}
	if first.CheckedIn {
		t.Error("the first is shown as checked in; they haven't")
	}
}

// Once checked in there is no cancel (docs/spec/outings.md, "Going").
func TestOnceCheckedInThereIsNoCancel(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 3)
	r.clk.Set(lanternsStart)
	r.checkIn(t, ids[0], outing)

	_, err := r.svc.Cancel(context.Background(), ids[0], outing)

	if !errors.Is(err, outings.ErrCheckedIn) {
		t.Fatalf("err = %v, want ErrCheckedIn", err)
	}
	if got := r.seatOf(t, ids[0]); got.State != outings.SeatGoing || !got.CheckedIn {
		t.Errorf("seat = %q checked in %v; want still going and checked in", got.State, got.CheckedIn)
	}
}
