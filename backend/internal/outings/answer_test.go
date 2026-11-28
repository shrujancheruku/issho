package outings_test

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertest"

	"github.com/shrujancheruku/issho/backend/internal/jobs"
	"github.com/shrujancheruku/issho/backend/internal/outings"
	"github.com/shrujancheruku/issho/backend/internal/people"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

// formed forms the lanterns round from n people who all ticked the evening of
// the 9th, and returns them with the one outing they share.
func (r *rig) formed(t *testing.T, n int) (ids []string, outing string) {
	t.Helper()
	ids = r.crowd(t, 1, n, person{})
	if formed, err := r.svc.Form(context.Background(), lanterns); err != nil || formed != 1 {
		t.Fatalf("Form = %d, %v; want 1 outing", formed, err)
	}
	return ids, r.seatsOf(t, ids[0])[0].OutingID
}

// seatOf is the person's one seat.
func (r *rig) seatOf(t *testing.T, id string) outings.Seat {
	t.Helper()
	seats := r.seatsOf(t, id)
	if len(seats) != 1 {
		t.Fatalf("%s has %d seats, want 1", id, len(seats))
	}
	return seats[0]
}

func (r *rig) answer(t *testing.T, id, outing string, a outings.Answer) outings.Seat {
	t.Helper()
	seat, err := r.svc.Answer(context.Background(), id, outing, a)
	if err != nil {
		t.Fatalf("%s answers %s: %v", id, a, err)
	}
	return seat
}

// An invited person says yes or no for 48 hours counted from the invitation.
// A yes is held while the outing isn't set; a no is a declined seat.
func TestAnInvitedPersonCanSayYesOrNoInsideTheFortyEightHours(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 4)

	r.clk.Advance(policyWindow - time.Second) // the last second of the 48 hours
	yes := r.answer(t, ids[0], outing, outings.Yes)
	no := r.answer(t, ids[1], outing, outings.No)

	if yes.State != outings.SeatAccepted {
		t.Errorf("after yes the seat is %q, want %q", yes.State, outings.SeatAccepted)
	}
	if no.State != outings.SeatDeclined {
		t.Errorf("after no the seat is %q, want %q", no.State, outings.SeatDeclined)
	}
	if got := r.seatOf(t, ids[0]).State; got != outings.SeatAccepted {
		t.Errorf("the stored seat is %q, want it kept", got)
	}
}

// At the 48 hours the invitation has closed: an answer is refused and nothing
// is written.
func TestAnAnswerAtTheFortyEightHoursIsRefused(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 4)
	r.clk.Advance(policyWindow)

	_, err := r.svc.Answer(context.Background(), ids[0], outing, outings.Yes)

	if !errors.Is(err, outings.ErrInvitationClosed) {
		t.Fatalf("err = %v, want ErrInvitationClosed", err)
	}
	if got := r.seatOf(t, ids[0]).State; got != outings.SeatInvited {
		t.Errorf("the seat is %q, want it left invited", got)
	}
}

// A no can be changed to a yes inside the 48 hours.
func TestANoCanBeChangedToAYesInsideTheFortyEightHours(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 4)
	r.answer(t, ids[0], outing, outings.No)
	r.clk.Advance(policyWindow - time.Minute)

	seat := r.answer(t, ids[0], outing, outings.Yes)

	if seat.State != outings.SeatAccepted {
		t.Fatalf("seat = %q, want accepted", seat.State)
	}
}

// ...and not after them.
func TestANoCannotBeChangedToAYesAfterTheFortyEightHours(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 4)
	r.answer(t, ids[0], outing, outings.No)
	r.clk.Advance(policyWindow)

	_, err := r.svc.Answer(context.Background(), ids[0], outing, outings.Yes)

	if !errors.Is(err, outings.ErrInvitationClosed) {
		t.Fatalf("err = %v, want ErrInvitationClosed", err)
	}
}

