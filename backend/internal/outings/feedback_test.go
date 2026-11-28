package outings_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/outings"
	"github.com/shrujancheruku/issho/backend/internal/policy"
)

// handleOf is the handle the others of the outing know the account's seat by.
// The API never shows it to its owner, so the test reads it from the table.
func (r *rig) handleOf(t *testing.T, id, outing string) string {
	t.Helper()
	var h string
	if err := r.pool.QueryRow(context.Background(), `select handle::text from app.seats where outing_id = $1 and account_id = $2`, outing, id).Scan(&h); err != nil {
		t.Fatal(err)
	}
	return h
}

func (r *rig) feedback(t *testing.T, id, outing string) outings.Feedback {
	t.Helper()
	f, err := r.svc.Feedback(context.Background(), id, outing)
	if err != nil {
		t.Fatalf("%s reads the feedback form: %v", id, err)
	}
	return f
}

func (r *rig) saveFeedback(t *testing.T, id, outing string, in outings.FeedbackInput) outings.Feedback {
	t.Helper()
	f, err := r.svc.SaveFeedback(context.Background(), id, outing, in)
	if err != nil {
		t.Fatalf("%s saves feedback: %v", id, err)
	}
	return f
}

// Feedback opens at the plan's stated end and closes 7 days later: the end
// itself is open, a second before it isn't, the last second of the 7 days is
// open and the moment they are over isn't.
func TestFeedbackIsOpenFromThePlansEndForSevenDays(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 3)
	ctx := context.Background()
	closes := lanternsEnd.Add(policy.FeedbackWindow)

	r.clk.Set(lanternsEnd.Add(-time.Second))
	_, early := r.svc.Feedback(ctx, ids[0], outing)
	_, earlySave := r.svc.SaveFeedback(ctx, ids[0], outing, outings.FeedbackInput{Went: true})
	r.clk.Set(lanternsEnd)
	form, atEnd := r.svc.Feedback(ctx, ids[0], outing)
	r.clk.Set(closes.Add(-time.Second))
	_, lastSecond := r.svc.SaveFeedback(ctx, ids[0], outing, outings.FeedbackInput{Went: true})
	r.clk.Set(closes)
	_, late := r.svc.Feedback(ctx, ids[0], outing)
	_, lateSave := r.svc.SaveFeedback(ctx, ids[0], outing, outings.FeedbackInput{Went: false})

	if !errors.Is(early, outings.ErrFeedbackNotOpen) || !errors.Is(earlySave, outings.ErrFeedbackNotOpen) {
		t.Errorf("before the end: %v, %v; want ErrFeedbackNotOpen", early, earlySave)
	}
	if atEnd != nil || !form.ClosesAt.Equal(closes) {
		t.Errorf("at the end: %v, closes at %v; want open, closing at %v", atEnd, form.ClosesAt, closes)
	}
	if lastSecond != nil {
		t.Errorf("the last second: %v, want it open", lastSecond)
	}
	if !errors.Is(late, outings.ErrFeedbackClosed) || !errors.Is(lateSave, outings.ErrFeedbackClosed) {
		t.Errorf("when it closes: %v, %v; want ErrFeedbackClosed", late, lateSave)
	}
	if got := r.feedbackWent(t, ids[0], outing); got != "yes" {
		t.Errorf("a refused answer changed the stored one to %q", got)
	}
}

// feedbackWent is "yes", "no" or "none" for the account's stored first answer.
func (r *rig) feedbackWent(t *testing.T, id, outing string) string {
	t.Helper()
	var went *bool
	err := r.pool.QueryRow(context.Background(), `select went from app.feedback where outing_id = $1 and account_id = $2`, outing, id).Scan(&went)
	switch {
	case err != nil && err.Error() == "no rows in result set":
		return "none"
	case err != nil:
		t.Fatal(err)
	case *went:
		return "yes"
	}
	return "no"
}

// Feedback is for people who were going at the start, decided from the seat's
// history: a person who only said yes to a later outing, one who never answered,
// and one who cancelled before the start are not asked; one who cancelled after
// the start was going at the start and is.
func TestFeedbackIsForThoseGoingAtTheStartFromTheirSeatHistory(t *testing.T) {
	r := newRig(t)
	ids, outing := r.formed(t, 6)
	for _, id := range ids[:4] {
		r.answer(t, id, outing, outings.Yes)
	}
	if _, err := r.svc.Cancel(context.Background(), ids[2], outing); err != nil { // before the start
		t.Fatal(err)
	}
	r.clk.Set(lanternsStart.Add(time.Minute))
	if _, err := r.svc.Cancel(context.Background(), ids[3], outing); err != nil { // after it
		t.Fatal(err)
	}
	r.clk.Set(lanternsEnd)

	for i, want := range []error{nil, nil, outings.ErrNotThere, nil, outings.ErrNotThere, outings.ErrNotThere} {
		_, err := r.svc.Feedback(context.Background(), ids[i], outing)
		if !errors.Is(err, want) && err != want {
			t.Errorf("person %d: err = %v, want %v", i, err, want)
		}
	}
	outsider := r.member(t, 50, person{})
	if _, err := r.svc.Feedback(context.Background(), outsider, outing); !errors.Is(err, outings.ErrNotFound) {
		t.Errorf("no seat: err = %v, want ErrNotFound", err)
	}
}

