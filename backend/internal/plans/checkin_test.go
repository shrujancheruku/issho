package plans

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationCheckIn: check-in is open from 30 minutes before the start
// to 3 hours after, only for people going; once the event is done, the one
// who checked in counts as having attended and stays fully reliable, and
// the one who didn't drops to 2/3.
func TestIntegrationCheckIn(t *testing.T) {
	f := newFixture(t, "zz-plans-checkin", "UTC", 3)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	a, b, c := f.users[0], f.users[1], f.users[2]
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}, Audience: 6, Score: 1},
		Window{Start: day(7), Days: 14, Deadline: day(14)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	starts := day(12).Add(14 * time.Hour)
	eventID, err := SaveEvent(ctx, f.pool, Event{
		PlanID: planID, Round: 1, StartsAt: starts, ConfirmBy: day(11), Now: day(5),
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(12), Block: matching.Afternoon},
			Groups:   []matching.Group{{Invited: []matching.UserID{matching.UserID(a.String()), matching.UserID(b.String()), matching.UserID(c.String())}, Quorum: 2, MinAttendance: 2, Score: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: f.pool}
	for _, u := range []uuid.UUID{a, b} {
		if err := svc.Respond(ctx, u, eventID, Confirm, day(6)); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.CheckIn(ctx, a, eventID, starts.Add(-31*time.Minute)); !errors.Is(err, ErrCheckInClosed) {
		t.Errorf("31 minutes early: %v, want ErrCheckInClosed", err)
	}
	if err := svc.CheckIn(ctx, c, eventID, starts); !errors.Is(err, ErrNotGoing) {
		t.Errorf("someone only invited: %v, want ErrNotGoing", err)
	}
	arrived := starts.Add(-10 * time.Minute)
	if err := svc.CheckIn(ctx, a, eventID, arrived); err != nil {
		t.Fatal(err)
	}
	if err := svc.CheckIn(ctx, a, eventID, starts.Add(time.Hour)); err != nil {
		t.Errorf("checking in again: %v", err)
	}
	var at time.Time
	if err := f.pool.QueryRow(ctx, "select checked_in_at from app.group_members where event_id = $1 and user_id = $2", eventID, a).Scan(&at); err != nil || !at.Equal(arrived) {
		t.Errorf("checked in at %v, %v; want the first time %v", at, err, arrived)
	}
	if err := svc.CheckIn(ctx, b, eventID, starts.Add(CheckInAfter+time.Minute)); !errors.Is(err, ErrCheckInClosed) {
		t.Errorf("3 hours late: %v, want ErrCheckInClosed", err)
	}
	if it, _, _, err := svc.Event(ctx, a, eventID, starts); err != nil || !it.CheckedIn || !it.CheckInFrom.Equal(starts.Add(-CheckInBefore)) {
		t.Errorf("event for a = %+v, %v", it, err)
	}

	// The day after, the event is done: a attended (and stays reliable), b
	// didn't turn up.
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if _, err := r.Settle(ctx, f.city, day(13)); err != nil {
		t.Fatal(err)
	}
	city, err := LoadCity(ctx, f.q, f.city, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range city.Users {
		want := map[matching.UserID][2]float64{
			matching.UserID(a.String()): {1, 1},       // attended once, reliability 1
			matching.UserID(b.String()): {0, 2.0 / 3}, // missed once
			matching.UserID(c.String()): {0, 1},       // never confirmed: no change
		}[u.ID]
		if float64(u.EventsAttended) != want[0] || math.Abs(u.Reliability-want[1]) > 1e-6 {
			t.Errorf("%s: attended %d, reliability %.3f; want %v", u.ID, u.EventsAttended, u.Reliability, want)
		}
	}
}
