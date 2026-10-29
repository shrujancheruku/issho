package plans

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationEvents walks one group of four through the confirmation
// rules (quorum 3, minimum 2), the deadline and settling, and a second group
// that settles as confirmed and is later marked done.
func TestIntegrationEvents(t *testing.T) {
	f := newFixture(t, "zz-plans-events", "UTC", 6)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	day := func(d, h int) time.Time { return time.Date(2030, 1, d, h, 0, 0, 0, time.UTC) }
	plan := matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: plan, Audience: 6, Score: 1}, Window{Start: day(7, 0), Days: 14, Deadline: day(14, 0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range f.users {
		if err := SaveSignup(ctx, f.pool, planID, matching.Signup{User: matching.UserID(u.String())}); err != nil {
			t.Fatal(err)
		}
	}
	ids := func(us ...uuid.UUID) []matching.UserID {
		out := make([]matching.UserID, len(us))
		for i, u := range us {
			out[i] = matching.UserID(u.String())
		}
		return out
	}
	a, b, c, d := f.users[0], f.users[1], f.users[2], f.users[3]
	starts, confirmBy := day(12, 14), day(11, 0)
	next := Window{Start: day(21, 0), Days: 14, Deadline: day(28, 0)}
	eventID, err := SaveEvent(ctx, f.pool, Event{
		PlanID: planID, Round: 1, StartsAt: starts, ConfirmBy: confirmBy, Next: &next, Now: day(5, 0),
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(12, 0), Block: matching.Afternoon},
			Groups:   []matching.Group{{Invited: ids(a, b, c, d), Quorum: 3, MinAttendance: 2, Score: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `update app.event_groups set meeting_point = '{"en":"North exit","ja":"北口"}' where event_id = $1`, eventID); err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: f.pool}
	before := day(8, 0)
	respond := func(u uuid.UUID, act Action, at time.Time) error { return svc.Respond(ctx, u, eventID, act, at) }
	group := func(u uuid.UUID) EventItem {
		t.Helper()
		it, _, _, err := svc.Event(ctx, u, eventID, before)
		if err != nil {
			t.Fatal(err)
		}
		return it
	}

	evs, err := svc.Events(ctx, a, before)
	if err != nil || len(evs) != 1 || evs[0].Response != Invited || evs[0].GroupStatus != GroupInviting ||
		evs[0].Title != "Test" || !evs[0].StartsAt.Equal(starts) || !evs[0].RevealAt().Equal(starts.Add(-RevealBefore)) {
		t.Fatalf("Events = %+v, %v", evs, err)
	}
	if _, _, _, err := svc.Event(ctx, f.users[4], eventID, before); !errors.Is(err, ErrNoEvent) {
		t.Errorf("someone not invited: %v, want ErrNoEvent", err)
	}

	// Two confirmations: still collecting (quorum 3). The third confirms the group.
	for _, u := range []uuid.UUID{a, b} {
		if err := respond(u, Confirm, before); err != nil {
			t.Fatal(err)
		}
	}
	if g := group(a); g.GroupStatus != GroupInviting || g.Confirmed != 2 {
		t.Errorf("after two: %s with %d", g.GroupStatus, g.Confirmed)
	}
	if err := respond(c, Confirm, before); err != nil {
		t.Fatal(err)
	}
	if g := group(a); g.GroupStatus != GroupConfirmed || g.Confirmed != 3 {
		t.Errorf("after three: %s with %d", g.GroupStatus, g.Confirmed)
	}

	// Only people who confirmed see who's going.
	if _, _, names, _ := svc.Event(ctx, a, eventID, before); len(names) != 3 {
		t.Errorf("attendees for a confirmed person = %v", names)
	}
	if _, _, names, _ := svc.Event(ctx, d, eventID, before); len(names) != 0 {
		t.Errorf("attendees for someone not confirmed = %v", names)
	}
	// The meeting point shows from 24 h before the start, to people going.
	if it, _, _, _ := svc.Event(ctx, a, eventID, starts.Add(-25*time.Hour)); it.MeetingPoint != "" {
		t.Errorf("meeting point too early: %q", it.MeetingPoint)
	}
	if it, _, _, _ := svc.Event(ctx, a, eventID, starts.Add(-23*time.Hour)); it.MeetingPoint != "北口" {
		t.Errorf("meeting point in the caller's language = %q, want 北口", it.MeetingPoint)
	}
	if it, _, _, _ := svc.Event(ctx, d, eventID, starts.Add(-23*time.Hour)); it.MeetingPoint != "" {
		t.Errorf("meeting point for someone not going: %q", it.MeetingPoint)
	}

	if err := respond(d, Decline, before); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		u    uuid.UUID
		act  Action
		at   time.Time
		want error
	}{
		{"declining twice", d, Decline, before, ErrNotNow},
		{"cancelling a place not confirmed", d, Cancel, before, ErrNotNow},
		{"confirming after the deadline", d, Confirm, confirmBy, ErrTooLate},
	} {
		if err := respond(c.u, c.act, c.at); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}

	// Cancelling while still at the minimum keeps the group; below it,
	// before the deadline, it goes back to collecting answers.
	if err := respond(a, Cancel, before); err != nil {
		t.Fatal(err)
	}
	if g := group(b); g.GroupStatus != GroupConfirmed || g.Confirmed != 2 {
		t.Errorf("at the minimum: %s with %d", g.GroupStatus, g.Confirmed)
	}
	if err := respond(b, Cancel, before); err != nil {
		t.Fatal(err)
	}
	if g := group(c); g.GroupStatus != GroupInviting || g.Confirmed != 1 {
		t.Errorf("below the minimum before the deadline: %s with %d", g.GroupStatus, g.Confirmed)
	}

	// At the deadline one confirmation isn't enough: the group and the event
	// are cancelled, and c goes back to waiting for the plan.
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	rep, err := r.Settle(ctx, f.city, confirmBy)
	if err != nil || rep != (SettleReport{Cancelled: 1}) {
		t.Fatalf("Settle = %+v, %v", rep, err)
	}
	if g := group(c); g.GroupStatus != GroupCancelled || g.EventStatus != EventCancelled {
		t.Errorf("after settling: group %s, event %s", g.GroupStatus, g.EventStatus)
	}
	waiting, err := f.q.ListSignups(ctx, planID)
	if err != nil || !slices.Contains(waiting, c) {
		t.Errorf("c not back on the waiting list: %v, %v", waiting, err)
	}
	if err := respond(c, Cancel, before); !errors.Is(err, ErrEventOver) {
		t.Errorf("answering a cancelled event: %v, want ErrEventOver", err)
	}

	// Round 2: two confirm, which is enough at the deadline; the day after
	// the event it's done.
	e, g := f.users[4], f.users[5]
	event2, err := SaveEvent(ctx, f.pool, Event{
		PlanID: planID, Round: 2, StartsAt: day(26, 14), ConfirmBy: day(25, 0), Now: day(20, 0),
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(26, 0), Block: matching.Afternoon},
			Groups:   []matching.Group{{Invited: ids(e, g), Quorum: 3, MinAttendance: 2, Score: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []uuid.UUID{e, g} {
		if err := svc.Respond(ctx, u, event2, Confirm, day(22, 0)); err != nil {
			t.Fatal(err)
		}
	}
	if rep, err := r.Settle(ctx, f.city, day(25, 0)); err != nil || rep != (SettleReport{Confirmed: 1}) {
		t.Errorf("Settle round 2 = %+v, %v", rep, err)
	}
	if rep, err := r.Settle(ctx, f.city, day(27, 9)); err != nil || rep.Finished != 1 {
		t.Errorf("Settle after the event = %+v, %v", rep, err)
	}
	if it, _, _, err := svc.Event(ctx, e, event2, day(27, 9)); err != nil || it.EventStatus != EventDone || it.GroupStatus != GroupConfirmed {
		t.Errorf("round 2 after the event: %+v, %v", it, err)
	}
}
