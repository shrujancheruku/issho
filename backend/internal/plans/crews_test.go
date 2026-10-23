package plans

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
	"github.com/shrujancheruku/issho/backend/internal/uistrings"
)

// TestIntegrationCrews: A met B and D at one outing, C and D at another,
// and B met C at a third. A–B and B–C picked each other, and so did A–D,
// but C would rather not see D again. A, B and C are offered a crew named
// after the activity they all love; D isn't in it. It forms when all three
// accept, gets a plan suggested a day later, and closes when only one is
// left.
func TestIntegrationCrews(t *testing.T) {
	f := newFixture(t, "zz-plans-crews", "UTC", 4)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	a, b, c, d := f.users[0], f.users[1], f.users[2], f.users[3]
	svc := &Service{DB: f.pool}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: log}

	type answer struct {
		who          uuid.UUID
		picks, avoid []uuid.UUID
	}
	outing := func(people []uuid.UUID, answers ...answer) time.Time {
		eventID, starts := feedbackEvent(t, f, people, len(people))
		for _, an := range answers {
			fb := Feedback{Attended: true, PlanRating: 4, GroupRating: 4, Picks: an.picks, Avoid: an.avoid}
			if err := svc.SubmitFeedback(ctx, an.who, eventID, fb, starts.Add(FeedbackAfter)); err != nil {
				t.Fatal(err)
			}
		}
		return starts
	}
	ids := func(us ...uuid.UUID) []uuid.UUID { return us }
	outing(ids(a, b, d), answer{a, ids(b, d), nil}, answer{b, ids(a), nil}, answer{d, ids(a), nil})
	outing(ids(a, c, d), answer{a, nil, nil}, answer{c, nil, ids(d)}, answer{d, nil, nil})
	starts := outing(ids(b, c), answer{b, ids(c), nil}, answer{c, ids(b), nil})

	// Someone can't be in both lists.
	var inv *InvalidError
	eventID, s2 := feedbackEvent(t, f, ids(a, b), 2)
	if err := svc.SubmitFeedback(ctx, a, eventID, Feedback{Attended: true, PlanRating: 3, GroupRating: 3, Picks: ids(b), Avoid: ids(b)}, s2.Add(FeedbackAfter)); !errors.As(err, &inv) {
		t.Errorf("picked and avoided: %v, want invalid", err)
	}

	now := starts.Add(FeedbackAfter + time.Hour)
	rep, err := r.Crews(ctx, f.city, now)
	if err != nil || rep.Offered != 1 {
		t.Fatalf("crews pass = %+v, %v; want one offered", rep, err)
	}
	crews, err := svc.Crews(ctx, a)
	if err != nil || len(crews) != 1 {
		t.Fatalf("a's crews = %+v, %v", crews, err)
	}
	crew := crews[0]
	var members []uuid.UUID
	for _, m := range crew.Members {
		members = append(members, m.ID)
	}
	want := map[uuid.UUID]bool{a: true, b: true, c: true}
	if crew.Status != CrewOffered || crew.Activity != f.act || len(members) != 3 || !want[members[0]] || !want[members[1]] || !want[members[2]] {
		t.Errorf("crew = %+v; want an offer to a, b and c on %s", crew, f.act)
	}
	if _, err := svc.Crew(ctx, d, crew.ID); !errors.Is(err, ErrNoCrew) {
		t.Errorf("d sees the crew: %v", err)
	}
	if rep, err := r.Crews(ctx, f.city, now.Add(time.Hour)); err != nil || rep.Offered != 0 {
		t.Errorf("second pass = %+v, %v; want no new offer", rep, err)
	}

	// The push is titled with the crew's name.
	if err := f.q.UpsertPushToken(ctx, dbgen.UpsertPushTokenParams{Token: "ExponentPushToken[crew-a]", UserID: a, Platform: "android"}); err != nil {
		t.Fatal(err)
	}
	catalog, err := uistrings.Load()
	if err != nil {
		t.Fatal(err)
	}
	sender := &fakeSender{}
	n := &Notifier{Plans: svc, Sender: sender, Catalog: catalog, Log: log}
	if _, err := n.Send(ctx, now); err != nil {
		t.Fatal(err)
	}
	var crewPush bool
	for _, m := range sender.sent {
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

	if _, err := svc.SendCrewMessage(ctx, a, crew.ID, "hi"); !errors.Is(err, ErrNotInCrew) {
		t.Errorf("chat before accepting: %v, want ErrNotInCrew", err)
	}
	for _, u := range ids(a, b, c) {
		if err := svc.RespondToCrew(ctx, u, crew.ID, true, now); err != nil {
			t.Fatal(err)
		}
	}
	if it, err := svc.Crew(ctx, c, crew.ID); err != nil || it.Status != CrewActive {
		t.Fatalf("after three yeses: %+v, %v; want active", it, err)
	}
	if err := svc.RespondToCrew(ctx, a, crew.ID, false, now); !errors.Is(err, ErrCrewOver) {
		t.Errorf("answering after it formed: %v, want ErrCrewOver", err)
	}

	if _, err := svc.SendCrewMessage(ctx, a, crew.ID, " hello crew "); err != nil {
		t.Fatal(err)
	}
	msgs, err := svc.CrewMessages(ctx, b, crew.ID, 0)
	if err != nil || len(msgs) != 1 || msgs[0].Body != "hello crew" || msgs[0].Mine || msgs[0].Name != "Test" {
		t.Errorf("b reads %+v, %v", msgs, err)
	}
	if _, err := svc.CrewMessages(ctx, d, crew.ID, 0); !errors.Is(err, ErrNoCrew) {
		t.Errorf("d reads the chat: %v, want ErrNoCrew", err)
	}

	// A day after forming, an open plan on its activity is suggested.
	day := func(n int) time.Time { return time.Date(2030, 1, n, 0, 0, 0, 0, time.UTC) }
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}, Audience: 6, Score: 9},
		Window{Start: day(14), Days: 14, Deadline: day(25)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep, err := r.Crews(ctx, f.city, now.Add(time.Hour)); err != nil || rep.Suggested != 0 {
		t.Errorf("suggested on the day it formed: %+v, %v", rep, err)
	}
	if rep, err := r.Crews(ctx, f.city, now.Add(25*time.Hour)); err != nil || rep.Suggested != 1 {
		t.Errorf("a day later: %+v, %v; want one suggestion", rep, err)
	}
	msgs, _ = svc.CrewMessages(ctx, b, crew.ID, msgs[0].ID)
	if len(msgs) != 1 || msgs[0].PlanID != planID {
		t.Errorf("suggestion = %+v, want plan %s", msgs, planID)
	}
	if rep, _ := r.Crews(ctx, f.city, now.Add(49*time.Hour)); rep.Suggested != 0 {
		t.Error("suggested again within the week")
	}

	if err := svc.ReportCrewMember(ctx, a, crew.ID, d, ReportOther, "", now); !errors.As(err, &inv) {
		t.Errorf("reporting someone outside the crew: %v, want invalid", err)
	}
	if err := svc.ReportCrewMember(ctx, a, crew.ID, b, ReportHarassment, "rude in the chat", now); err != nil {
		t.Fatal(err)
	}

	// Leaving: the reason is kept; with one member left it closes.
	if err := svc.LeaveCrew(ctx, c, crew.ID, "bored", "", now); !errors.As(err, &inv) {
		t.Errorf("unknown reason: %v, want invalid", err)
	}
	if err := svc.LeaveCrew(ctx, c, crew.ID, LeaveInactive, "nothing happens", now); err != nil {
		t.Fatal(err)
	}
	if it, err := svc.Crew(ctx, a, crew.ID); err != nil || it.Status != CrewActive || len(it.Members) != 2 {
		t.Errorf("after c left: %+v, %v; want active with two", it, err)
	}
	if err := svc.LeaveCrew(ctx, b, crew.ID, LeaveUncomfortable, "", now); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Crew(ctx, a, crew.ID); !errors.Is(err, ErrNoCrew) {
		t.Errorf("after b left: %v, want closed (not found)", err)
	}
	var leaves int
	if err := f.pool.QueryRow(ctx, "select count(*) from app.crew_leaves where crew_id = $1", crew.ID).Scan(&leaves); err != nil || leaves != 2 {
		t.Errorf("leave reasons = %d, %v; want 2", leaves, err)
	}
	// Having left a crew with each other, they aren't offered one together again.
	if rep, err := r.Crews(ctx, f.city, now.Add(72*time.Hour)); err != nil || rep.Offered != 0 {
		t.Errorf("after leaving: %+v, %v; want no new offer", rep, err)
	}
}
