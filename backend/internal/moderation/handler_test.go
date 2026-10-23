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
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/translate"
)

func TestNonAdminRefused(t *testing.T) {
	// moderator() runs before any query, so no database is needed.
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
	const city, source = "moderation-test", "Moderation test: code sent to {email}"
	hash := translate.Key(source, "test")
	users := []uuid.UUID{reporter, reported}
	var supabase bool
	if err := pool.QueryRow(ctx, "select exists (select 1 from pg_namespace where nspname = 'auth')").Scan(&supabase); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		stmts := [][]any{
			{"delete from app.users where id = any($1)", users},
			{"delete from app.cities where id = $1", city},
			{"delete from app.translations where source_hash = $1", hash},
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
		[]any{"insert into app.translations (source_hash, lang, source, context, text, model) values ($1, 'ja', $2, 'test', '{email} にコードを送信', 'machine')", hash, source},
	)
	for _, s := range setup {
		if _, err := pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
			t.Fatal(err)
		}
	}

	q := dbgen.New(pool)
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	h := &Handler{Q: q, Now: func() time.Time { return now }}
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

	// Translations: a correction must keep {email}; a reviewed one is never
	// overwritten by a machine translation.
	list, err := h.ListTranslations(ctx, connect.NewRequest(&isshov1.ListTranslationsRequest{Lang: "ja", UnreviewedOnly: true, Search: "moderation test"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Msg.Translations) != 1 || list.Msg.Translations[0].SourceHash != hash {
		t.Fatalf("translations = %v", list.Msg.Translations)
	}
	if _, err := h.ReviewTranslation(ctx, connect.NewRequest(&isshov1.ReviewTranslationRequest{SourceHash: hash, Lang: "ja", Text: "コードを送信しました"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("lost placeholder: %v", err)
	}
	res, err := h.ReviewTranslation(ctx, connect.NewRequest(&isshov1.ReviewTranslationRequest{SourceHash: hash, Lang: "ja", Text: "{email} にコードを送信しました"}))
	if err != nil {
		t.Fatal(err)
	}
	if tr := res.Msg.Translation; !tr.Reviewed || tr.Text != "{email} にコードを送信しました" {
		t.Fatalf("reviewed = %v", tr)
	}
	if err := q.InsertTranslation(context.Background(), dbgen.InsertTranslationParams{
		SourceHash: hash, Lang: "ja", Source: source, Context: "test", Text: "machine again", Model: "machine",
	}); err != nil {
		t.Fatal(err)
	}
	if got, _ := q.GetTranslation(context.Background(), dbgen.GetTranslationParams{SourceHash: hash, Lang: "ja"}); got.Text != "{email} にコードを送信しました" {
		t.Fatalf("machine translation overwrote the review: %q", got.Text)
	}
}
