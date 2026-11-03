package testfixture

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

// member is one person's place in a test outing: their answer, whether they
// checked in, and what their feedback says (nil = none given).
type member struct {
	user      uuid.UUID
	response  string
	checkedIn bool
	feedback  *bool
}

// outing inserts a plan with one event and one group on date, and its members.
func outing(t *testing.T, f *Fixture, date time.Time, eventStatus, groupStatus string, members ...member) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	plan, event, group := uuid.New(), uuid.New(), uuid.New()
	stmts := [][]any{
		{`insert into app.plans (id, city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline)
		  values ($1, $2, $3, $4, 'en', 4, 1, $5::date, 7, $5::timestamptz)`, plan, f.City, f.Area, f.Act, date},
		{`insert into app.events (id, plan_id, round, date, block, confirm_by, forced, status)
		  values ($1, $2, 1, $3::date, 'afternoon', $3::timestamptz, false, $4)`, event, plan, date, eventStatus},
		{`insert into app.event_groups (id, event_id, quorum, min_attendance, score, status)
		  values ($1, $2, 2, 2, 1, $3)`, group, event, groupStatus},
	}
	for _, m := range members {
		var checkedIn any
		if m.checkedIn {
			checkedIn = date
		}
		stmts = append(stmts, []any{`insert into app.group_members (group_id, event_id, user_id, response, responded_at, checked_in_at)
			values ($1, $2, $3, $4, $5, $6)`, group, event, m.user, m.response, date, checkedIn})
	}
	// Feedback after the members exist (it references them). Ratings go with
	// "went" (event_feedback_ratings).
	for _, m := range members {
		if m.feedback == nil {
			continue
		}
		var plan, grp any
		if *m.feedback {
			plan, grp = 4, 4
		}
		stmts = append(stmts, []any{`insert into app.event_feedback (group_id, user_id, attended, plan_rating, group_rating)
			values ($1, $2, $3, $4, $5)`, group, m.user, *m.feedback, plan, grp})
	}
	for _, s := range stmts {
		if _, err := f.Pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
			t.Fatal(err)
		}
	}
	return group
}

func ptr(b bool) *bool { return &b }