// Someone with no seat in the outing is told there is no such outing, as for
// one that doesn't exist: they can't learn which outings exist.
func TestAnAnswerFromSomeoneWithNoSeatFindsNoOuting(t *testing.T) {
	r := newRig(t)
	_, outing := r.formed(t, 3)
	outsider := r.member(t, 9, person{})

	_, err := r.svc.Answer(context.Background(), outsider, outing, outings.Yes)
	_, missing := r.svc.Answer(context.Background(), outsider, "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0abc", outings.Yes)

	if !errors.Is(err, outings.ErrNotFound) || !errors.Is(missing, outings.ErrNotFound) {
		t.Fatalf("errs = %v, %v; want ErrNotFound for both", err, missing)
	}
}

// Anything but yes or no is refused.
func TestAnAnswerThatIsNeitherYesNorNoIsRefused(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 3)

	_, err := r.svc.Answer(context.Background(), ids[0], outing, outings.Answer("maybe"))

	if !errors.Is(err, outings.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

// Two yeses do not set the outing: they wait.
func TestTwoYesesDoNotSetTheOuting(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 4)

	r.answer(t, ids[0], outing, outings.Yes)
	seat := r.answer(t, ids[1], outing, outings.Yes)

	if seat.OutingState != outings.OutingOffered || seat.State != outings.SeatAccepted {
		t.Fatalf("after two yeses: outing %q, seat %q; want offered and accepted", seat.OutingState, seat.State)
	}
}

// The outing is set as soon as 3 say yes, and everyone who said yes is going.
// Someone who has not answered stays invited, and one who said no stays out.
func TestTheThirdYesSetsTheOutingAndThoseWhoSaidYesAreGoing(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 5)
	r.answer(t, ids[0], outing, outings.Yes)
	r.answer(t, ids[1], outing, outings.No)
	r.answer(t, ids[2], outing, outings.Yes)

	third := r.answer(t, ids[3], outing, outings.Yes)

	if third.OutingState != outings.OutingSet || third.State != outings.SeatGoing {
		t.Fatalf("third yes: outing %q, seat %q; want set and going", third.OutingState, third.State)
	}
	want := []outings.SeatState{outings.SeatGoing, outings.SeatDeclined, outings.SeatGoing, outings.SeatGoing, outings.SeatInvited}
	for i, id := range ids {
		seat := r.seatOf(t, id)
		if seat.State != want[i] || seat.OutingState != outings.OutingSet {
			t.Errorf("person %d: seat %q in an outing %q; want %q in a set one", i, seat.State, seat.OutingState, want[i])
		}
	}
}

// With the outing set, a later yes inside the 48 hours is going at once, and a
// person who said no may still change to yes.
func TestAYesAfterTheOutingIsSetIsGoingAtOnce(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 5)
	r.answer(t, ids[4], outing, outings.No)
	for _, id := range ids[:3] {
		r.answer(t, id, outing, outings.Yes)
	}

	fourth := r.answer(t, ids[3], outing, outings.Yes)
	changed := r.answer(t, ids[4], outing, outings.Yes)

	if fourth.State != outings.SeatGoing || changed.State != outings.SeatGoing {
		t.Fatalf("seats = %q and %q, want both going", fourth.State, changed.State)
	}
}

// Three people answering at the same moment set the outing once, and all of
// them are going: no yes is lost between them.
func TestThreeYesesAtTheSameTimeSetTheOutingOnce(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 4)

	// The answers queue on the outing's row, which the test holds, and are let go
	// together: they overlap for certain, and only the lock sets it once.
	release := r.holdOutings(t, outing)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, id := range ids[:3] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := r.svc.Answer(context.Background(), id, outing, outings.Yes); err != nil {
				t.Error(err)
			}
		}()
	}
	close(start)
	testdb.WaitForLockWaiters(t, r.pool, "LockOuting", 3)
	release()
	wg.Wait()

	for _, id := range ids[:3] {
		if got := r.seatOf(t, id); got.State != outings.SeatGoing || got.OutingState != outings.OutingSet {
			t.Errorf("seat %q in an outing %q, want going in a set one", got.State, got.OutingState)
		}
	}
}

// A yes already given is answered again without harm, and a going person's
// "no" is a cancel, which has its own call.
func TestAnsweringYesTwiceChangesNothingAndANoFromAGoingPersonIsRefused(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 3)
	for _, id := range ids {
		r.answer(t, id, outing, outings.Yes)
	}

	again := r.answer(t, ids[0], outing, outings.Yes)
	_, err := r.svc.Answer(context.Background(), ids[0], outing, outings.No)

	if again.State != outings.SeatGoing {
		t.Errorf("a second yes leaves %q, want going", again.State)
	}
	if !errors.Is(err, outings.ErrInvitationClosed) {
		t.Errorf("a no from a going person = %v, want ErrInvitationClosed (they cancel instead)", err)
	}
}

