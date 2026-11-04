package plans

import (
	"context"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

// ids runs query (selecting one text column) and returns it in order.
func ids(t *testing.T, ctx context.Context, f *fixture, query string, args ...any) []string {
	t.Helper()
	rows, err := f.pool.Query(ctx, query, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestIntegrationPagingReviewPlans: drafts oldest first, closed plans newest
// first, both with repeating created_at, a page at a time.
func TestIntegrationPagingReviewPlans(t *testing.T) {
	f := newFixture(t, "zz-plans-paging", "Asia/Tokyo", 1)
	ctx := context.Background()
	base := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	insert := func(status, closedReason string, n int) {
		t.Helper()
		// created_at repeats every 3 plans, so the id breaks ties.
		_, err := f.pool.Exec(ctx, `insert into app.plans (city_id, area_id, activity_id, primary_language, audience, score,
				window_start, window_days, signup_deadline, status, closed_reason, closed_at, created_at)
			select $1, $2, $3, 'ja', 1, 1, '2030-01-01'::date, 7, $4::timestamptz, $5::text, nullif($6::text, ''), case when $5::text = 'closed' then $4::timestamptz end, $4::timestamptz + (i % 3) * interval '1 hour'
			from generate_series(1, $7) i`, f.city, f.area, f.act, base, status, closedReason, n)
		if err != nil {
			t.Fatal(err)
		}
	}
	insert("draft", "", 11)
	insert("closed", "done", 11)
	insert("open", "", 3)

	h := &ReviewHandler{Svc: &Service{DB: f.pool}, Now: func() time.Time { return base }}
	admin := auth.WithCaller(ctx, auth.Caller{UserID: uuid.New(), Admin: true})
	list := func(status isshov1.ReviewPlanStatus, size int32, token string) (*isshov1.ListReviewPlansResponse, error) {
		res, err := h.ListReviewPlans(admin, connect.NewRequest(&isshov1.ListReviewPlansRequest{CityId: f.city, Status: status, PageSize: size, PageToken: token}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	walk := func(status isshov1.ReviewPlanStatus, size int32) []string {
		t.Helper()
		var all []string
		token := ""
		for range 100 {
			res, err := list(status, size, token)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range res.Plans {
				all = append(all, p.Id)
			}
			if res.NextPageToken == "" {
				return all
			}
			token = res.NextPageToken
		}
		t.Fatal("paging does not end")
		return nil
	}
	draftOrder := `select id::text from app.plans where city_id = $1 and status = $2 order by created_at asc, id asc`
	otherOrder := `select id::text from app.plans where city_id = $1 and status = $2 order by created_at desc, id desc`
	for _, size := range []int32{4, 0, 1000} {
		if got, want := walk(isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_DRAFT, size), ids(t, ctx, f, draftOrder, f.city, "draft"); !slices.Equal(got, want) {
			t.Errorf("drafts, page_size %d: got %v, want %v", size, got, want)
		}
		if got, want := walk(isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_CLOSED, size), ids(t, ctx, f, otherOrder, f.city, "closed"); !slices.Equal(got, want) {
			t.Errorf("closed, page_size %d: got %v, want %v", size, got, want)
		}
	}
	// A draft token can't page the closed list, nor garbage anything.
	first, err := list(isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_DRAFT, 2, "")
	if err != nil || first.NextPageToken == "" {
		t.Fatalf("first page: %v", err)
	}
	if _, err := list(isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_CLOSED, 2, first.NextPageToken); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("token for another status: %v", err)
	}
	if _, err := list(isshov1.ReviewPlanStatus_REVIEW_PLAN_STATUS_DRAFT, 2, "garbage"); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("garbage token: %v", err)
	}
}

// TestIntegrationPagingReviewEvents: proposals by date, block, id, with the
// same date and block shared by several.
func TestIntegrationPagingReviewEvents(t *testing.T) {
	f := newFixture(t, "zz-plans-paging-ev", "Asia/Tokyo", 1)
	ctx := context.Background()
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	// One plan per event (unique per plan and round).
	_, err := f.pool.Exec(ctx, `with p as (
			insert into app.plans (city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline)
			select $1, $2, $3, 'ja', 1, 1, '2030-01-01'::date, 7, $4 from generate_series(1, 11) i
			returning id)
		insert into app.events (plan_id, round, date, block, confirm_by, forced, status)
		select p.id, 1, '2030-01-05'::date + (n % 3)::int, (array['morning', 'afternoon', 'evening'])[1 + n % 2],
		       $4, false, 'proposed'
		from (select id, row_number() over (order by id) n from p) p`, f.city, f.area, f.act, now)
	if err != nil {
		t.Fatal(err)
	}

	h := &ReviewHandler{Svc: &Service{DB: f.pool}, Now: func() time.Time { return now }}
	admin := auth.WithCaller(ctx, auth.Caller{UserID: uuid.New(), Admin: true})
	status := isshov1.ReviewEventStatus_REVIEW_EVENT_STATUS_PROPOSED
	var got []string
	token := ""
	for range 100 {
		res, err := h.ListReviewEvents(admin, connect.NewRequest(&isshov1.ListReviewEventsRequest{CityId: f.city, Status: status, PageSize: 4, PageToken: token}))
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range res.Msg.Events {
			got = append(got, e.Id)
		}
		if token = res.Msg.NextPageToken; token == "" {
			break
		}
	}
	want := ids(t, ctx, f, `select e.id::text from app.events e join app.plans p on p.id = e.plan_id
		where p.city_id = $1 and e.status = 'proposed' order by e.date, e.block, e.id`, f.city)
	if len(want) != 11 || !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if _, err := h.ListReviewEvents(admin, connect.NewRequest(&isshov1.ListReviewEventsRequest{CityId: f.city, Status: status, PageToken: "garbage"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("garbage token: %v", err)
	}
	// The same token for scheduled events (another status) is refused.
	first, err := h.ListReviewEvents(admin, connect.NewRequest(&isshov1.ListReviewEventsRequest{CityId: f.city, Status: status, PageSize: 2}))
	if err != nil || first.Msg.NextPageToken == "" {
		t.Fatalf("first page: %v", err)
	}
	if _, err := h.ListReviewEvents(admin, connect.NewRequest(&isshov1.ListReviewEventsRequest{
		CityId: f.city, Status: isshov1.ReviewEventStatus_REVIEW_EVENT_STATUS_SCHEDULED, PageToken: first.Msg.NextPageToken,
	})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("token for another status: %v", err)
	}
}
