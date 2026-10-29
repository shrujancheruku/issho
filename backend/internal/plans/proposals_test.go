package plans

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

// TestIntegrationProposals: in a city under review the scheduler proposes
// a round instead of inviting anyone; an admin changes it (remove, move,
// start time, meeting point) and approves it, which invites the people
// still signed up and moves the plan on. Another proposal is rejected and
// the plan rests.
func TestIntegrationProposals(t *testing.T) {
	f := newFixture(t, "zz-plans-proposals", "Asia/Tokyo", 8)
	ctx := context.Background()
	for _, id := range f.users {
		f.addProfile(t, id, "ja")
	}
	if _, err := f.pool.Exec(ctx, "update app.cities set review = true where id = $1", f.city); err != nil {
		t.Fatal(err)
	}
	svc := &Service{DB: f.pool}
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC) // Tuesday 18:00 in Tokyo
	h := &ReviewHandler{Svc: svc, Timing: DefaultTiming(), Now: func() time.Time { return now }}
	adminID := uuid.New()
	adminCtx := auth.WithCaller(ctx, auth.Caller{UserID: adminID, Admin: true})

	plan := matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: plan, Audience: 8, Score: 1}, DefaultTiming().Round(now, mustLoc(t, "Asia/Tokyo")), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range f.users {
		if err := SaveSignup(ctx, f.pool, planID, matching.Signup{User: matching.UserID(id.String())}); err != nil {
			t.Fatal(err)
		}
	}

	// The scheduler proposes: nobody invited, told or taken off the list.
	if rep, err := r.Schedule(ctx, f.city, now); err != nil || rep.Scheduled != 1 {
		t.Fatalf("Schedule = %+v, %v", rep, err)
	}
	list := func() []*isshov1.ReviewEvent {
		t.Helper()
		res, err := h.ListReviewEvents(adminCtx, connect.NewRequest(&isshov1.ListReviewEventsRequest{
			CityId: f.city, Status: isshov1.ReviewEventStatus_REVIEW_EVENT_STATUS_PROPOSED,
		}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg.Events
	}
	proposals := list()
	if len(proposals) != 1 || proposals[0].People != 6 || proposals[0].TimeZone != "Asia/Tokyo" {
		t.Fatalf("proposals = %v", proposals)
	}
	eventID := uuid.MustParse(proposals[0].Id)
	count := func(sql string, args ...any) int {
		t.Helper()
		var n int
		if err := f.pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count("select count(*) from app.plan_signups where plan_id = $1", planID); n != 8 {
		t.Errorf("signups while proposed = %d, want 8", n)
	}
	if n := count("select count(*) from app.notifications where event_id = $1", eventID); n != 0 {
		t.Errorf("notifications while proposed = %d", n)
	}
	get := func() *isshov1.GetReviewEventResponse {
		t.Helper()
		res, err := h.GetReviewEvent(adminCtx, connect.NewRequest(&isshov1.GetReviewEventRequest{EventId: eventID.String()}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg
	}
	ev := get()
	if len(ev.Groups) != 1 || len(ev.Groups[0].Members) != 6 {
		t.Fatalf("groups = %v", ev.Groups)
	}
	member := func(i int) uuid.UUID { return uuid.MustParse(ev.Groups[0].Members[i].UserId) }
	first := member(0)
	if items, err := svc.Events(ctx, first, now); err != nil || len(items) != 0 {
		t.Fatalf("a proposed member's events = %d, %v (want none)", len(items), err)
	}
	if err := svc.Respond(ctx, first, eventID, Confirm, now); err == nil {
		t.Fatal("confirming a proposal: want an error")
	}
	// A second pass leaves the plan alone.
	if rep, err := r.Schedule(ctx, f.city, now); err != nil || rep.Scheduled != 0 {
		t.Fatalf("second Schedule = %+v, %v", rep, err)
	}

	// Changes: take one out (still signed up), a move to a group that isn't
	// the event's, a start too soon, then a good one, and a meeting point.
	if _, err := h.RemoveProposalMember(adminCtx, connect.NewRequest(&isshov1.RemoveProposalMemberRequest{EventId: eventID.String(), UserId: member(1).String()})); err != nil {
		t.Fatal(err)
	}
	if _, err := h.MoveProposalMember(adminCtx, connect.NewRequest(&isshov1.MoveProposalMemberRequest{EventId: eventID.String(), UserId: first.String(), GroupId: uuid.NewString()})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("moving to another event's group: %v", err)
	}
	start := func(day, clock string) error {
		_, err := h.SetProposalStart(adminCtx, connect.NewRequest(&isshov1.SetProposalStartRequest{EventId: eventID.String(), Date: day, Time: clock}))
		return err
	}
	if err := start("2030-01-02", "19:00"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("a start tomorrow (answers due today): %v", err)
	}
	if err := start("2030-01-06", "nine"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a bad time: %v", err)
	}
	if err := start("2030-01-06", "10:30"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SetGroupMeetingPoint(adminCtx, connect.NewRequest(&isshov1.SetGroupMeetingPointRequest{GroupId: ev.Groups[0].Id, Text: "North exit, by the clock"})); err != nil {
		t.Fatal(err)
	}
	ev = get()
	tokyo := mustLoc(t, "Asia/Tokyo")
	if e := ev.Event; e.Date != "2030-01-06" || e.Block != "morning" || !e.StartsAt.AsTime().Equal(time.Date(2030, 1, 6, 10, 30, 0, 0, tokyo)) ||
		!e.ConfirmBy.AsTime().Equal(time.Date(2030, 1, 5, 0, 0, 0, 0, tokyo)) || ev.Groups[0].MeetingPoint != "North exit, by the clock" || len(ev.Groups[0].Members) != 5 {
		t.Fatalf("after changes = %v", ev)
	}

	// Someone withdraws before approval: they're left out. The other four
	// are invited and told; the three still waiting carry on in round 2.
	withdrawn := uuid.MustParse(ev.Groups[0].Members[4].UserId)
	if err := Withdraw(ctx, f.q, planID, withdrawn); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ApproveProposal(adminCtx, connect.NewRequest(&isshov1.ApproveProposalRequest{EventId: eventID.String()})); err != nil {
		t.Fatal(err)
	}
	if n := count("select count(*) from app.group_members where event_id = $1", eventID); n != 4 {
		t.Errorf("invited = %d, want 4", n)
	}
	if n := count("select count(*) from app.notifications where event_id = $1 and kind = $2", eventID, NotifyInvited); n != 4 {
		t.Errorf("invitations sent = %d, want 4", n)
	}
	if n := count("select count(*) from app.plan_signups where plan_id = $1", planID); n != 3 {
		t.Errorf("still waiting = %d, want 3", n)
	}
	if n := count("select round from app.plans where id = $1", planID); n != 2 {
		t.Errorf("round = %d, want 2", n)
	}
	if items, err := svc.Events(ctx, first, now); err != nil || len(items) != 1 {
		t.Fatalf("an invited member's events = %d, %v", len(items), err)
	}
	if _, err := h.ApproveProposal(adminCtx, connect.NewRequest(&isshov1.ApproveProposalRequest{EventId: eventID.String()})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("approving twice: %v", err)
	}
	// Scheduled events can still get a meeting point.
	if _, err := h.SetGroupMeetingPoint(adminCtx, connect.NewRequest(&isshov1.SetGroupMeetingPointRequest{GroupId: ev.Groups[0].Id, Text: "South exit"})); err != nil {
		t.Fatalf("meeting point on a scheduled event: %v", err)
	}

	// A proposal for round 2, rejected: deleted, everyone still signed up,
	// and the scheduler leaves the plan alone for a day.
	waiting, err := LoadSignups(ctx, f.q, planID, now)
	if err != nil {
		t.Fatal(err)
	}
	var invite []matching.UserID
	for _, s := range waiting[:2] {
		invite = append(invite, s.User)
	}
	second, _, err := SaveEvent(ctx, f.pool, Event{
		PlanID: planID, Round: 2, Propose: true, Now: now,
		StartsAt: time.Date(2030, 1, 12, 14, 0, 0, 0, tokyo), ConfirmBy: time.Date(2030, 1, 11, 0, 0, 0, 0, tokyo),
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: time.Date(2030, 1, 12, 0, 0, 0, 0, time.UTC), Block: matching.Afternoon},
			Groups:   []matching.Group{{Invited: invite, Quorum: 2, MinAttendance: 2, Score: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.RejectProposal(adminCtx, connect.NewRequest(&isshov1.RejectProposalRequest{EventId: second.String()})); err != nil {
		t.Fatal(err)
	}
	if n := count("select count(*) from app.events where id = $1", second); n != 0 {
		t.Errorf("rejected proposal still there")
	}
	if n := count("select count(*) from app.plan_signups where plan_id = $1", planID); n != 3 {
		t.Errorf("waiting after rejection = %d, want 3", n)
	}
	held := func(at time.Time) bool {
		t.Helper()
		ids, err := f.q.ListPlansOnHold(ctx, dbgen.ListPlansOnHoldParams{CityID: f.city, RejectedSince: pgtype.Timestamptz{Time: at.Add(-ProposalRest), Valid: true}})
		if err != nil {
			t.Fatal(err)
		}
		return len(ids) == 1 && ids[0] == planID
	}
	if !held(now) || held(now.Add(ProposalRest+time.Hour)) {
		t.Errorf("on hold now %v, a day later %v; want true, false", held(now), held(now.Add(ProposalRest+time.Hour)))
	}
}

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}
