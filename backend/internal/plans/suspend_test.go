package plans

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationRemoveFromUpcoming takes suspended people out of a signup
// and an outing (invited and confirmed, with the group re-opening when too
// few are left). Crews: crews.TestIntegrationRemoveFromAll.
func TestIntegrationRemoveFromUpcoming(t *testing.T) {
	f := newFixture(t, "zz-plans-suspend", "UTC", 4)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "en")
	}
	a, b, c, d := f.users[0], f.users[1], f.users[2], f.users[3]
	day := func(n, h int) time.Time { return time.Date(2030, 1, n, h, 0, 0, 0, time.UTC) }
	before := day(8, 0)
	newPlan := func(area string) uuid.UUID {
		t.Helper()
		plan := matching.Plan{Area: matching.AreaID(area), Activity: matching.ActivityID(f.act), Primary: "en"}
		id, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: plan, Audience: 5, Score: 1}, Window{Start: day(7, 0), Days: 14, Deadline: day(14, 0)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	ids := func(us ...uuid.UUID) []matching.UserID {
		out := make([]matching.UserID, len(us))
		for i, u := range us {
			out[i] = matching.UserID(u.String())
		}
		return out
	}

	// An outing: a, b and c confirmed (quorum 3, minimum 2), d invited.
	outing := newPlan(f.area)
	eventID, _, err := SaveEvent(ctx, f.pool, Event{
		PlanID: outing, Round: 1, StartsAt: day(12, 14), ConfirmBy: day(11, 0), Now: day(5, 0),
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(12, 0), Block: matching.Afternoon},
			Groups:   []matching.Group{{Invited: ids(a, b, c, d), Quorum: 3, MinAttendance: 2, Score: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: f.pool}
	for _, u := range []uuid.UUID{a, b, c} {
		if err := svc.Respond(ctx, u, eventID, Confirm, before); err != nil {
			t.Fatal(err)
		}
	}
	// Another open plan a signed up for.
	other := newPlan(f.area)
	if err := SaveSignup(ctx, f.pool, other, matching.Signup{User: matching.UserID(a.String())}); err != nil {
		t.Fatal(err)
	}

	response := func(u uuid.UUID) string {
		t.Helper()
		var r string
		if err := f.pool.QueryRow(ctx, `select response from app.group_members where event_id = $1 and user_id = $2`, eventID, u).Scan(&r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	groupStatus := func() string {
		t.Helper()
		var s string
		if err := f.pool.QueryRow(ctx, `select status from app.event_groups where event_id = $1`, eventID).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}

	// a: out of the signup and the outing (the group stays on with two).
	if err := svc.RemoveFromUpcoming(ctx, a, before); err != nil {
		t.Fatal(err)
	}
	var signups int
	if err := f.pool.QueryRow(ctx, `select count(*) from app.plan_signups where user_id = $1`, a).Scan(&signups); err != nil || signups != 0 {
		t.Errorf("a's signups = %d, %v", signups, err)
	}
	if r, s := response(a), groupStatus(); r != Dropped || s != GroupConfirmed {
		t.Errorf("after a: a %s, group %s; want dropped, confirmed", r, s)
	}

	// b: one confirmed left, before the deadline: the group collects
	// answers again.
	if err := svc.RemoveFromUpcoming(ctx, b, before); err != nil {
		t.Fatal(err)
	}
	if s := groupStatus(); s != GroupInviting {
		t.Errorf("group after b = %s, want inviting", s)
	}

	// d: an invitation is declined.
	if err := svc.RemoveFromUpcoming(ctx, d, before); err != nil {
		t.Fatal(err)
	}
	if r := response(d); r != Declined {
		t.Errorf("d = %s, want declined", r)
	}

	// Again: nothing left to do, no error.
	if err := svc.RemoveFromUpcoming(ctx, a, before); err != nil {
		t.Fatalf("second run: %v", err)
	}
}