// "Who else was there?": everyone who was going at the start but the person,
// all ticked by default, by first name and the handle they go by in the outing.
func TestTheFormListsEveryoneGoingAtTheStartTickedByDefault(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 4)
	r.clk.Set(lanternsStart.Add(time.Minute))
	if _, err := r.svc.Cancel(context.Background(), ids[3], outing); err != nil { // still counted: they were going at the start
		t.Fatal(err)
	}
	r.clk.Set(lanternsEnd)

	form := r.feedback(t, ids[0], outing)

	var handles []string
	for _, c := range form.Others {
		if c.FirstName != "Person" {
			t.Errorf("companion %+v, want the first name only", c)
		}
		handles = append(handles, c.Handle)
	}
	want := []string{r.handleOf(t, ids[1], outing), r.handleOf(t, ids[2], outing), r.handleOf(t, ids[3], outing)}
	slices.Sort(handles)
	slices.Sort(want)
	if !reflect.DeepEqual(handles, want) {
		t.Errorf("others = %v, want the three others %v", handles, want)
	}
	if form.Answered || len(form.NotThere) != 0 || len(form.MeetAgain) != 0 {
		t.Errorf("an unanswered form = %+v, want nothing unticked and nothing picked", form)
	}
}

// Answers are kept, read back, and changed until feedback closes: the new ones
// replace the old whole.
func TestAnswersAreKeptAndCanBeChangedUntilItCloses(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 4)
	r.clk.Set(lanternsEnd)
	h1, h2, h3 := r.handleOf(t, ids[1], outing), r.handleOf(t, ids[2], outing), r.handleOf(t, ids[3], outing)

	saved := r.saveFeedback(t, ids[0], outing, outings.FeedbackInput{Went: true, NotThere: []string{h1}, MeetAgain: []string{h2}})
	r.clk.Advance(24 * time.Hour)
	changed := r.saveFeedback(t, ids[0], outing, outings.FeedbackInput{Went: true, NotThere: []string{h3}, MeetAgain: []string{h1, h2}})
	// h1 was unticked in the first answer: the second ticks them again.
	read := r.feedback(t, ids[0], outing)

	if !saved.Answered || !saved.Went || !reflect.DeepEqual(saved.NotThere, []string{h1}) || !reflect.DeepEqual(saved.MeetAgain, []string{h2}) {
		t.Errorf("saved = %+v", saved)
	}
	sorted := func(s ...string) []string { slices.Sort(s); return s }
	if !reflect.DeepEqual(read, changed) || !reflect.DeepEqual(read.NotThere, []string{h3}) || !reflect.DeepEqual(read.MeetAgain, sorted(h1, h2)) {
		t.Errorf("after changing: read = %+v, saved %+v; want only the new answers", read, changed)
	}
}

// A "no" to "did you go?" ends the feedback there: nothing after it is kept,
// and what was said before is dropped.
func TestANoToDidYouGoEndsTheFeedbackThere(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 3)
	r.clk.Set(lanternsEnd)
	h1 := r.handleOf(t, ids[1], outing)
	r.saveFeedback(t, ids[0], outing, outings.FeedbackInput{Went: true, NotThere: []string{h1}, MeetAgain: []string{r.handleOf(t, ids[2], outing)}})

	no := r.saveFeedback(t, ids[0], outing, outings.FeedbackInput{Went: false, NotThere: []string{h1}, MeetAgain: []string{h1}})

	if !no.Answered || no.Went || len(no.NotThere) != 0 || len(no.MeetAgain) != 0 {
		t.Errorf("after no: %+v, want an answered no with nothing else", no)
	}
	var kept int
	if err := r.pool.QueryRow(context.Background(), `select (select count(*) from app.picks) + (select count(*) from app.feedback_unticks)`).Scan(&kept); err != nil || kept != 0 {
		t.Errorf("%d picks and unticks kept after a no, %v; want none", kept, err)
	}
	// Changing the no back to a yes is allowed until it closes.
	yes := r.saveFeedback(t, ids[0], outing, outings.FeedbackInput{Went: true, MeetAgain: []string{h1}})
	if !yes.Went || !reflect.DeepEqual(yes.MeetAgain, []string{h1}) {
		t.Errorf("after changing back: %+v", yes)
	}
}

