package crews

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/push"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

func day(n int) time.Time { return time.Date(2030, 1, n, 0, 0, 0, 0, time.UTC) }

// outing schedules an event on a new plan for the people invited, with
// everyone confirmed, and returns it and its start.
func outing(t *testing.T, f *testfixture.Fixture, people []uuid.UUID) (uuid.UUID, time.Time) {
	t.Helper()
	ctx := context.Background()
	planID, err := plans.Create(ctx, f.Q, f.City, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.Area), Activity: matching.ActivityID(f.Act), Primary: "ja"}, Audience: 6, Score: 1},
		plans.Window{Start: day(7), Days: 14, Deadline: day(14)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]matching.UserID, len(people))
	for i, u := range people {
		ids[i] = matching.UserID(u.String())
	}
	starts := day(12).Add(14 * time.Hour)
	eventID, _, err := plans.SaveEvent(ctx, f.Pool, plans.Event{
		PlanID: planID, Round: 1, StartsAt: starts, ConfirmBy: day(11), Now: day(5),
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(12), Block: matching.Afternoon},
			Groups:   []matching.Group{{Invited: ids, Quorum: min(3, len(people)), MinAttendance: 2, Score: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := &plans.Service{DB: f.Pool}
	for _, u := range people {
		if err := svc.Respond(ctx, u, eventID, plans.Confirm, day(6)); err != nil {
			t.Fatal(err)
		}
	}
	return eventID, starts
}

// sender records the pushes a Notifier sends.
type sender struct{ sent []push.Message }

func (s *sender) Send(_ context.Context, msgs []push.Message) ([]push.Ticket, error) {
	s.sent = append(s.sent, msgs...)
	tickets := make([]push.Ticket, len(msgs))
	for i, m := range msgs {
		tickets[i] = push.Ticket{OK: true, ID: m.To}
	}
	return tickets, nil
}

func (s *sender) Receipts(context.Context, []string) (map[string]push.Receipt, error) {
	return nil, nil
}

// TestIntegrationCrews: A met B and D at one outing, C and D at another,
// and B met C at a third. A–B and B–C picked each other; A picked D but D
// didn't pick anyone, and C would rather not see D again. A, B and C are
// offered a crew named after the activity they all love; D, with no mutual
// pick, isn't in one. (Had D picked A back, A, B, D would be an equally good
// crew and the tie would go by user id.) It forms when all three accept,
// gets a plan suggested a day later, and closes when only one is left.
func TestIntegrationCrews(t *testing.T) {
	f := testfixture.New(t, "zz-crews", "UTC", 4)
	ctx := context.Background()
	for _, u := range f.Users {
		f.AddProfile(t, u, "ja")
	}
	a, b, c, d := f.Users[0], f.Users[1], f.Users[2], f.Users[3]
	plansSvc := &plans.Service{DB: f.Pool}
	svc := &Service{DB: f.Pool}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := &Runner{DB: f.Pool, Log: log}

	type answer struct {
		who          uuid.UUID
		picks, avoid []uuid.UUID
	}
	met := func(people []uuid.UUID, answers ...answer) time.Time {
		eventID, starts := outing(t, f, people)
		for _, an := range answers {
			fb := plans.Feedback{Attended: true, PlanRating: 4, GroupRating: 4, Picks: an.picks, Avoid: an.avoid}
			if err := plansSvc.SubmitFeedback(ctx, an.who, eventID, fb, starts.Add(plans.FeedbackAfter)); err != nil {
				t.Fatal(err)
			}
		}
		return starts
	}
	ids := func(us ...uuid.UUID) []uuid.UUID { return us }
	met(ids(a, b, d), answer{a, ids(b, d), nil}, answer{b, ids(a), nil}, answer{d, nil, nil})
	met(ids(a, c, d), answer{a, nil, nil}, answer{c, nil, ids(d)}, answer{d, nil, nil})
	starts := met(ids(b, c), answer{b, ids(c), nil}, answer{c, ids(b), nil})

	now := starts.Add(plans.FeedbackAfter + time.Hour)
	rep, err := r.Pass(ctx, f.City, now)
	if err != nil || rep.Offered != 1 {
		t.Fatalf("crews pass = %+v, %v; want one offered", rep, err)
	}
	crews, err := svc.List(ctx, a)
	if err != nil || len(crews) != 1 {
		t.Fatalf("a's crews = %+v, %v", crews, err)
	}
	crew := crews[0]
	var members []uuid.UUID
	for _, m := range crew.Members {
		members = append(members, m.ID)
	}
	want := map[uuid.UUID]bool{a: true, b: true, c: true}
	if crew.Status != Offered || crew.Activity != f.Act || len(members) != 3 || !want[members[0]] || !want[members[1]] || !want[members[2]] {
		t.Errorf("crew = %+v; want an offer to a, b and c on %s", crew, f.Act)
	}
	if _, err := svc.Get(ctx, d, crew.ID); !errors.Is(err, ErrNoCrew) {
		t.Errorf("d sees the crew: %v", err)
	}
	if rep, err := r.Pass(ctx, f.City, now.Add(time.Hour)); err != nil || rep.Offered != 0 {
		t.Errorf("second pass = %+v, %v; want no new offer", rep, err)
	}

	// The push is titled with the crew's name.
	if err := f.Q.UpsertPushToken(ctx, dbgen.UpsertPushTokenParams{Token: "ExponentPushToken[crew-a]", UserID: a, Platform: "android"}); err != nil {
		t.Fatal(err)
	}
	catalog, err := uistrings.Load()
	if err != nil {
		t.Fatal(err)
	}
	pushes := &sender{}
	n := &plans.Notifier{Plans: plansSvc, Sender: pushes, Catalog: catalog, Log: log}
	if _, err := n.Send(ctx, now); err != nil {
		t.Fatal(err)
	}
	var crewPush bool
	for _, m := range pushes.sent {
		if m.Data["crewId"] == crew.ID.String() {
			crewPush = true
			if m.Title != "Test crew" || !strings.Contains(m.Body, "crew") {
				t.Errorf("crew push = %q / %q", m.Title, m.Body)
			}
		}
	}
	if !crewPush {
		t.Error("no push about the crew offer")
	}

	if _, err := svc.Send(ctx, a, crew.ID, "hi"); !errors.Is(err, ErrNotMember) {
		t.Errorf("chat before accepting: %v, want ErrNotMember", err)
	}
	for _, u := range ids(a, b, c) {
		if err := svc.Respond(ctx, u, crew.ID, true, now); err != nil {
			t.Fatal(err)
		}
	}
	if it, err := svc.Get(ctx, c, crew.ID); err != nil || it.Status != Active {
		t.Fatalf("after three yeses: %+v, %v; want active", it, err)
	}
	if err := svc.Respond(ctx, a, crew.ID, false, now); !errors.Is(err, ErrOver) {
		t.Errorf("answering after it formed: %v, want ErrOver", err)
	}

	if _, err := svc.Send(ctx, a, crew.ID, " hello crew "); err != nil {
		t.Fatal(err)
	}
	msgs, err := svc.Messages(ctx, b, crew.ID, 0)
	if err != nil || len(msgs) != 1 || msgs[0].Body != "hello crew" || msgs[0].Mine || msgs[0].Name != "Test" {
		t.Errorf("b reads %+v, %v", msgs, err)
	}
	if _, err := svc.Messages(ctx, d, crew.ID, 0); !errors.Is(err, ErrNoCrew) {
		t.Errorf("d reads the chat: %v, want ErrNoCrew", err)
	}
	var inv *plans.InvalidError
	if _, err := svc.Send(ctx, a, crew.ID, "   "); !errors.As(err, &inv) {
		t.Errorf("empty message: %v, want invalid", err)
	}

	// A day after forming, an open plan on its activity is suggested.
	planID, err := plans.Create(ctx, f.Q, f.City, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.Area), Activity: matching.ActivityID(f.Act), Primary: "ja"}, Audience: 6, Score: 9},
		plans.Window{Start: day(14), Days: 14, Deadline: day(25)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep, err := r.Pass(ctx, f.City, now.Add(time.Hour)); err != nil || rep.Suggested != 0 {
		t.Errorf("suggested on the day it formed: %+v, %v", rep, err)
	}
	if rep, err := r.Pass(ctx, f.City, now.Add(25*time.Hour)); err != nil || rep.Suggested != 1 {
		t.Errorf("a day later: %+v, %v; want one suggestion", rep, err)
	}
	msgs, _ = svc.Messages(ctx, b, crew.ID, msgs[0].ID)
	if len(msgs) != 1 || msgs[0].PlanID != planID {
		t.Errorf("suggestion = %+v, want plan %s", msgs, planID)
	}
	if rep, _ := r.Pass(ctx, f.City, now.Add(49*time.Hour)); rep.Suggested != 0 {
		t.Error("suggested again within the week")
	}

	if err := svc.ReportMember(ctx, a, crew.ID, d, plans.ReportOther, "", now); !errors.As(err, &inv) {
		t.Errorf("reporting someone outside the crew: %v, want invalid", err)
	}
	if err := svc.ReportMember(ctx, a, crew.ID, b, plans.ReportHarassment, "rude in the chat", now); err != nil {
		t.Fatal(err)
	}

	// Leaving: the reason is kept; with one member left it closes.
	if err := svc.Leave(ctx, c, crew.ID, "bored", "", now); !errors.As(err, &inv) {
		t.Errorf("unknown reason: %v, want invalid", err)
	}
	if err := svc.Leave(ctx, c, crew.ID, LeaveInactive, "nothing happens", now); err != nil {
		t.Fatal(err)
	}
	if it, err := svc.Get(ctx, a, crew.ID); err != nil || it.Status != Active || len(it.Members) != 2 {
		t.Errorf("after c left: %+v, %v; want active with two", it, err)
	}
	if err := svc.Leave(ctx, b, crew.ID, LeaveUncomfortable, "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, a, crew.ID); !errors.Is(err, ErrNoCrew) {
		t.Errorf("after b left: %v, want closed (not found)", err)
	}
	var leaves int
	if err := f.Pool.QueryRow(ctx, "select count(*) from app.crew_leaves where crew_id = $1", crew.ID).Scan(&leaves); err != nil || leaves != 2 {
		t.Errorf("leave reasons = %d, %v; want 2", leaves, err)
	}
	// Having left a crew with each other, they aren't offered one together again.
	if rep, err := r.Pass(ctx, f.City, now.Add(72*time.Hour)); err != nil || rep.Offered != 0 {
		t.Errorf("after leaving: %+v, %v; want no new offer", rep, err)
	}
}

// TestIntegrationRemoveFromAll takes suspended people out of crews: an
// active one closing once one member is left, an offer declined, and no
// leave reason stored (it wasn't their choice).
func TestIntegrationRemoveFromAll(t *testing.T) {
	f := testfixture.New(t, "zz-crews-suspend", "UTC", 4)
	ctx := context.Background()
	a, b, c, e := f.Users[0], f.Users[1], f.Users[2], f.Users[3]
	svc := &Service{DB: f.Pool}
	at := func(n int) time.Time { return day(n) }

	var active, offer uuid.UUID
	if err := f.Pool.QueryRow(ctx, `insert into app.crews (city_id, activity_id, offer_until, status) values ($1, $2, $3, 'active') returning id`,
		f.City, f.Act, at(1)).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if err := f.Pool.QueryRow(ctx, `insert into app.crews (city_id, activity_id, offer_until) values ($1, $2, $3) returning id`,
		f.City, f.Act, at(20)).Scan(&offer); err != nil {
		t.Fatal(err)
	}
	for _, s := range [][]any{
		{`insert into app.crew_members (crew_id, user_id, response, responded_at) select $1, unnest($2::uuid[]), 'accepted', $3`, active, []uuid.UUID{a, b, c}, at(2)},
		{`insert into app.crew_members (crew_id, user_id) select $1, unnest($2::uuid[])`, offer, []uuid.UUID{a, e}},
	} {
		if _, err := f.Pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
			t.Fatal(err)
		}
	}
	crew := func(id, u uuid.UUID) (status, resp string) {
		t.Helper()
		if err := f.Pool.QueryRow(ctx, `select c.status, m.response from app.crews c join app.crew_members m on m.crew_id = c.id where c.id = $1 and m.user_id = $2`,
			id, u).Scan(&status, &resp); err != nil {
			t.Fatal(err)
		}
		return
	}

	if err := svc.RemoveFromAll(ctx, a, at(8)); err != nil {
		t.Fatal(err)
	}
	if s, r := crew(active, a); s != Active || r != Left {
		t.Errorf("active crew after a: %s, a %s", s, r)
	}
	if s, r := crew(offer, a); s != Offered || r != Declined {
		t.Errorf("offer after a: %s, a %s", s, r)
	}
	var leaves int
	if err := f.Pool.QueryRow(ctx, `select count(*) from app.crew_leaves where crew_id = $1`, active).Scan(&leaves); err != nil || leaves != 0 {
		t.Errorf("leave reasons stored = %d, %v (none: it wasn't their choice)", leaves, err)
	}

	// b: the crew is down to c and closes.
	if err := svc.RemoveFromAll(ctx, b, at(8)); err != nil {
		t.Fatal(err)
	}
	if s, _ := crew(active, c); s != Closed {
		t.Errorf("crew with one left = %s, want closed", s)
	}

	// Again: nothing left to do, no error.
	if err := svc.RemoveFromAll(ctx, a, at(8)); err != nil {
		t.Fatalf("second run: %v", err)
	}
}