// work runs the expiring job for the outing the way the worker process would.
func (r *rig) expire(t *testing.T, args outings.ExpireArgs) error {
	t.Helper()
	ctx := context.Background()
	w := rivertest.NewWorker(t, riverpgxv5.New(r.pool), &river.Config{Schema: jobs.Schema}, outings.NewExpireWorker(r.svc))
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = w.Work(ctx, t, tx, args, nil)
	return err
}

// riverDriver is River's driver on the rig's pool, to look at the jobs the
// service has made.
func (r *rig) riverDriver() *riverpgxv5.Driver { return riverpgxv5.New(r.pool) }

// An invitation's unanswered seat is recorded by a job made with the
// invitation: it is scheduled for the end of the 48 hours, made when the group
// is formed, and when it runs the seat that nobody answered is unanswered. The
// outing with fewer than 3 yeses is not set, and a yes stays a yes.
func TestAnUnansweredSeatIsRecordedAtFortyEightHoursByAJobMadeWithTheInvitation(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	ids, outing := r.formed(t, 5)
	r.answer(t, ids[0], outing, outings.Yes)
	r.answer(t, ids[1], outing, outings.Yes)
	r.answer(t, ids[2], outing, outings.No)

	job := rivertest.RequireInserted(ctx, t, r.riverDriver(), outings.ExpireArgs{OutingID: outing}, &rivertest.RequireInsertedOpts{Schema: jobs.Schema})
	if want := rigStart.Add(policyWindow); !job.ScheduledAt.Equal(want) {
		t.Errorf("the job is scheduled for %v, want %v (48 hours after the invitations)", job.ScheduledAt, want)
	}
	r.clk.Advance(policyWindow)
	if err := r.expire(t, outings.ExpireArgs{OutingID: outing}); err != nil {
		t.Fatal(err)
	}

	for i, want := range []outings.SeatState{outings.SeatAccepted, outings.SeatAccepted, outings.SeatDeclined, outings.SeatUnanswered, outings.SeatUnanswered} {
		seat := r.seatOf(t, ids[i])
		if seat.State != want || seat.OutingState != outings.OutingNotSet {
			t.Errorf("person %d: seat %q in an outing %q; want %q in one not set", i, seat.State, seat.OutingState, want)
		}
	}
}

// A job that runs before the 48 hours are over, or twice, closes nothing it
// shouldn't: invited people can still answer, and an answered seat is not
// rewritten.
func TestTheExpiringJobClosesOnlyWhatIsDue(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 4)
	r.clk.Advance(policyWindow - time.Second)
	if err := r.expire(t, outings.ExpireArgs{OutingID: outing}); err != nil {
		t.Fatal(err)
	}
	if got := r.seatOf(t, ids[0]); got.State != outings.SeatInvited || got.OutingState != outings.OutingOffered {
		t.Fatalf("a second before the 48 hours: seat %q in an outing %q; want it still invited and offered", got.State, got.OutingState)
	}
	r.answer(t, ids[0], outing, outings.Yes)

	r.clk.Advance(time.Second)
	for range 2 { // River runs a job at least once
		if err := r.expire(t, outings.ExpireArgs{OutingID: outing}); err != nil {
			t.Fatal(err)
		}
	}

	if got := r.seatOf(t, ids[0]).State; got != outings.SeatAccepted {
		t.Errorf("the yes is now %q, want it kept", got)
	}
	if got := r.seatOf(t, ids[1]).State; got != outings.SeatUnanswered {
		t.Errorf("an unanswered seat is %q, want unanswered", got)
	}
}

