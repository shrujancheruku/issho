package moderation

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// pagingEnv opens the test database, makes n users with stable ids (a rerun
// deletes an interrupted run's rows first) and returns an admin's context
// and a handler. Skipped unless TEST_DATABASE_URL is set.
func pagingEnv(t *testing.T, name string, n int) (context.Context, *pgxpool.Pool, *Handler, []uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool := testfixture.Pool(t)
	var supabase bool
	if err := pool.QueryRow(ctx, "select exists (select 1 from pg_namespace where nspname = 'auth')").Scan(&supabase); err != nil {
		t.Fatal(err)
	}
	users := make([]uuid.UUID, n)
	for i := range users {
		users[i] = uuid.NewSHA1(uuid.NameSpaceURL, fmt.Appendf(nil, "issho-test:paging/%s/%d", name, i))
	}
	city := "paging-" + name
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
	for i, id := range users {
		email := fmt.Sprintf("paging-%s-%d@example.com", name, i)
		if supabase {
			if _, err := pool.Exec(ctx, "insert into auth.users (id, email) values ($1, $2)", id, email); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := pool.Exec(ctx, "insert into app.users (id, email) values ($1, $2)", id, email); err != nil {
			t.Fatal(err)
		}
	}
	h := &Handler{Q: dbgen.New(pool)}
	ctx = auth.WithCaller(ctx, auth.Caller{UserID: uuid.New(), Email: "admin@example.com", Admin: true})
	return ctx, pool, h, users
}

func exec(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}

// walk calls list with each page's token until the last, with the given
// page size, and returns every id in the order sent. It fails if a page is
// longer than size.
func walk(t *testing.T, size int32, list func(size int32, token string) (ids []string, next string, err error)) []string {
	t.Helper()
	var all []string
	token := ""
	for pages := 0; ; pages++ {
		if pages > 1000 {
			t.Fatal("paging does not end")
		}
		ids, next, err := list(size, token)
		if err != nil {
			t.Fatal(err)
		}
		if size > 0 && int32(len(ids)) > size {
			t.Fatalf("page of %d with page_size %d", len(ids), size)
		}
		all = append(all, ids...)
		if next == "" {
			return all
		}
		token = next
	}
}

func wantInvalid(t *testing.T, what string, err error) {
	t.Helper()
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("%s: %v, want InvalidArgument", what, err)
	}
}

