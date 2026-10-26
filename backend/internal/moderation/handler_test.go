package moderation

import (
	"context"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/crews"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

func TestNonAdminRefused(t *testing.T) {
	// auth.RequireAdmin runs before any query, so no database is needed.
	h := &Handler{}
	ctx := auth.WithCaller(context.Background(), auth.Caller{UserID: uuid.New()})
	if _, err := h.ListReports(ctx, connect.NewRequest(&isshov1.ListReportsRequest{})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("ListReports as a user: %v", err)
	}
	if _, err := h.GetModerator(context.Background(), connect.NewRequest(&isshov1.GetModeratorRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("GetModerator without a caller: %v", err)
	}
}

// TestIntegrationQueue walks a report, a flag, a crew leave and a
// translation through the queue. Skipped unless TEST_DATABASE_URL is set.
func TestIntegrationQueue(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	// Stable ids, so a rerun deletes an interrupted run's rows first.
	reporter := uuid.NewSHA1(uuid.NameSpaceURL, []byte("issho-test:moderation/reporter"))
	reported := uuid.NewSHA1(uuid.NameSpaceURL, []byte("issho-test:moderation/reported"))
	adminID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("issho-test:moderation/admin"))
	const city = "moderation-test"
	users := []uuid.UUID{reporter, reported}
	var supabase bool
	if err := pool.QueryRow(ctx, "select exists (select 1 from pg_namespace where nspname = 'auth')").Scan(&supabase); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		stmts := [][]any{
			{"delete from app.users where id = any($1)", users},
			{"delete from app.cities where id = $1", city},
		}
		if supabase {
			stmts = append(stmts, []any{"delete from auth.users where id = any($1)", users})
		}
		for _, s := range stmts {
			if _, err := pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	var setup [][]any
	for _, id := range users {
		if supabase {
			setup = append(setup, []any{"insert into auth.users (id, email) values ($1, $2)", id, id.String() + "@example.com"})
		}
		setup = append(setup, []any{"insert into app.users (id) values ($1)", id})
	}
	setup = append(setup,
		[]any{`insert into app.cities (id, name, timezone, currency, languages, travel_modes)
		       values ($1, '{"en":"Test"}', 'UTC', 'USD', '{en}', '{walk}')`, city},
		[]any{`insert into app.areas (id, city_id, name, centroid) values ($1, $1, '{"en":"Testville"}', 'POINT(0 0)')`, city},
		[]any{"insert into app.reports (reporter_id, reported_id, reason, details) values ($1, $2, 'dating', 'Asked me out')", reporter, reported},
		[]any{`insert into app.moderation_flags (user_id, kind, details) values ($1, 'pick_gender_lean', '{"gender":"woman"}')`, reporter},
		[]any{`with c as (insert into app.crews (city_id, area_id, offer_until) values ($1, $1, now()) returning id)
		       insert into app.crew_leaves (crew_id, user_id, reason, details) select id, $2, 'uncomfortable', 'Too pushy' from c`, city, reporter},
		// The reported person has a profile in the city (so matching sees
		// them) and a phone that gets notifications.
		[]any{`insert into app.profiles (user_id, display_name, birth_year, gender, home_area_id, max_travel_min, energy, budget, size_pref, primary_language, conduct_version)
		       values ($1, 'Modtest Ben', 1995, 'man', $2, 30, 1, 1, 'small', 'en', 1)`, reported, city},
		[]any{"insert into app.push_tokens (token, user_id, platform) values ('ExponentPushToken[moderation-test]', $1, 'android')", reported},
	)
	for _, s := range setup {
		if _, err := pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
			t.Fatal(err)
		}
	}

	q := dbgen.New(pool)
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	suspensions := &Suspensions{Q: q}
	h := &Handler{Q: q, Plans: &plans.Service{DB: pool}, Crews: &crews.Service{DB: pool}, Suspensions: suspensions, Now: func() time.Time { return now }}
	ctx = auth.WithCaller(ctx, auth.Caller{UserID: adminID, Email: "admin@example.com", Admin: true})

	// Reports: open, then dismissed with a note.
	reports := func(status isshov1.ReportStatus) []*isshov1.ModerationReport {
		t.Helper()
		res, err := h.ListReports(ctx, connect.NewRequest(&isshov1.ListReportsRequest{Status: status, ReportedId: reported.String()}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg.Reports
	}
	open := reports(isshov1.ReportStatus_REPORT_STATUS_OPEN)
	if len(open) != 1 || open[0].Reason != isshov1.ReportReason_REPORT_REASON_DATING || open[0].ReportsAboutReported != 1 || open[0].ReporterId != reporter.String() {
		t.Fatalf("open reports = %v", open)
	}
	if _, err := h.ResolveReport(ctx, connect.NewRequest(&isshov1.ResolveReportRequest{
		ReportId: open[0].Id, Status: isshov1.ReportStatus_REPORT_STATUS_DISMISSED, Note: " Spoke to both ",
	})); err != nil {
		t.Fatal(err)
	}
	if got := reports(isshov1.ReportStatus_REPORT_STATUS_OPEN); len(got) != 0 {
		t.Fatalf("still open: %v", got)
	}
	done := reports(isshov1.ReportStatus_REPORT_STATUS_DISMISSED)
	if len(done) != 1 || done[0].Note != "Spoke to both" || done[0].ReviewedBy != adminID.String() || !done[0].ReviewedAt.AsTime().Equal(now) {
		t.Fatalf("dismissed reports = %v", done)
	}
	if _, err := h.ResolveReport(ctx, connect.NewRequest(&isshov1.ResolveReportRequest{
		ReportId: uuid.NewString(), Status: isshov1.ReportStatus_REPORT_STATUS_ACTIONED,
	})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("unknown report: %v", err)
	}

	// Flags: resolved once; resolving again is refused.
	flags, err := h.ListFlags(ctx, connect.NewRequest(&isshov1.ListFlagsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	var flagID int64
	for _, f := range flags.Msg.Flags {
		if f.UserId == reporter.String() {
			flagID = f.Id
			if f.Kind != isshov1.FlagKind_FLAG_KIND_PICK_GENDER_LEAN || f.DetailsJson != `{"gender": "woman"}` {
				t.Fatalf("flag = %v", f)
			}
		}
	}
	if flagID == 0 {
		t.Fatal("open flag not listed")
	}
	resolve := &isshov1.ResolveFlagRequest{FlagId: flagID, Note: "Picks were fine"}
	if _, err := h.ResolveFlag(ctx, connect.NewRequest(resolve)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ResolveFlag(ctx, connect.NewRequest(resolve)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("resolving twice: %v", err)
	}

	// Crew leaves, filtered by reason, named after the crew's area.
	leaves, err := h.ListCrewLeaves(ctx, connect.NewRequest(&isshov1.ListCrewLeavesRequest{Reason: isshov1.LeaveReason_LEAVE_REASON_UNCOMFORTABLE}))
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, l := range leaves.Msg.Leaves {
		if l.UserId == reporter.String() {
			found = l.CrewName == "Testville" && l.Details == "Too pushy" && l.Reason == isshov1.LeaveReason_LEAVE_REASON_UNCOMFORTABLE
		}
	}
	if !found {
		t.Fatalf("crew leave not listed: %v", leaves.Msg.Leaves)
	}

	// People: found by name, then their view; suspended (not themselves),
	// which the API's check, GetMe's query, matching and push all see;
	// then lifted.
	people, err := h.SearchPeople(ctx, connect.NewRequest(&isshov1.SearchPeopleRequest{Query: "modtest b"}))
	if err != nil || len(people.Msg.People) != 1 || people.Msg.People[0].UserId != reported.String() || people.Msg.People[0].Suspended {
		t.Fatalf("SearchPeople = %v, %v", people.Msg.GetPeople(), err)
	}
	if _, err := h.SearchPeople(ctx, connect.NewRequest(&isshov1.SearchPeopleRequest{Query: "b"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("one-letter search: %v", err)
	}
	person := func() *isshov1.Person {
		t.Helper()
		res, err := h.GetPerson(ctx, connect.NewRequest(&isshov1.GetPersonRequest{UserId: reported.String()}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg.Person
	}
	if p := person(); p.Name != "Modtest Ben" || p.HomeArea != "Testville" || p.ReportsAbout != 1 || p.OpenReportsAbout != 0 || p.Suspended {
		t.Fatalf("person = %v", p)
	}
	if _, err := h.SuspendPerson(ctx, connect.NewRequest(&isshov1.SuspendPersonRequest{UserId: adminID.String(), Reason: "x"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("suspending yourself: %v", err)
	}
	if _, err := h.SuspendPerson(ctx, connect.NewRequest(&isshov1.SuspendPersonRequest{UserId: reported.String(), Reason: " "})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("no reason: %v", err)
	}
	if _, err := h.LiftSuspension(ctx, connect.NewRequest(&isshov1.LiftSuspensionRequest{UserId: reported.String()})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("lifting before suspending: %v", err)
	}
	isSuspended := func() bool {
		t.Helper()
		got, err := suspensions.Suspended(ctx, reported)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	isSuspended() // load the cache: the suspension must invalidate it
	suspend := &isshov1.SuspendPersonRequest{UserId: reported.String(), Reason: "Repeated dating reports"}
	for range 2 { // again: no error, still one suspension
		if _, err := h.SuspendPerson(ctx, connect.NewRequest(suspend)); err != nil {
			t.Fatal(err)
		}
	}
	if p := person(); !p.Suspended || len(p.Suspensions) != 1 || p.Suspensions[0].Reason != "Repeated dating reports" || p.Suspensions[0].CreatedBy != adminID.String() {
		t.Fatalf("suspended person = %v", p)
	}
	if !isSuspended() {
		t.Error("the API's check doesn't see the suspension")
	}
	if got, err := q.IsSuspended(ctx, reported); err != nil || !got {
		t.Errorf("IsSuspended (GetMe) = %v, %v", got, err)
	}
	inMatching := func() bool {
		t.Helper()
		users, err := q.ListMatchingUsers(ctx, city)
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range users {
			if u.UserID == reported {
				return true
			}
		}
		return false
	}
	tokens := func() int {
		t.Helper()
		got, err := q.ListPushTokens(ctx, []uuid.UUID{reported})
		if err != nil {
			t.Fatal(err)
		}
		return len(got)
	}
	if inMatching() || tokens() != 0 {
		t.Errorf("while suspended: in matching %v, push tokens %d", inMatching(), tokens())
	}
	listed, err := h.ListSuspended(ctx, connect.NewRequest(&isshov1.ListSuspendedRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	var onList bool
	for _, p := range listed.Msg.People {
		onList = onList || (p.UserId == reported.String() && p.Name == "Modtest Ben")
	}
	if !onList {
		t.Errorf("not in ListSuspended: %v", listed.Msg.People)
	}

	if _, err := h.LiftSuspension(ctx, connect.NewRequest(&isshov1.LiftSuspensionRequest{UserId: reported.String(), Note: "Appeal accepted"})); err != nil {
		t.Fatal(err)
	}
	if p := person(); p.Suspended || p.Suspensions[0].LiftNote != "Appeal accepted" || p.Suspensions[0].LiftedAt == nil {
		t.Fatalf("after lifting = %v", p)
	}
	if isSuspended() || !inMatching() || tokens() != 1 {
		t.Errorf("after lifting: check %v, in matching %v, push tokens %d", isSuspended(), inMatching(), tokens())
	}
}