// An outing already set stays set when the rest of its invitations run out; the
// people who never answered are unanswered.
func TestAnOutingSetStaysSetWhenTheRemainingInvitationsRunOut(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 4)
	for _, id := range ids[:3] {
		r.answer(t, id, outing, outings.Yes)
	}
	r.clk.Advance(policyWindow)

	if err := r.expire(t, outings.ExpireArgs{OutingID: outing}); err != nil {
		t.Fatal(err)
	}

	if got := r.seatOf(t, ids[3]); got.State != outings.SeatUnanswered || got.OutingState != outings.OutingSet {
		t.Fatalf("seat %q in an outing %q, want unanswered in a set one", got.State, got.OutingState)
	}
	if got := r.seatOf(t, ids[0]).State; got != outings.SeatGoing {
		t.Fatalf("a yes is %q, want going", got)
	}
}

// An outing that is not set takes no more answers, even from people whose
// invitation hadn't been closed by the job yet.
func TestAnOutingThatWasNotSetTakesNoAnswers(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 3)
	r.clk.Advance(policyWindow)
	if err := r.expire(t, outings.ExpireArgs{OutingID: outing}); err != nil {
		t.Fatal(err)
	}

	_, err := r.svc.Answer(context.Background(), ids[0], outing, outings.Yes)

	if !errors.Is(err, outings.ErrInvitationClosed) {
		t.Fatalf("err = %v, want ErrInvitationClosed", err)
	}
}

// An invited person sees how many have said yes and never who. Someone going
// sees the first names of the others going, not their own and not the
// surnames; someone who has not said yes sees no name.
func TestAnInvitedPersonSeesTheCountOfYesesAndAPersonGoingSeesTheFirstNames(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 5)
	for i, name := range []string{"Ada Lovelace", "Grace Hopper", "Katherine Johnson", "Mary Jackson", "Dorothy Vaughan"} {
		in := people.ProfileInput{
			Name: name, Gender: people.GenderMan, Languages: []string{"en", "vi"}, LivesInCity: true, AcceptedCodeOfConduct: true,
		}
		if _, err := r.people.SaveProfile(context.Background(), ids[i], in); err != nil {
			t.Fatal(err)
		}
	}
	r.answer(t, ids[0], outing, outings.Yes)
	r.answer(t, ids[1], outing, outings.Yes)

	waiting := r.seatOf(t, ids[0])
	invited := r.seatOf(t, ids[4])
	if waiting.YesCount != 2 || invited.YesCount != 2 || len(waiting.GoingFirstNames) != 0 || len(invited.GoingFirstNames) != 0 {
		t.Fatalf("before the outing is set: yeses %d and %d, names %v and %v; want 2 yeses and no names",
			waiting.YesCount, invited.YesCount, waiting.GoingFirstNames, invited.GoingFirstNames)
	}

	r.answer(t, ids[2], outing, outings.Yes)

	ada := r.seatOf(t, ids[0])
	if !reflect.DeepEqual(ada.GoingFirstNames, []string{"Grace", "Katherine"}) || ada.YesCount != 3 {
		t.Errorf("going: names %v, %d yeses; want Grace and Katherine, 3", ada.GoingFirstNames, ada.YesCount)
	}
	invited = r.seatOf(t, ids[4])
	if len(invited.GoingFirstNames) != 0 || invited.YesCount != 3 {
		t.Errorf("still invited: names %v, %d yeses; want no names and 3", invited.GoingFirstNames, invited.YesCount)
	}
}

// A person going can cancel, and the seat records it; they stop being one of
// those going, to themselves and to the others.
func TestAPersonGoingCanCancelAndTheSeatRecordsIt(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 4)
	for _, id := range ids[:3] {
		r.answer(t, id, outing, outings.Yes)
	}

	cancelled, err := r.svc.Cancel(context.Background(), ids[0], outing)

	if err != nil {
		t.Fatal(err)
	}
	if cancelled.State != outings.SeatCancelled || r.seatOf(t, ids[0]).State != outings.SeatCancelled {
		t.Fatalf("seat = %q, want cancelled (and stored so)", cancelled.State)
	}
	if got := r.seatOf(t, ids[1]); got.YesCount != 2 {
		t.Errorf("the others count %d yeses, want 2 once one cancelled", got.YesCount)
	}
}

