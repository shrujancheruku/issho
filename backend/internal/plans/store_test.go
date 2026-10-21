package plans

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationStore runs a plan through signups, scheduling and closing
// against a real, migrated database.
func TestIntegrationStore(t *testing.T) {
	f := newFixture(t, "zz-plans-store", "UTC", 3)
	ctx, pool, q, users := context.Background(), f.pool, f.q, f.users
	city, area, act := f.city, matching.AreaID(f.area), matching.ActivityID(f.act)
	uid := func(i int) matching.UserID { return matching.UserID(users[i].String()) }
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }

	w := Window{Start: day(7), Days: 14, Deadline: time.Date(2030, 1, 20, 12, 0, 0, 0, time.UTC)}
	plan := matching.Plan{Area: area, Activity: act, Primary: "ja"}
	planID, err := Create(ctx, q, city, matching.Suggestion{Plan: plan, Audience: 9, Score: 4.5}, w, nil)
	if err != nil {
		t.Fatal(err)
	}
	open, err := ListOpen(ctx, q, city)
	if err != nil {
		t.Fatal(err)
	}
	want := []Open{{ID: planID, OpenPlan: matching.OpenPlan{Plan: plan}, Round: 1, Window: w}}
	if !equalOpen(open, want) {
		t.Fatalf("ListOpen = %+v, want %+v", open, want)
	}

	// Signing up again replaces the times given before.
	first := matching.Signup{User: uid(0), Pattern: []matching.Slot{{Weekday: time.Monday, Block: matching.Morning}}}
	custom := matching.Signup{
		User:    uid(0),
		Pattern: []matching.Slot{{Weekday: time.Sunday, Block: matching.Evening}, {Weekday: time.Saturday, Block: matching.Morning}},
		Unavailable: []matching.Occasion{
			{Date: day(9)},                           // the whole day
			{Date: day(12), Block: matching.Evening}, // one part of it
		},
	}
	for _, s := range []matching.Signup{first, custom, {User: uid(1)}, {User: uid(2)}} {
		if err := SaveSignup(ctx, pool, planID, s); err != nil {
			t.Fatal(err)
		}
	}
	signups, err := LoadSignups(ctx, q, planID, w.Start)
	if err != nil {
		t.Fatal(err)
	}
	// Sorted by user, then (as stored) by weekday and date.
	wantSignups := []matching.Signup{{User: uid(0), Pattern: custom.Pattern, Unavailable: custom.Unavailable}, {User: uid(1)}, {User: uid(2)}}
	if !reflect.DeepEqual(signups, wantSignups) {
		t.Fatalf("LoadSignups = %+v, want %+v", signups, wantSignups)
	}

	occasion := matching.Occasion{Date: day(13), Block: matching.Evening}
	next := Window{Start: day(21), Days: 14, Deadline: day(30)}
	ev := Event{
		PlanID: planID, Round: 1, ConfirmBy: day(11), Next: &next,
		Scheduled: matching.Scheduled{
			Occasion: occasion,
			Groups:   []matching.Group{{Invited: []matching.UserID{uid(0), uid(1)}, Quorum: 2, MinAttendance: 2, Score: 1.5}},
		},
	}
	if _, err := SaveEvent(ctx, pool, Event{PlanID: planID, Round: 2, Scheduled: ev.Scheduled, ConfirmBy: day(11)}); !errors.Is(err, ErrClosed) {
		t.Errorf("SaveEvent for a round that isn't current: %v, want ErrClosed", err)
	}
	if _, err := SaveEvent(ctx, pool, ev); err != nil {
		t.Fatal(err)
	}
	// Invited people stop waiting; the third stays for the next round,
	// which has started.
	open, err = ListOpen(ctx, q, city)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].Signups != 1 || open[0].Round != 2 {
		t.Errorf("after scheduling: %+v, want one plan in round 2 with 1 signup", open)
	}

	// Signing up for another plan, the event already invited to is blocked out.
	other, err := Create(ctx, q, city, matching.Suggestion{Plan: matching.Plan{Area: area, Activity: act, Primary: "en"}, Audience: 6, Score: 1}, w, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveSignup(ctx, pool, other, matching.Signup{User: uid(0)}); err != nil {
		t.Fatal(err)
	}
	signups, err = LoadSignups(ctx, q, other, w.Start)
	if err != nil {
		t.Fatal(err)
	}
	if len(signups) != 1 || !reflect.DeepEqual(signups[0].Unavailable, []matching.Occasion{occasion}) {
		t.Errorf("busy occasions not added: %+v", signups)
	}

	if err := Close(ctx, q, other, ClosedNoGroup, day(20)); err != nil {
		t.Fatal(err)
	}
	open, err = ListOpen(ctx, q, city)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].ID != planID || open[0].Round != 2 || !open[0].Window.Start.Equal(next.Start) {
		t.Errorf("after closing the other plan: %+v", open)
	}
	if err := SaveSignup(ctx, pool, other, matching.Signup{User: uid(1)}); !errors.Is(err, ErrClosed) {
		t.Errorf("signup for a closed plan: %v, want ErrClosed", err)
	}
	if err := Withdraw(ctx, q, planID, users[2]); err != nil {
		t.Fatal(err)
	}
	if signups, err := LoadSignups(ctx, q, planID, next.Start); err != nil || len(signups) != 0 {
		t.Errorf("after withdrawing: %+v, %v", signups, err)
	}
}

func equalOpen(a, b []Open) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if x.ID != y.ID || x.OpenPlan != y.OpenPlan || x.Round != y.Round || x.Window.Days != y.Window.Days ||
			!x.Window.Start.Equal(y.Window.Start) || !x.Window.Deadline.Equal(y.Window.Deadline) {
			return false
		}
	}
	return true
}