// A handle that is no one else of the outing, or a pick of someone the person
// said wasn't there, is refused and nothing is kept.
func TestAnswersNamingNoOneOrContradictingThemselvesAreRefused(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 3)
	r.clk.Set(lanternsEnd)
	own := r.handleOf(t, ids[0], outing)
	h1 := r.handleOf(t, ids[1], outing)

	for name, in := range map[string]outings.FeedbackInput{
		"an unknown handle":   {Went: true, MeetAgain: []string{"00000000-0000-4000-8000-0000000000ff"}},
		"not a handle":        {Went: true, NotThere: []string{"Grace"}},
		"themselves":          {Went: true, MeetAgain: []string{own}},
		"picking who wasn't":  {Went: true, NotThere: []string{h1}, MeetAgain: []string{h1}},
		"another outing's id": {Went: true, MeetAgain: []string{ids[1]}},
	} {
		if _, err := r.svc.SaveFeedback(context.Background(), ids[0], outing, in); !errors.Is(err, outings.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
	if got := r.feedbackWent(t, ids[0], outing); got != "none" {
		t.Errorf("a refused answer left %q stored", got)
	}
}

// A person attended if they checked in or said so in feedback, and their own
// "I didn't go" wins (docs/spec/outings.md, "Attended, met and no-show").
func TestAPersonAttendedIfTheyCheckedInOrSaidSoAndTheirOwnNoWins(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 5)
	r.clk.Set(lanternsStart)
	r.checkIn(t, ids[0], outing) // checked in only
	r.checkIn(t, ids[1], outing) // checked in, then says they didn't go
	r.checkIn(t, ids[4], outing) // checked in, then says they went
	r.clk.Set(lanternsEnd)
	r.saveFeedback(t, ids[1], outing, outings.FeedbackInput{Went: false})
	r.saveFeedback(t, ids[2], outing, outings.FeedbackInput{Went: true}) // said so, never checked in
	r.saveFeedback(t, ids[4], outing, outings.FeedbackInput{Went: true})
	// ids[3] did neither.

	for i, want := range []bool{true, false, true, false, true} {
		got, err := r.svc.Attended(context.Background(), ids[i], outing)
		if err != nil || got != want {
			t.Errorf("person %d attended = %v, %v; want %v", i, got, err, want)
		}
	}
	if _, err := r.svc.Attended(context.Background(), r.member(t, 50, person{}), outing); !errors.Is(err, outings.ErrNotFound) {
		t.Errorf("no seat: err = %v, want ErrNotFound", err)
	}
}

// Each "meet again" answer is a pick, stored for the person who gave it; the
// person picked is told nothing, however they look: not in their seat, not in
// their own form, and not when the pick is mutual.
func TestAPickIsStoredAndNeverShownToThePersonPicked(t *testing.T) {
	r := newRig(t)
	ids, outing := r.going(t, 3)
	r.clk.Set(lanternsEnd)
	adaPicks, gracePicks := r.handleOf(t, ids[0], outing), r.handleOf(t, ids[1], outing)
	seatBefore, formBefore := r.seatOf(t, ids[1]), r.feedback(t, ids[1], outing)

	r.saveFeedback(t, ids[0], outing, outings.FeedbackInput{Went: true, MeetAgain: []string{gracePicks}})

	var picks int
	if err := r.pool.QueryRow(context.Background(), `select count(*) from app.picks where outing_id = $1 and picker_id = $2 and picked_id = $3`, outing, ids[0], ids[1]).Scan(&picks); err != nil || picks != 1 {
		t.Fatalf("%d picks of the second by the first, %v; want 1", picks, err)
	}
	if got := r.seatOf(t, ids[1]); !reflect.DeepEqual(got, seatBefore) {
		t.Errorf("the seat of the person picked changed: %+v, was %+v", got, seatBefore)
	}
	if got := r.feedback(t, ids[1], outing); !reflect.DeepEqual(got, formBefore) {
		t.Errorf("the form of the person picked changed: %+v, was %+v", got, formBefore)
	}
	// A mutual pick: each sees only their own.
	mutual := r.saveFeedback(t, ids[1], outing, outings.FeedbackInput{Went: true, MeetAgain: []string{adaPicks}})
	first := r.feedback(t, ids[0], outing)
	if !reflect.DeepEqual(mutual.MeetAgain, []string{adaPicks}) || !reflect.DeepEqual(first.MeetAgain, []string{gracePicks}) {
		t.Errorf("a mutual pick shows %v and %v, want each their own", mutual.MeetAgain, first.MeetAgain)
	}
}