// Only a person going can cancel; an invited, declined or cancelled one can't,
// and someone with no seat finds no outing.
func TestOnlyAPersonGoingCanCancel(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 4)
	r.answer(t, ids[1], outing, outings.No)
	outsider := r.member(t, 9, person{})

	_, invited := r.svc.Cancel(context.Background(), ids[0], outing)
	_, declined := r.svc.Cancel(context.Background(), ids[1], outing)
	_, stranger := r.svc.Cancel(context.Background(), outsider, outing)

	if !errors.Is(invited, outings.ErrNotGoing) || !errors.Is(declined, outings.ErrNotGoing) {
		t.Errorf("errs = %v, %v; want ErrNotGoing for an invited and a declined seat", invited, declined)
	}
	if !errors.Is(stranger, outings.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound for someone with no seat", stranger)
	}
}

// A third plan that also runs in the evening, so a person can tick the same
// time on two plans.
const lateLanterns = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0003"

const lateLanternsFixture = `{"plans": [{
  "id": "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0003", "city": "da-nang-hoi-an", "price": 90000,
  "meeting_address": "An Hoi, Hoi An", "parts_of_day": ["morning", "evening"], "starts_at": "18:30", "ends_at": "21:00",
  "opens_on": "2026-01-05",
  "texts": {"en": {"title": "Late lanterns", "description": "A late walk.", "meeting_point": "The bridge.", "what_to_bring": []}},
  "stops": []
}]}`

// An outing a person is going to takes that time off their other signups: the
// third yes makes three people going, and each of them is withdrawn from the
// same part of the same day on another plan, but keeps the rest.
func TestAnOutingAPersonIsGoingToRemovesThatTimeFromTheirOtherSignups(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	if err := plans.LoadFixture(ctx, r.pool, []byte(lateLanternsFixture)); err != nil {
		t.Fatal(err)
	}
	ids, outing := r.formed(t, 4)
	morning9 := plans.Tick{Date: "2026-01-09", Part: "morning"}
	// Everyone but the fourth also ticked the same evening on the other plan,
	// and the first also a morning, which doesn't clash with it.
	r.signUp(t, ids[0], lateLanterns, evening9th, morning9)
	r.signUp(t, ids[1], lateLanterns, evening9th)
	r.signUp(t, ids[3], lateLanterns, evening9th)
	r.answer(t, ids[0], outing, outings.Yes)
	r.answer(t, ids[1], outing, outings.Yes)
	if got := r.ticksOn(t, ids[0], lateLanterns); len(got) != 2 {
		t.Fatalf("two yeses haven't set the outing, yet the first's other ticks are %v", got)
	}

	r.answer(t, ids[2], outing, outings.Yes)

	if got := r.ticksOn(t, ids[0], lateLanterns); !reflect.DeepEqual(got, []plans.Tick{morning9}) {
		t.Errorf("the first person's other plan has %v, want only the morning", got)
	}
	if got := r.ticksOn(t, ids[1], lateLanterns); got != nil {
		t.Errorf("the second person's other plan has %v, want no signup left", got)
	}
	if got := r.ticksOn(t, ids[3], lateLanterns); !reflect.DeepEqual(got, []plans.Tick{evening9th}) {
		t.Errorf("the fourth person, still invited, keeps %v on the other plan", got)
	}
	if got := r.ticksOn(t, ids[0], lanterns); !reflect.DeepEqual(got, []plans.Tick{evening9th}) {
		t.Errorf("the outing's own plan has %v, want it kept", got)
	}

	// The fourth says yes once it is set: going at once, and the time goes too.
	r.answer(t, ids[3], outing, outings.Yes)
	if got := r.ticksOn(t, ids[3], lateLanterns); got != nil {
		t.Errorf("a later yes leaves %v on the other plan, want no signup", got)
	}
}

