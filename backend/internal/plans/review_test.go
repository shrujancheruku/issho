package plans

import (
	"context"
	"io"
	"log/slog"
	"strings"
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

// TestIntegrationReview: in a city under review the matcher's plan waits
// as a draft (unseen, but counted so it isn't repeated); an admin approves
// it with a fresh window, rejects another (which then rests like a failed
// plan), edits, withdraws, and creates a plan by hand.
func TestIntegrationReview(t *testing.T) {
	f := newFixture(t, "zz-plans-review", "Asia/Tokyo", 8)
	ctx := context.Background()
	for _, id := range f.users {
		f.addProfile(t, id, "ja")
	}
	svc := &Service{DB: f.pool}
	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC) // Tuesday 18:00 in Tokyo
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	adminID := uuid.New()
	h := &ReviewHandler{Svc: svc, Timing: DefaultTiming(), Now: func() time.Time { return now }}
	adminCtx := auth.WithCaller(ctx, auth.Caller{UserID: adminID, Admin: true})

	if _, err := h.SetCityReview(adminCtx, connect.NewRequest(&isshov1.SetCityReviewRequest{CityId: f.city, Review: true})); err != nil {
		t.Fatal(err)
	}
	if _, err := h.SetCityReview(auth.WithCaller(ctx, auth.Caller{UserID: f.users[0]}), connect.NewRequest(&isshov1.SetCityReviewRequest{CityId: f.city})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("a user setting review: %v", err)
	}

	// The matcher's plan is a draft: not open, not in anyone's feed, and
	// counted under the city's cap, so a second pass adds nothing.
	if opened, err := r.Suggest(ctx, f.city, now); err != nil || opened != 1 {
		t.Fatalf("Suggest = %d, %v", opened, err)
	}
	if open, err := ListOpen(ctx, f.q, f.city); err != nil || len(open) != 0 {
		t.Fatalf("open plans in review = %+v, %v", open, err)
	}
	if feed, err := svc.Feed(ctx, f.users[0]); err != nil || len(feed) != 0 {
		t.Fatalf("feed with only a draft = %d plans, %v", len(feed), err)
	}
	if opened, err := r.Suggest(ctx, f.city, now); err != nil || opened != 0 {
		t.Fatalf("second Suggest = %d, %v (the draft should count)", opened, err)
	}
	list := func(status isshov1.ReviewPlanStatus) []*isshov1.ReviewPlan {
		t.Helper()
		res, err := h.ListReviewPlans(adminCtx, connect.NewRequest(&isshov1.ListReviewPlansRequest{CityId: f.city, Status: status}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg.Plans
	}
	drafts := list(isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_DRAFT)
	if len(drafts) != 1 || drafts[0].Title != "Test" || drafts[0].StoredTitle != "" || drafts[0].Audience == 0 {
		t.Fatalf("drafts = %v", drafts)
	}
	draft := drafts[0].Id

	// Edit, approve two days later: open, in the feed, with a window from then.
	if _, err := h.EditPlan(adminCtx, connect.NewRequest(&isshov1.EditPlanRequest{PlanId: draft, Title: strings.Repeat("x", 81)})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("an 81-character title: %v", err)
	}
	if _, err := h.EditPlan(adminCtx, connect.NewRequest(&isshov1.EditPlanRequest{PlanId: draft, Title: "Kichijoji board games", Summary: "Games and tea."})); err != nil {
		t.Fatal(err)
	}
	now = now.Add(48 * time.Hour)
	if _, err := h.ApprovePlan(adminCtx, connect.NewRequest(&isshov1.ApprovePlanRequest{PlanId: draft})); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ApprovePlan(adminCtx, connect.NewRequest(&isshov1.ApprovePlanRequest{PlanId: draft})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("approving twice: %v", err)
	}
	open := list(isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_OPEN)
	// Thursday 3 January 18:00 in Tokyo: the window starts two days later.
	if len(open) != 1 || open[0].Title != "Kichijoji board games" || open[0].Summary != "Games and tea." ||
		open[0].WindowStart != "2030-01-05" || open[0].ReviewedBy != adminID.String() {
		t.Fatalf("approved = %v", open)
	}
	feed, err := svc.Feed(ctx, f.users[0])
	if err != nil || len(feed) != 1 {
		t.Fatalf("feed after approval = %d, %v", len(feed), err)
	}
	if feed[0].Title != "Kichijoji board games" {
		t.Errorf("feed title = %q", feed[0].Title)
	}
	item, _, _, err := svc.Plan(ctx, f.users[0], uuid.MustParse(draft), time.Now())
	if err != nil || item.Summary != "Games and tea." {
		t.Errorf("plan summary = %q, %v", item.Summary, err)
	}

	// Another draft, rejected: it rests like a plan that found no group.
	other, err := create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "en"}},
		Window{Start: now, Days: 14, Deadline: now.Add(7 * 24 * time.Hour)}, nil, PlanDraft)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.RejectPlan(adminCtx, connect.NewRequest(&isshov1.RejectPlanRequest{PlanId: other.String()})); err != nil {
		t.Fatal(err)
	}
	failed, err := f.q.ListRecentlyFailedPlans(ctx, dbgen.ListRecentlyFailedPlansParams{CityID: f.city, Reasons: failedReasons, Since: pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true}})
	if err != nil || len(failed) != 1 || failed[0].PrimaryLanguage != "en" {
		t.Fatalf("recently failed after a rejection = %+v, %v", failed, err)
	}
	if closed := list(isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_CLOSED); len(closed) != 1 || closed[0].ClosedReason != ClosedRejected {
		t.Fatalf("closed = %v", closed)
	}

	// A plan made by hand opens at once, even under review.
	create := func(m *isshov1.CreatePlanRequest) (*connect.Response[isshov1.CreatePlanResponse], error) {
		return h.CreatePlan(adminCtx, connect.NewRequest(m))
	}
	if _, err := create(&isshov1.CreatePlanRequest{CityId: f.city, AreaId: "nowhere", ActivityId: f.act, PrimaryLanguage: "ja"}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("unknown area: %v", err)
	}
	if _, err := create(&isshov1.CreatePlanRequest{CityId: f.city, AreaId: f.area, ActivityId: f.act, PrimaryLanguage: "ja", VenuePlaceId: "ChIJmadeup"}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("a venue not found there: %v", err)
	}
	if _, err := create(&isshov1.CreatePlanRequest{CityId: f.city, AreaId: f.area, ActivityId: f.act, PrimaryLanguage: "ja", SecondaryLanguage: "ja"}); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("the same language twice: %v", err)
	}
	made, err := create(&isshov1.CreatePlanRequest{CityId: f.city, AreaId: f.area, ActivityId: f.act, PrimaryLanguage: "ja", SecondaryLanguage: "en", Title: "Launch night"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := h.GetReviewPlan(adminCtx, connect.NewRequest(&isshov1.GetReviewPlanRequest{PlanId: made.Msg.PlanId}))
	if err != nil {
		t.Fatal(err)
	}
	if p := got.Msg.Plan; p.Status != isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_OPEN || p.CreatedBy != adminID.String() || p.Title != "Launch night" || p.SecondaryLanguage != "en" {
		t.Fatalf("made by hand = %v", p)
	}

	// Withdrawing closes an open plan, once.
	withdraw := &isshov1.WithdrawPlanRequest{PlanId: made.Msg.PlanId}
	if _, err := h.WithdrawPlan(adminCtx, connect.NewRequest(withdraw)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.WithdrawPlan(adminCtx, connect.NewRequest(withdraw)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("withdrawing twice: %v", err)
	}
	cities, err := h.ListReviewCities(adminCtx, connect.NewRequest(&isshov1.ListReviewCitiesRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cities.Msg.Cities {
		if c.Id == f.city && (!c.Review || c.Drafts != 0 || c.Open != 1) {
			t.Errorf("city = %v, want review on, 0 drafts, 1 open", c)
		}
	}
}