// TestIntegrationPagingReports: 250 open reports, many with the same
// created_at, come back exactly once each in order; open ones oldest first,
// then the dismissed ones newest first.
func TestIntegrationPagingReports(t *testing.T) {
	ctx, pool, h, users := pagingEnv(t, "reports", 2)
	reported, other := users[0], users[1]
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	// created_at repeats every 50 reports, so the id has to break ties.
	exec(t, ctx, pool, `insert into app.reports (reported_id, reason, created_at, updated_at)
		select $1, 'other', $2::timestamptz + (i % 50) * interval '1 minute', $2 from generate_series(1, 250) i`, reported, base)
	exec(t, ctx, pool, `insert into app.reports (reported_id, reason, status, created_at, updated_at)
		select $1, 'other', 'dismissed', $2::timestamptz + (i % 4) * interval '1 hour', $2 from generate_series(1, 9) i`, reported, base)
	// Another person's reports must not turn up.
	exec(t, ctx, pool, `insert into app.reports (reported_id, reason) values ($1, 'other')`, other)

	var want []string
	rows, err := pool.Query(ctx, `select id::text from app.reports where reported_id = $1
		order by (status <> 'open'), case when status = 'open' then created_at end asc,
		         case when status = 'open' then id end asc, created_at desc, id desc`, reported)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		want = append(want, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(want) != 259 {
		t.Fatalf("setup: %d reports", len(want))
	}

	list := func(status isshov1.ReportStatus) func(int32, string) ([]string, string, error) {
		return func(size int32, token string) ([]string, string, error) {
			res, err := h.ListReports(ctx, &isshov1.ListReportsRequest{
				ReportedId: reported.String(), Status: status, PageSize: size, PageToken: token,
			})
			if err != nil {
				return nil, "", err
			}
			var ids []string
			for _, r := range res.Reports {
				ids = append(ids, r.Id)
			}
			return ids, res.NextPageToken, nil
		}
	}
	unspecified := isshov1.ReportStatus_REPORT_STATUS_UNSPECIFIED
	for _, size := range []int32{0, 1000, 7} {
		if got := walk(t, size, list(unspecified)); !slices.Equal(got, want) {
			t.Errorf("page_size %d: %d reports, not the %d in order", size, len(got), len(want))
		}
	}
	if got := walk(t, 0, list(isshov1.ReportStatus_REPORT_STATUS_OPEN)); !slices.Equal(got, want[:250]) {
		t.Errorf("open only: %d reports", len(got))
	}
	if got := walk(t, 4, list(isshov1.ReportStatus_REPORT_STATUS_DISMISSED)); !slices.Equal(got, want[250:]) {
		t.Errorf("dismissed only: %d reports", len(got))
	}

	// page_size 0 is the default 50 and 1000 is clamped to 200.
	first, next, err := list(unspecified)(0, "")
	if err != nil || len(first) != 50 || next == "" {
		t.Errorf("default page: %d reports, next %q, %v", len(first), next, err)
	}
	first, next, err = list(unspecified)(1000, "")
	if err != nil || len(first) != 200 || next == "" {
		t.Errorf("clamped page: %d reports, next %q, %v", len(first), next, err)
	}

	// A bad or stale token is refused; so is one used with other filters or
	// on another list.
	_, _, err = list(unspecified)(10, "not-a-token")
	wantInvalid(t, "garbage token", err)
	_, _, err = list(isshov1.ReportStatus_REPORT_STATUS_OPEN)(10, next)
	wantInvalid(t, "token with another status", err)
	_, err = h.ListFlags(ctx, &isshov1.ListFlagsRequest{PageToken: next})
	wantInvalid(t, "reports token on flags", err)
	_, _, err = list(unspecified)(-1, "")
	wantInvalid(t, "negative page_size", err)

	// A non-admin gets nothing, token or not.
	user := auth.WithCaller(context.Background(), auth.Caller{UserID: uuid.New()})
	if _, err := h.ListReports(user, &isshov1.ListReportsRequest{PageToken: next}); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("non-admin with a token: %v", err)
	}
}