// ticksOn is the person's ticks on the plan, nil with no signup to it.
func (r *rig) ticksOn(t *testing.T, id, plan string) []plans.Tick {
	t.Helper()
	mine, err := r.signups.Mine(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range mine {
		if s.PlanID == plan {
			return s.Ticks
		}
	}
	return nil
}

// A yes that only waits, or a no, takes nothing from the person's other
// signups: they are going to nothing yet.
func TestAYesThatIsStillWaitingKeepsTheOtherSignups(t *testing.T) {
	r := newRig(t)
	ctx := context.Background()
	if err := plans.LoadFixture(ctx, r.pool, []byte(lateLanternsFixture)); err != nil {
		t.Fatal(err)
	}
	ids, outing := r.formed(t, 4)
	r.signUp(t, ids[0], lateLanterns, evening9th)
	r.signUp(t, ids[1], lateLanterns, evening9th)

	r.answer(t, ids[0], outing, outings.Yes)
	r.answer(t, ids[1], outing, outings.No)

	for _, id := range ids[:2] {
		if got := r.ticksOn(t, id, lateLanterns); !reflect.DeepEqual(got, []plans.Tick{evening9th}) {
			t.Errorf("%s has %v on the other plan, want it kept", id, got)
		}
	}
}

// twoOutings forms two outings on the same evening, one on each plan, that share
// the first person: n people on the lanterns plan and n on the late lanterns
// plan, the first of the lanterns' also ticking the late one. Forming runs per
// plan, so nothing yet stops the shared person holding an invitation to each.
func (r *rig) twoOutings(t *testing.T, n int) (shared string, a []string, outingA string, b []string, outingB string) {
	t.Helper()
	ctx := context.Background()
	if err := plans.LoadFixture(ctx, r.pool, []byte(lateLanternsFixture)); err != nil {
		t.Fatal(err)
	}
	a = r.crowd(t, 1, n, person{})
	shared = a[0]
	b = []string{shared}
	r.signUp(t, shared, lateLanterns, evening9th)
	for i := n + 1; i < 2*n; i++ {
		id := r.member(t, i, person{})
		r.signUp(t, id, lateLanterns, evening9th)
		b = append(b, id)
	}
	for _, plan := range []string{lanterns, lateLanterns} {
		if formed, err := r.svc.Form(ctx, plan); err != nil || formed != 1 {
			t.Fatalf("Form(%s) = %d, %v; want 1 outing", plan, formed, err)
		}
	}
	for _, seat := range r.seatsOf(t, shared) {
		if seat.PlanID == lanterns {
			outingA = seat.OutingID
		} else {
			outingB = seat.OutingID
		}
	}
	if outingA == "" || outingB == "" {
		t.Fatalf("%s has no seat in each outing: %v", shared, r.seatsOf(t, shared))
	}
	return shared, a, outingA, b, outingB
}

// seatIn is the person's seat in the one outing.
func (r *rig) seatIn(t *testing.T, id, outing string) outings.Seat {
	t.Helper()
	for _, seat := range r.seatsOf(t, id) {
		if seat.OutingID == outing {
			return seat
		}
	}
	t.Fatalf("%s has no seat in %s", id, outing)
	return outings.Seat{}
}

// Nobody is double-booked (docs/spec/matching.md): a person who said yes to two
// outings at one time is going to the one that is set first, and their other
// seat at that time becomes a no, which the other outing's count follows.
func TestAPersonGoingToOneOutingIsNeverGoingToAnotherAtTheSameTime(t *testing.T) {
	r := newRig(t)
	shared, a, outingA, b, outingB := r.twoOutings(t, 4)
	r.answer(t, shared, outingA, outings.Yes)
	r.answer(t, shared, outingB, outings.Yes)
	r.answer(t, b[1], outingB, outings.Yes)
	if got := r.seatIn(t, b[1], outingB).YesCount; got != 2 {
		t.Fatalf("before either is set the second outing has %d yeses, want 2", got)
	}

	r.answer(t, a[1], outingA, outings.Yes)
	r.answer(t, a[2], outingA, outings.Yes) // the third yes: the first outing is set

	if got := r.seatIn(t, shared, outingA).State; got != outings.SeatGoing {
		t.Errorf("the shared person's seat in the set outing is %q, want going", got)
	}
	if got := r.seatIn(t, shared, outingB).State; got != outings.SeatDeclined {
		t.Errorf("their seat at the same time in the other outing is %q, want declined", got)
	}
	if got := r.seatIn(t, b[1], outingB); got.YesCount != 1 || got.OutingState != outings.OutingOffered {
		t.Errorf("the other outing has %d yeses and is %q, want 1 and offered", got.YesCount, got.OutingState)
	}

	// The other outing can still be set, without them.
	r.answer(t, b[2], outingB, outings.Yes)
	r.answer(t, b[3], outingB, outings.Yes)
	for _, id := range b[1:] {
		if got := r.seatIn(t, id, outingB); got.State != outings.SeatGoing || got.OutingState != outings.OutingSet {
			t.Errorf("seat %q in an outing %q, want going in a set one", got.State, got.OutingState)
		}
	}
	if got := r.seatIn(t, shared, outingB).State; got != outings.SeatDeclined {
		t.Errorf("setting the other outing leaves the shared person %q there, want declined", got)
	}
}

// A yes to an outing at a date and part of the day where the person is already
// going to another is refused, and leaves the seat as it was.
func TestAYesToAnOutingAtATimeAlreadyTakenIsRefused(t *testing.T) {
	r := newRig(t)
	shared, a, outingA, _, outingB := r.twoOutings(t, 4)
	for _, id := range a[:3] {
		r.answer(t, id, outingA, outings.Yes)
	}

	_, err := r.svc.Answer(context.Background(), shared, outingB, outings.Yes)

	if !errors.Is(err, outings.ErrTimeTaken) {
		t.Fatalf("a yes to a second outing at the same time = %v, want ErrTimeTaken", err)
	}
	if got := r.seatIn(t, shared, outingA).State; got != outings.SeatGoing {
		t.Errorf("the seat they are going to is %q, want it untouched", got)
	}
	if got := r.seatIn(t, shared, outingB).State; got == outings.SeatGoing || got == outings.SeatAccepted {
		t.Errorf("the refused yes left the seat %q", got)
	}
}

// Two third yeses at once, on two outings that share a person, set one of them:
// neither waits on the other forever, and the shared person is going once.
func TestTwoThirdYesesAtOnceOnOutingsSharingAPersonSetOneAndDoNotDeadlock(t *testing.T) {
	r := newRig(t)
	shared, a, outingA, b, outingB := r.twoOutings(t, 3)
	r.answer(t, shared, outingA, outings.Yes)
	r.answer(t, shared, outingB, outings.Yes)
	r.answer(t, a[1], outingA, outings.Yes)
	r.answer(t, b[1], outingB, outings.Yes)

	// Both answers queue on their outings' rows, which the test holds, and are
	// let go together, so they overlap for certain.
	release := r.holdOutings(t, outingA, outingB)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, c := range []struct{ id, outing string }{{a[2], outingA}, {b[2], outingB}} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := r.svc.Answer(context.Background(), c.id, c.outing, outings.Yes); err != nil {
				t.Errorf("%s answers: %v", c.id, err)
			}
		}()
	}
	close(start)
	testdb.WaitForLockWaiters(t, r.pool, "LockOuting", 2)
	release()
	wg.Wait()

	going := 0
	for _, seat := range r.seatsOf(t, shared) {
		if seat.State == outings.SeatGoing {
			going++
		}
	}
	if going != 1 {
		t.Errorf("the shared person is going to %d outings, want 1", going)
	}
	// Exactly one outing is set. The other is still offered, with the yeses of
	// its two others, and the shared person's seat there is declined.
	set, other, otherPeople := outingA, outingB, b
	if r.seatIn(t, b[1], outingB).OutingState == outings.OutingSet {
		set, other, otherPeople = outingB, outingA, a
	}
	if got := r.seatIn(t, shared, set); got.State != outings.SeatGoing || got.OutingState != outings.OutingSet {
		t.Errorf("in the set outing the shared person is %q and it is %q, want going in a set one", got.State, got.OutingState)
	}
	if got := r.seatIn(t, otherPeople[1], other); got.OutingState != outings.OutingOffered || got.YesCount != 2 {
		t.Errorf("the other outing is %q with %d yeses, want offered with 2", got.OutingState, got.YesCount)
	}
	if got := r.seatIn(t, shared, other).State; got != outings.SeatDeclined {
		t.Errorf("the shared person's seat in the other outing is %q, want declined", got)
	}
}

// holdOutings locks the outings' rows in a transaction of the test's own, so
// answers to them queue; the returned func lets them go.
func (r *rig) holdOutings(t *testing.T, ids ...string) (release func()) {
	t.Helper()
	ctx := context.Background()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	if _, err := tx.Exec(ctx, `select 1 from app.outings where id = any($1::uuid[]) order by id for update`, ids); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
}
