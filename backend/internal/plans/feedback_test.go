package plans

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// feedbackEvent creates a plan and an event starting on 2030-01-12 at
// 14:00 UTC with one group inviting everyone given; the first `confirm` of
// them confirm.
func feedbackEvent(t *testing.T, f *fixture, invited []uuid.UUID, confirm int) (uuid.UUID, time.Time) {
	t.Helper()
	ctx := context.Background()
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}, Audience: 6, Score: 1},
		Window{Start: day(7), Days: 14, Deadline: day(14)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]matching.UserID, len(invited))
	for i, u := range invited {
		ids[i] = matching.UserID(u.String())
	}
	starts := day(12).Add(14 * time.Hour)
	eventID, err := SaveEvent(ctx, f.pool, Event{
		PlanID: planID, Round: 1, StartsAt: starts, ConfirmBy: day(11), Now: day(5),
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(12), Block: matching.Afternoon},
			Groups:   []matching.Group{{Invited: ids, Quorum: int(min(3, confirm)), MinAttendance: 2, Score: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: f.pool}
	for _, u := range invited[:confirm] {
		if err := svc.Respond(ctx, u, eventID, Confirm, day(6)); err != nil {
			t.Fatal(err)
		}
	}
	return eventID, starts
}

// TestIntegrationFeedback: feedback opens 3 hours after the start for
// people who went ahead with the group; the worker asks once; picks only of
// group mates; mutual picks and reports reach matching; saying you went
// counts as attending.
func TestIntegrationFeedback(t *testing.T) {
	f := newFixture(t, "zz-plans-feedback", "UTC", 4)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	a, b, c, d := f.users[0], f.users[1], f.users[2], f.users[3]
	eventID, starts := feedbackEvent(t, f, f.users, 3) // d only invited
	if err := (&Service{DB: f.pool}).Respond(ctx, d, eventID, Decline, starts.Add(-72*time.Hour)); err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: f.pool}
	open := starts.Add(FeedbackAfter)

	if fm, err := svc.FeedbackForm(ctx, a, eventID, starts); err != nil || fm.Open || !fm.OpensAt.Equal(open) {
		t.Errorf("form at the start = %+v, %v; want closed until %v", fm, err, open)
	}
	good := Feedback{Attended: true, PlanRating: 4, GroupRating: 5, Picks: []uuid.UUID{b}}
	if err := svc.SubmitFeedback(ctx, a, eventID, good, starts.Add(time.Hour)); !errors.Is(err, ErrFeedbackClosed) {
		t.Errorf("before it opens: %v, want ErrFeedbackClosed", err)
	}
	if _, err := svc.FeedbackForm(ctx, d, eventID, open); !errors.Is(err, ErrNotGoing) {
		t.Errorf("someone who declined: %v, want ErrNotGoing", err)
	}

	// The worker asks the three who went, once.
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	for i, want := range []int{3, 0} {
		rep, err := r.Settle(ctx, f.city, open.Add(time.Minute))
		if err != nil || rep.FeedbackAsked != want {
			t.Errorf("settle %d: asked %d, %v; want %d", i, rep.FeedbackAsked, err, want)
		}
	}

	now := open.Add(time.Hour)
	for _, bad := range []Feedback{
		{Attended: true, PlanRating: 0, GroupRating: 3},
		{Attended: true, PlanRating: 3, GroupRating: 6},
		{Attended: true, PlanRating: 3, GroupRating: 3, Picks: []uuid.UUID{d}},                        // declined
		{Attended: true, PlanRating: 3, GroupRating: 3, Picks: []uuid.UUID{a}},                        // themself
		{Attended: true, PlanRating: 3, GroupRating: 3, Picks: []uuid.UUID{b}, Avoid: []uuid.UUID{b}}, // both lists
	} {
		var inv *InvalidError
		if err := svc.SubmitFeedback(ctx, a, eventID, bad, now); !errors.As(err, &inv) {
			t.Errorf("%+v: %v, want invalid", bad, err)
		}
	}
	// c hasn't answered yet: the event is still in their list days later.
	later := starts.Add(5 * 24 * time.Hour)
	items, err := svc.Events(ctx, c, later)
	if err != nil || len(items) != 1 || items[0].FeedbackGiven || !items[0].FeedbackFrom.Equal(open) {
		t.Errorf("c's events days later = %+v, %v; want the event, feedback due", items, err)
	}

	if err := svc.SubmitFeedback(ctx, a, eventID, Feedback{Attended: true, PlanRating: 2, GroupRating: 2, Picks: []uuid.UUID{b, c}}, now); err != nil {
		t.Fatal(err)
	}
	// Sending again replaces it.
	if err := svc.SubmitFeedback(ctx, a, eventID, good, now); err != nil {
		t.Fatal(err)
	}
	if err := svc.SubmitFeedback(ctx, b, eventID, Feedback{Attended: true, PlanRating: 5, GroupRating: 5, Picks: []uuid.UUID{a, a}}, now); err != nil {
		t.Fatal(err)
	}
	// c didn't go: ratings and picks sent anyway are dropped.
	if err := svc.SubmitFeedback(ctx, c, eventID, Feedback{Attended: false, PlanRating: 5, GroupRating: 5, Picks: []uuid.UUID{a}}, now); err != nil {
		t.Fatal(err)
	}
	fm, err := svc.FeedbackForm(ctx, a, eventID, now)
	mates := []uuid.UUID{}
	for _, m := range fm.Mates {
		mates = append(mates, m.ID)
	}
	if err != nil || !fm.Given || fm.PlanRating != 4 || fm.GroupRating != 5 || !slices.Equal(fm.Picks, []uuid.UUID{b}) || len(mates) != 2 || slices.Contains(mates, d) {
		t.Errorf("a's form = %+v, %v", fm, err)
	}
	if fm, err := svc.FeedbackForm(ctx, c, eventID, now); err != nil || !fm.Given || fm.Attended || fm.PlanRating != 0 || len(fm.Picks) != 0 {
		t.Errorf("c's form = %+v, %v", fm, err)
	}
	if items, err := svc.Events(ctx, c, later); err != nil || len(items) != 0 {
		t.Errorf("c's events after answering = %d, %v; want none", len(items), err)
	}
	if err := svc.SubmitFeedback(ctx, a, eventID, good, starts.Add(FeedbackFor)); !errors.Is(err, ErrFeedbackClosed) {
		t.Errorf("after two weeks: %v, want ErrFeedbackClosed", err)
	}

	// Reports: only about someone in the group, for 30 days.
	var inv *InvalidError
	if err := svc.Report(ctx, a, eventID, d, ReportDating, "", now); !errors.As(err, &inv) {
		t.Errorf("reporting someone who declined: %v, want invalid", err)
	}
	if err := svc.Report(ctx, a, eventID, c, "rude", "", now); !errors.As(err, &inv) {
		t.Errorf("unknown reason: %v, want invalid", err)
	}
	if err := svc.Report(ctx, a, eventID, c, ReportNoShow, "", starts.Add(ReportFor+time.Minute)); !errors.Is(err, ErrTooLate) {
		t.Errorf("after 30 days: %v, want ErrTooLate", err)
	}
	if err := svc.Report(ctx, d, eventID, a, ReportOther, "", now); !errors.Is(err, ErrNotGoing) {
		t.Errorf("reported by someone who declined: %v, want ErrNotGoing", err)
	}
	for _, details := range []string{"first", "  didn't come  "} {
		if err := svc.Report(ctx, a, eventID, c, ReportNoShow, details, now); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	var details string
	if err := f.pool.QueryRow(ctx, "select count(*), max(details) from app.reports where reporter_id = $1", a).Scan(&n, &details); err != nil || n != 1 || details != "didn't come" {
		t.Errorf("reports = %d %q, %v; want one, updated and trimmed", n, details, err)
	}

	// The next day: a and b are linked, a and c blocked; a and b attended
	// (by their word), c didn't.
	if _, err := r.Settle(ctx, f.city, starts.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	city, err := LoadCity(ctx, f.q, f.city, 0)
	if err != nil {
		t.Fatal(err)
	}
	id := func(u uuid.UUID) matching.UserID { return matching.UserID(u.String()) }
	if got := city.Relations.Mutual; len(got) != 1 || got[matching.MakePair(id(a), id(b))] != 1 {
		t.Errorf("mutual = %v, want a-b at 1", got)
	}
	if got := city.Relations.Blocked; len(got) != 1 || !got[matching.MakePair(id(c), id(a))] {
		t.Errorf("blocked = %v, want a-c", got)
	}
	// b, whom a gets on with, signs up for a new plan (with c): a's feed
	// counts one friend (a count, never who); d, with no mutual pick, none.
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}, Audience: 6, Score: 1},
		Window{Start: starts.AddDate(0, 0, 3), Days: 14, Deadline: starts.AddDate(0, 0, 10)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []uuid.UUID{b, c} {
		if _, err := f.pool.Exec(ctx, "insert into app.plan_signups (plan_id, user_id) values ($1, $2)", planID, u); err != nil {
			t.Fatal(err)
		}
	}
	friendsOn := func(u uuid.UUID) int {
		feed, err := svc.Feed(ctx, u)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range feed {
			if it.ID == planID {
				return it.Friends
			}
		}
		t.Fatalf("plan not in the feed")
		return 0
	}
	if got := friendsOn(a); got != 1 {
		t.Errorf("a's feed: %d friends interested, want 1", got)
	}
	if got := friendsOn(d); got != 0 {
		t.Errorf("d's feed: %d friends interested, want 0", got)
	}
	for _, u := range city.Users {
		want := map[matching.UserID][2]float64{
			id(a): {1, 1}, id(b): {1, 1}, id(c): {0, 2.0 / 3}, id(d): {0, 1},
		}[u.ID]
		if float64(u.EventsAttended) != want[0] || math.Abs(u.Reliability-want[1]) > 1e-6 {
			t.Errorf("%s: attended %d, reliability %.3f; want %v", u.ID, u.EventsAttended, u.Reliability, want)
		}
	}
}

// TestIntegrationPickLean: someone who picks seven women and no men from a
// group of seven of each is flagged for a moderator, once.
func TestIntegrationPickLean(t *testing.T) {
	f := newFixture(t, "zz-plans-picklean", "UTC", 15)
	ctx := context.Background()
	for i, u := range f.users {
		f.addProfile(t, u, "ja")
		gender := "man"
		if i >= 1 && i <= 7 {
			gender = "woman"
		}
		if _, err := f.pool.Exec(ctx, "update app.profiles set gender = $2 where user_id = $1", u, gender); err != nil {
			t.Fatal(err)
		}
	}
	eventID, starts := feedbackEvent(t, f, f.users, len(f.users))
	svc := &Service{DB: f.pool}
	picker, women := f.users[0], f.users[1:8]
	now := starts.Add(FeedbackAfter)
	flags := func() (int, matching.Lean) {
		var n int
		var raw []byte
		if err := f.pool.QueryRow(ctx, "select count(*), coalesce(max(details::text), '{}')::bytea from app.moderation_flags where user_id = $1 and kind = $2", picker, FlagPickLean).Scan(&n, &raw); err != nil {
			t.Fatal(err)
		}
		var lean matching.Lean
		_ = json.Unmarshal(raw, &lean)
		return n, lean
	}
	// Six picks to women: not yet enough to be unlikely by chance.
	if err := svc.SubmitFeedback(ctx, picker, eventID, Feedback{Attended: true, PlanRating: 3, GroupRating: 3, Picks: women[:6]}, now); err != nil {
		t.Fatal(err)
	}
	if n, _ := flags(); n != 0 {
		t.Errorf("flagged after six picks")
	}
	for range 2 {
		if err := svc.SubmitFeedback(ctx, picker, eventID, Feedback{Attended: true, PlanRating: 3, GroupRating: 3, Picks: women}, now); err != nil {
			t.Fatal(err)
		}
	}
	if n, lean := flags(); n != 1 || lean.Gender != matching.Woman || lean.Picked != 7 || lean.Met != 14 {
		t.Errorf("flags = %d %+v; want one, 7 of 7 picks to women who were 7 of 14", n, lean)
	}
}
