package plans

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// history is one outing a person was in, for seedOuting: the event's and the
// group's status, their response, and whether they checked in or what their
// feedback said (nil = none).
type history struct {
	event, group, response string
	checkedIn              bool
	feedback               *bool
}

// seedOuting inserts a plan with one event and one group, and user as its
// only member. Plans are one per outing, so nothing here depends on the
// fixture's other plans.
func seedOuting(t *testing.T, f *fixture, user uuid.UUID, day time.Time, h history) {
	t.Helper()
	ctx := context.Background()
	plan, event, group := uuid.New(), uuid.New(), uuid.New()
	var checkedIn any
	if h.checkedIn {
		checkedIn = day
	}
	stmts := [][]any{
		{`insert into app.plans (id, city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline)
		  values ($1, $2, $3, $4, 'en', 4, 1, $5::date, 7, $5::timestamptz)`, plan, f.city, f.area, f.act, day},
		{`insert into app.events (id, plan_id, round, date, block, confirm_by, forced, status)
		  values ($1, $2, 1, $3::date, 'afternoon', $3::timestamptz, false, $4)`, event, plan, day, h.event},
		{`insert into app.event_groups (id, event_id, quorum, min_attendance, score, status)
		  values ($1, $2, 2, 2, 1, $3)`, group, event, h.group},
		{`insert into app.group_members (group_id, event_id, user_id, response, responded_at, checked_in_at)
		  values ($1, $2, $3, $4, $5, $6)`, group, event, user, h.response, day, checkedIn},
	}
	if h.feedback != nil {
		var rating any
		if *h.feedback {
			rating = 4 // ratings go with "went" (event_feedback_ratings)
		}
		stmts = append(stmts, []any{`insert into app.event_feedback (group_id, user_id, attended, plan_rating, group_rating)
			values ($1, $2, $3, $4, $4)`, group, user, *h.feedback, rating})
	}
	for _, s := range stmts {
		if _, err := f.pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
			t.Fatal(err)
		}
	}
}

func yes() *bool { b := true; return &b }
func no() *bool  { b := false; return &b }

// TestIntegrationMatchingCounts pins the three counts ListMatchingUsers
// returns for a person with a mixed history (#185). The query computes them
// in one pass over the person's memberships, so each outing below is a case
// where the counts could drift apart:
//
//   - done_confirmed counts every confirmed place at a finished event,
//     including one at a group that was cancelled (the group never met, but
//     the confirmation stands in the denominator);
//   - done_attended and events_attended are the same number: app.attendance's
//     rule (went ahead, and checked in or feedback says so; feedback "no"
//     wins over a check-in).
func TestIntegrationMatchingCounts(t *testing.T) {
	f := newFixture(t, "zz-plans-counts", "UTC", 2)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "en")
	}
	mixed := f.users[0] // f.users[1] is a newcomer with no history
	day := func(d int) time.Time { return time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC) }
	for i, h := range []history{
		{"done", "confirmed", "confirmed", true, nil},       // attended (check-in): +1 confirmed, +1 attended
		{"done", "confirmed", "confirmed", false, yes()},    // attended (feedback, no check-in): +1, +1
		{"done", "confirmed", "confirmed", true, no()},      // checked in but feedback says no: +1 confirmed only
		{"done", "cancelled", "confirmed", true, nil},       // group cancelled: +1 confirmed only
		{"done", "confirmed", "declined", false, nil},       // declined: neither
		{"done", "confirmed", "dropped", false, nil},        // cancelled in advance: neither
		{"scheduled", "confirmed", "confirmed", false, nil}, // not finished yet: neither
		{"cancelled", "cancelled", "confirmed", false, nil}, // event cancelled: neither
	} {
		seedOuting(t, f, mixed, day(i+1), h)
	}

	rows, err := f.q.ListMatchingUsers(ctx, f.city)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("ListMatchingUsers returned %d people, want 2", len(rows))
	}
	for _, r := range rows {
		want := [3]int32{0, 0, 0}
		if r.UserID == mixed {
			want = [3]int32{2, 4, 2}
		}
		if got := [3]int32{r.DoneAttended, r.DoneConfirmed, r.EventsAttended}; got != want {
			t.Errorf("%s: done_attended/done_confirmed/events_attended = %v, want %v", r.UserID, got, want)
		}
	}
}