// TestIntegrationPagingFlags: open flags oldest first, resolved newest
// first, with timestamps that repeat.
func TestIntegrationPagingFlags(t *testing.T) {
	ctx, pool, h, users := pagingEnv(t, "flags", 7)
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for i, u := range users {
		// One open flag each (a person has one open flag per kind), created
		// in pairs at the same time; then two resolved flags each, also tied.
		exec(t, ctx, pool, `insert into app.moderation_flags (user_id, kind, details, created_at) values ($1, 'pick_gender_lean', '{}', $2)`,
			u, base.Add(time.Duration(i/2)*time.Hour))
		for j := range 2 {
			exec(t, ctx, pool, `insert into app.moderation_flags (user_id, kind, details, created_at, resolved_at, resolved_by)
				values ($1, 'pick_gender_lean', '{}', $2, $3, $4)`, u, base, base.Add(time.Duration((i+j)/3)*time.Hour), u)
		}
	}
	for _, resolved := range []bool{false, true} {
		var want []string
		// f.id, not id: a bare id in ORDER BY would be the text output column.
		order := "f.created_at asc, f.id asc"
		if resolved {
			order = "f.resolved_at desc, f.id desc"
		}
		rows, err := pool.Query(ctx, fmt.Sprintf(`select f.id::text from app.moderation_flags f
			where f.user_id = any($1) and (f.resolved_at is not null) = $2 order by %s`, order), users, resolved)
		if err != nil {
			t.Fatal(err)
		}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			want = append(want, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		got := walk(t, 3, func(size int32, token string) ([]string, string, error) {
			res, err := h.ListFlags(ctx, &isshov1.ListFlagsRequest{Resolved: resolved, PageSize: size, PageToken: token})
			if err != nil {
				return nil, "", err
			}
			var ids []string
			for _, f := range res.Flags {
				if slices.Contains(users, uuid.MustParse(f.UserId)) {
					ids = append(ids, fmt.Sprint(f.Id))
				}
			}
			return ids, res.NextPageToken, nil
		})
		// Other rows in the database may share the list, so compare ours in order.
		if !slices.Equal(got, want) {
			t.Errorf("resolved=%v: got %v, want %v", resolved, got, want)
		}
	}
	// An open-flags token doesn't work for resolved flags.
	res, err := h.ListFlags(ctx, &isshov1.ListFlagsRequest{PageSize: 1})
	if err != nil || res.NextPageToken == "" {
		t.Fatalf("first page: %v", err)
	}
	_, err = h.ListFlags(ctx, &isshov1.ListFlagsRequest{Resolved: true, PageToken: res.NextPageToken})
	wantInvalid(t, "open token on resolved flags", err)
}

// TestIntegrationPagingCrewLeaves: newest first with repeating timestamps,
// and the reason filter.
func TestIntegrationPagingCrewLeaves(t *testing.T) {
	ctx, pool, h, users := pagingEnv(t, "leaves", 1)
	const city = "paging-leaves"
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	exec(t, ctx, pool, `insert into app.cities (id, name, timezone, currency, languages, travel_modes)
		values ($1, '{"en":"Test"}', 'UTC', 'USD', '{en}', '{walk}')`, city)
	exec(t, ctx, pool, `insert into app.areas (id, city_id, name, centroid) values ($1, $1, '{"en":"Testville"}', 'POINT(0 0)')`, city)
	exec(t, ctx, pool, `with c as (insert into app.crews (city_id, area_id, offer_until) values ($1, $1, now()) returning id)
		insert into app.crew_leaves (crew_id, user_id, reason, created_at)
		select c.id, $2, case when i % 3 = 0 then 'inactive' else 'other' end, $3::timestamptz + (i % 4) * interval '1 hour'
		from c, generate_series(1, 23) i`, city, users[0], base)
	var want, wantInactive []string
	rows, err := pool.Query(ctx, `select l.id::text, l.reason from app.crew_leaves l join app.crews c on c.id = l.crew_id
		where c.city_id = $1 order by l.created_at desc, l.id desc`, city)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, reason string
		if err := rows.Scan(&id, &reason); err != nil {
			t.Fatal(err)
		}
		want = append(want, id)
		if reason == "inactive" {
			wantInactive = append(wantInactive, id)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	list := func(reason isshov1.LeaveReason) func(int32, string) ([]string, string, error) {
		return func(size int32, token string) ([]string, string, error) {
			res, err := h.ListCrewLeaves(ctx, &isshov1.ListCrewLeavesRequest{Reason: reason, PageSize: size, PageToken: token})
			if err != nil {
				return nil, "", err
			}
			var ids []string
			for _, l := range res.Leaves {
				if l.CrewId != "" && l.UserId == users[0].String() {
					ids = append(ids, fmt.Sprint(l.Id))
				}
			}
			return ids, res.NextPageToken, nil
		}
	}
	if got := walk(t, 5, list(isshov1.LeaveReason_LEAVE_REASON_UNSPECIFIED)); !slices.Equal(got, want) {
		t.Errorf("all: got %v, want %v", got, want)
	}
	if got := walk(t, 2, list(isshov1.LeaveReason_LEAVE_REASON_INACTIVE)); !slices.Equal(got, wantInactive) {
		t.Errorf("inactive: got %v, want %v", got, wantInactive)
	}
	_, next, err := list(isshov1.LeaveReason_LEAVE_REASON_UNSPECIFIED)(1, "")
	if err != nil || next == "" {
		t.Fatal(err)
	}
	_, _, err = list(isshov1.LeaveReason_LEAVE_REASON_INACTIVE)(1, next)
	wantInvalid(t, "token with another reason", err)
}

// TestIntegrationPagingSuspended: most recent first, ties by user id.
func TestIntegrationPagingSuspended(t *testing.T) {
	ctx, pool, h, users := pagingEnv(t, "suspended", 7)
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	for i, u := range users {
		exec(t, ctx, pool, `insert into app.suspensions (user_id, reason, created_by, created_at) values ($1, 'test', $1, $2)`,
			u, base.Add(time.Duration(i/3)*time.Hour))
	}
	want := slices.Clone(users)
	slices.SortFunc(want, func(a, b uuid.UUID) int {
		ta, tb := slices.Index(users, a)/3, slices.Index(users, b)/3
		if ta != tb {
			return tb - ta // later first
		}
		return -slices.Compare(a[:], b[:]) // then higher user id first
	})
	got := walk(t, 3, func(size int32, token string) ([]string, string, error) {
		res, err := h.ListSuspended(ctx, &isshov1.ListSuspendedRequest{PageSize: size, PageToken: token})
		if err != nil {
			return nil, "", err
		}
		var ids []string
		for _, p := range res.People {
			if slices.Contains(users, uuid.MustParse(p.UserId)) {
				ids = append(ids, p.UserId)
			}
		}
		return ids, res.NextPageToken, nil
	})
	var wantIDs []string
	for _, u := range want {
		wantIDs = append(wantIDs, u.String())
	}
	if !slices.Equal(got, wantIDs) {
		t.Errorf("got %v, want %v", got, wantIDs)
	}
}

// TestIntegrationPagingPeople: by name, repeated names broken by id, people
// without a name last.
func TestIntegrationPagingPeople(t *testing.T) {
	ctx, pool, h, users := pagingEnv(t, "people", 8)
	const city = "paging-people"
	exec(t, ctx, pool, `insert into app.cities (id, name, timezone, currency, languages, travel_modes)
		values ($1, '{"en":"Test"}', 'UTC', 'USD', '{en}', '{walk}')`, city)
	exec(t, ctx, pool, `insert into app.areas (id, city_id, name, centroid) values ($1, $1, '{"en":"Testville"}', 'POINT(0 0)')`, city)
	// Six named (three share a name), two who never finished onboarding.
	names := []string{"Pagetest Cy", "Pagetest Ann", "Pagetest Cy", "Pagetest Bo", "Pagetest Cy", "Pagetest Ann"}
	for i, name := range names {
		exec(t, ctx, pool, `insert into app.profiles (user_id, display_name, birth_year, gender, home_area_id, max_travel_min, energy, budget, size_pref, primary_language, conduct_version)
			values ($1, $2, 1995, 'man', $3, 30, 1, 1, 'small', 'en', 1)`, users[i], name, city)
	}
	// Matches every one of them by email.
	const query = "paging-people-"
	got := walk(t, 3, func(size int32, token string) ([]string, string, error) {
		res, err := h.SearchPeople(ctx, &isshov1.SearchPeopleRequest{Query: query, PageSize: size, PageToken: token})
		if err != nil {
			return nil, "", err
		}
		var ids []string
		for _, p := range res.People {
			ids = append(ids, p.UserId)
		}
		return ids, res.NextPageToken, nil
	})
	var want []string
	rows, err := pool.Query(ctx, `select u.id::text from app.users u left join app.profiles p on p.user_id = u.id
		where u.id = any($1) order by (coalesce(p.display_name, '') = ''), coalesce(p.display_name, ''), u.id`, users)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		want = append(want, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) || len(got) != 8 {
		t.Errorf("got %v, want %v", got, want)
	}
	// The unnamed two are last.
	if got[6] != want[6] || got[7] != want[7] {
		t.Error("unnamed people are not last")
	}
	// A token is for one query.
	res, err := h.SearchPeople(ctx, &isshov1.SearchPeopleRequest{Query: query, PageSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	_, err = h.SearchPeople(ctx, &isshov1.SearchPeopleRequest{Query: "Pagetest", PageToken: res.NextPageToken})
	wantInvalid(t, "token with another query", err)
}