// TestIntegrationDomainRuleViews: app.attendance, app.suspended_users and
// app.kept_apart, seen through the queries that read them (#186).
func TestIntegrationDomainRuleViews(t *testing.T) {
	f := New(t, "zz-rules", "UTC", 7)
	ctx := context.Background()
	for _, u := range f.Users {
		f.AddProfile(t, u, "en")
	}
	// a checked in; b checked in but says they didn't go; c says they went
	// without checking in; d declined; e checked in at a cancelled group;
	// g went (feedback) at a finished outing too, and is the one picks and
	// reports are tested on.
	a, b, c, d, e, g := f.Users[0], f.Users[1], f.Users[2], f.Users[3], f.Users[4], f.Users[5]
	monday := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC) // a Monday
	outing(t, f, monday, "done", "confirmed",
		member{user: a, response: "confirmed", checkedIn: true},
		member{user: b, response: "confirmed", checkedIn: true, feedback: ptr(false)},
		member{user: c, response: "confirmed", feedback: ptr(true)},
		member{user: d, response: "declined"},
		member{user: g, response: "confirmed", feedback: ptr(true)})
	outing(t, f, monday.AddDate(0, 0, 1), "done", "cancelled",
		member{user: e, response: "confirmed", checkedIn: true})

	attended := func() map[uuid.UUID]int32 {
		t.Helper()
		rows, err := f.Q.ListMatchingUsers(ctx, f.City)
		if err != nil {
			t.Fatal(err)
		}
		got := map[uuid.UUID]int32{}
		for _, r := range rows {
			if r.DoneAttended != r.EventsAttended {
				t.Errorf("%s: done_attended %d != events_attended %d", r.UserID, r.DoneAttended, r.EventsAttended)
			}
			got[r.UserID] = r.DoneAttended
		}
		return got
	}
	got := attended()
	for _, c := range []struct {
		who  uuid.UUID
		name string
		want int32
	}{
		{a, "checked in", 1},
		{b, "checked in but feedback says no (the feedback wins)", 0},
		{c, "feedback says went, no check-in", 1},
		{d, "declined", 0},
		{e, "checked in at a cancelled group", 0},
	} {
		if got[c.who] != c.want {
			t.Errorf("reliability/newcomer count, %s: attended %d, want %d", c.name, got[c.who], c.want)
		}
	}

	// Metrics: a, c and g attended; b is invited and confirmed but didn't go.
	rows, err := f.Q.WeeklyMetrics(ctx, dbgen.WeeklyMetricsParams{
		CityID: f.City,
		First:  pgtype.Date{Time: monday, Valid: true},
		Last:   pgtype.Date{Time: monday, Valid: true},
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("WeeklyMetrics = %v, %v", rows, err)
	}
	if rows[0].Attendees != 3 || rows[0].Attended != 3 || rows[0].Confirmed != 4 || rows[0].Invited != 5 {
		t.Errorf("metrics attendees/attended/confirmed/invited = %d/%d/%d/%d, want 3/3/4/5",
			rows[0].Attendees, rows[0].Attended, rows[0].Confirmed, rows[0].Invited)
	}

	// Met: a (checked in) and c (said they went) met though only one of them
	// gave feedback; b (feedback no), d and e didn't attend, so met nobody.
	pairs := func() map[[2]uuid.UUID]bool {
		t.Helper()
		rows, err := f.Q.ListCityMetPairs(ctx, f.City)
		if err != nil {
			t.Fatal(err)
		}
		m := map[[2]uuid.UUID]bool{}
		for _, r := range rows {
			m[[2]uuid.UUID{r.AID, r.BID}] = true
		}
		return m
	}
	order := func(x, y uuid.UUID) [2]uuid.UUID {
		if bytes.Compare(x[:], y[:]) > 0 {
			x, y = y, x
		}
		return [2]uuid.UUID{x, y}
	}
	want := map[[2]uuid.UUID]bool{order(a, c): true, order(a, g): true, order(c, g): true}
	if got := pairs(); len(got) != len(want) || !got[order(a, c)] || !got[order(a, g)] || !got[order(c, g)] {
		t.Errorf("met pairs = %v, want %v", got, want)
	}

	// Suspended: out of matching, met pairs and IsSuspended while it lasts.
	if _, err := f.Pool.Exec(ctx, `insert into app.suspensions (user_id, reason, created_by) values ($1, 'test', $1)`, a); err != nil {
		t.Fatal(err)
	}
	if s, err := f.Q.IsSuspended(ctx, a); err != nil || !s {
		t.Errorf("IsSuspended = %v, %v, want true", s, err)
	}
	if _, ok := attended()[a]; ok {
		t.Error("a suspended person is still matched")
	}
	if got := pairs(); len(got) != 1 || !got[order(c, g)] {
		t.Errorf("met pairs with a suspended = %v, want only c-g", got)
	}
	if ids, err := f.Q.ListSuspendedIDs(ctx); err != nil || !contains(ids, a) {
		t.Errorf("ListSuspendedIDs = %v, %v, want a in it", ids, err)
	}
	if n, err := f.Q.LiftSuspension(ctx, dbgen.LiftSuspensionParams{
		At: pgtype.Timestamptz{Time: time.Now(), Valid: true}, LiftedBy: pgtype.UUID{Bytes: a, Valid: true}, UserID: a}); err != nil || n != 1 {
		t.Fatalf("LiftSuspension = %d, %v", n, err)
	}
	if s, _ := f.Q.IsSuspended(ctx, a); s {
		t.Error("IsSuspended after lifting")
	}
	if _, ok := attended()[a]; !ok || len(pairs()) != 3 {
		t.Error("lifting didn't bring a back")
	}

	// Kept apart: a report (c reported g) and a "rather not" (g would rather
	// not meet a) are both in ListCityBlocks, and each hides the friendship
	// from the other's friend picks, in either direction.
	if _, err := f.Pool.Exec(ctx, `insert into app.mate_picks (group_id, picker_id, picked_id, pick)
		select group_id, $1, $2, 'avoid' from app.group_members where user_id = $1 limit 1`, g, a); err != nil {
		t.Fatal(err)
	}
	// c and g picked each other, so they are friends until the report.
	for _, p := range [][2]uuid.UUID{{c, g}, {g, c}} {
		if _, err := f.Pool.Exec(ctx, `insert into app.mate_picks (group_id, picker_id, picked_id, pick)
			select group_id, $1, $2, 'again' from app.group_members where user_id = $1 limit 1`, p[0], p[1]); err != nil {
			t.Fatal(err)
		}
	}
	friendRows := func() int {
		t.Helper()
		rows, err := f.Q.ListFriendPicks(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		return len(rows)
	}
	if n := friendRows(); n != 2 {
		t.Errorf("friend picks before the report = %d, want 2", n)
	}
	if _, err := f.Pool.Exec(ctx, `insert into app.reports (reporter_id, reported_id, reason) values ($1, $2, 'other')`, c, g); err != nil {
		t.Fatal(err)
	}
	blocks, err := f.Q.ListCityBlocks(ctx, f.City)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[[2]uuid.UUID]bool{}
	for _, b := range blocks {
		seen[[2]uuid.UUID{b.ReporterID, b.OtherID}] = true
	}
	if len(seen) != 2 || !seen[[2]uuid.UUID{g, a}] || !seen[[2]uuid.UUID{c, g}] {
		t.Errorf("blocks = %v, want g->a and c->g", seen)
	}
	// Reported by c: g is no longer c's friend, so only c's own pick is left.
	if n := friendRows(); n != 1 {
		t.Errorf("friend picks after the report = %d, want 1 (c's own)", n)
	}
}

func contains(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
