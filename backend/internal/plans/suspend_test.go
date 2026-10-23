package plans

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// TestIntegrationRemoveFromUpcoming takes suspended people out of a signup,
// an outing (invited and confirmed, with the group re-opening when too few
// are left) and crews (an active one closing, an offer declined).
func TestIntegrationRemoveFromUpcoming(t *testing.T) {
	f := newFixture(t, "zz-plans-suspend", "UTC", 5)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "en")
	}
	a, b, c, d, e := f.users[0], f.users[1], f.users[2], f.users[3], f.users[4]
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
	eventID, err := SaveEvent(ctx, f.pool, Event{
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
	// An active crew of a, b and c, and an offer to a and e.
	var active, offer uuid.UUID
	if err := f.pool.QueryRow(ctx, `insert into app.crews (city_id, activity_id, offer_until, status) values ($1, $2, $3, 'active') returning id`,
		f.city, f.act, day(1, 0)).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if err := f.pool.QueryRow(ctx, `insert into app.crews (city_id, activity_id, offer_until) values ($1, $2, $3) returning id`,
		f.city, f.act, day(20, 0)).Scan(&offer); err != nil {
		t.Fatal(err)
	}
	for _, s := range [][]any{
		{`insert into app.crew_members (crew_id, user_id, response, responded_at) select $1, unnest($2::uuid[]), 'accepted', $3`, active, []uuid.UUID{a, b, c}, day(2, 0)},
		{`insert into app.crew_members (crew_id, user_id) select $1, unnest($2::uuid[])`, offer, []uuid.UUID{a, e}},
	} {
		if _, err := f.pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
			t.Fatal(err)
		}
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
	crew := func(id, u uuid.UUID) (status, resp string) {
		t.Helper()
		if err := f.pool.QueryRow(ctx, `select c.status, m.response from app.crews c join app.crew_members m on m.crew_id = c.id where c.id = $1 and m.user_id = $2`,
			id, u).Scan(&status, &resp); err != nil {
			t.Fatal(err)
		}
		return
	}

	// a: out of the signup, the outing (the group stays on with two) and
	// both crews.
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
	if s, r := crew(active, a); s != CrewActive || r != CrewLeft {
		t.Errorf("active crew after a: %s, a %s", s, r)
	}
	if s, r := crew(offer, a); s != CrewOffered || r != CrewDeclined {
		t.Errorf("offer after a: %s, a %s", s, r)
	}
	var leaves int
	if err := f.pool.QueryRow(ctx, `select count(*) from app.crew_leaves where crew_id = $1`, active).Scan(&leaves); err != nil || leaves != 0 {
		t.Errorf("leave reasons stored = %d, %v (none: it wasn't their choice)", leaves, err)
	}

	// b: one confirmed left, before the deadline: the group collects
	// answers again; the crew is down to c and closes.
	if err := svc.RemoveFromUpcoming(ctx, b, before); err != nil {
		t.Fatal(err)
	}
	if s := groupStatus(); s != GroupInviting {
		t.Errorf("group after b = %s, want inviting", s)
	}
	if s, _ := crew(active, c); s != CrewClosed {
		t.Errorf("crew with one left = %s, want closed", s)
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
