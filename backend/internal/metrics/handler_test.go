package metrics

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

func TestMonday(t *testing.T) {
	for in, want := range map[string]string{
		"2030-01-14": "2030-01-14", // Monday
		"2030-01-16": "2030-01-14",
		"2030-01-20": "2030-01-14", // Sunday
		"2030-01-01": "2029-12-31",
	} {
		d, _ := time.Parse(time.DateOnly, in)
		if got := Monday(d).Format(time.DateOnly); got != want {
			t.Errorf("Monday(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestNonAdminRefused(t *testing.T) {
	h := &Handler{}
	ctx := auth.WithCaller(context.Background(), auth.Caller{UserID: uuid.New()})
	if _, err := h.GetCityMetrics(ctx, connect.NewRequest(&isshov1.GetCityMetricsRequest{})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("as a user: %v", err)
	}
}

// TestIntegrationMetrics: two finished outings a week apart (one person at
// both), a cancelled group, a no-show, feedback and picks; the weekly
// numbers and the city now. Skipped unless TEST_DATABASE_URL is set.
func TestIntegrationMetrics(t *testing.T) {
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
	const city, area, act = "zz-metrics", "zz-metrics-a", "zz-metrics"
	// Stable ids, so a rerun deletes an interrupted run's rows first.
	user := func(name string) uuid.UUID {
		return uuid.NewSHA1(uuid.NameSpaceURL, []byte("issho-test:metrics/"+name))
	}
	a, b, c, d, e, f := user("a"), user("b"), user("c"), user("d"), user("e"), user("f")
	users := []uuid.UUID{a, b, c, d, e, f}
	var supabase bool
	if err := pool.QueryRow(ctx, "select exists (select 1 from pg_namespace where nspname = 'auth')").Scan(&supabase); err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		stmts := [][]any{{"delete from app.users where id = any($1)", users}}
		if supabase {
			stmts = append(stmts, []any{"delete from auth.users where id = any($1)", users})
		}
		stmts = append(stmts, []any{"delete from app.cities where id = $1", city}, []any{"delete from app.activities where id = $1", act})
		for _, s := range stmts {
			if _, err := pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	plan, e1, e2 := uuid.New(), uuid.New(), uuid.New()
	g1, g2, g3 := uuid.New(), uuid.New(), uuid.New()
	setup := [][]any{
		{`insert into app.cities (id, name, timezone, currency, languages, travel_modes) values ($1, '{"en":"M"}', 'Asia/Tokyo', 'JPY', '{ja}', '{walk}')`, city},
		{`insert into app.areas (id, city_id, name, centroid) values ($1, $2, '{"en":"A"}', 'POINT(0 0)')`, area, city},
		{`insert into app.activities (id, name, category, indoor, energy, cost_level, conversational, min_group_size, max_group_size)
		  values ($1, '{"en":"M"}', 'test', true, 1, 0, true, 2, 6)`, act},
	}
	for i, id := range users {
		if supabase {
			setup = append(setup, []any{"insert into auth.users (id, email) values ($1, $2)", id, id.String() + "@example.com"})
		}
		// a joined in the second week; the others long before.
		created := time.Date(2029, 6, 1, 0, 0, 0, 0, time.UTC)
		if i == 0 {
			created = time.Date(2030, 1, 7, 3, 0, 0, 0, time.UTC)
		}
		setup = append(setup, []any{"insert into app.users (id, created_at) values ($1, $2)", id, created})
	}
	setup = append(setup,
		[]any{`insert into app.profiles (user_id, display_name, birth_year, gender, home_area_id, max_travel_min, energy, budget, size_pref, primary_language, conduct_version)
		  values ($1, 'A', 1995, 'unspecified', $2, 30, 2, 1, 'medium', 'ja', 1)`, a, area},
		[]any{`insert into app.plans (id, city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline, status)
		  values ($1, $2, $3, $4, 'ja', 6, 1, '2029-12-25', 14, '2030-01-20', 'open')`, plan, city, area, act},
		[]any{`insert into app.events (id, plan_id, round, date, block, starts_at, confirm_by, status, forced) values
		  ($1, $3, 1, '2030-01-01', 'afternoon', '2030-01-01 05:00Z', '2029-12-31 00:00Z', 'done', false),
		  ($2, $3, 2, '2030-01-08', 'afternoon', '2030-01-08 05:00Z', '2030-01-07 00:00Z', 'done', false)`, e1, e2, plan},
		[]any{`insert into app.event_groups (id, event_id, quorum, min_attendance, score, status) values
		  ($1, $4, 3, 2, 1, 'confirmed'), ($2, $5, 2, 2, 1, 'confirmed'), ($3, $5, 2, 2, 1, 'cancelled')`, g1, g2, g3, e1, e2},
		// Week 1: a checked in, b said so in feedback, c didn't come, d declined.
		// Week 2: a and e checked in; f's group was cancelled.
		[]any{`insert into app.group_members (group_id, event_id, user_id, response, checked_in_at, responded_at) values
		  ($1, $3, $5, 'confirmed', now(), now()), ($1, $3, $6, 'confirmed', null, now()), ($1, $3, $7, 'confirmed', null, now()), ($1, $3, $8, 'declined', null, now()),
		  ($2, $4, $5, 'confirmed', now(), now()), ($2, $4, $9, 'confirmed', now(), now())`, g1, g2, e1, e2, a, b, c, d, e},
		[]any{`insert into app.group_members (group_id, event_id, user_id, response, checked_in_at, responded_at) values ($1, $2, $3, 'confirmed', now(), now())`, g3, e2, f},
		[]any{`insert into app.event_feedback (group_id, user_id, attended, plan_rating, group_rating) values
		  ($1, $3, true, 2, 2), ($2, $4, true, 4, 5), ($2, $5, true, 3, 3)`, g1, g2, b, a, e},
		[]any{`insert into app.mate_picks (group_id, picker_id, picked_id, pick) values
		  ($1, $3, $4, 'avoid'), ($2, $4, $5, 'again'), ($2, $5, $4, 'again')`, g1, g2, b, a, e},
	)
	for i, s := range setup {
		if _, err := pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
			t.Fatalf("setup %d: %v", i, err)
		}
	}

	h := &Handler{Q: dbgen.New(pool), Now: func() time.Time { return time.Date(2030, 1, 16, 3, 0, 0, 0, time.UTC) }}
	adminCtx := auth.WithCaller(ctx, auth.Caller{UserID: uuid.New(), Admin: true})
	res, err := h.GetCityMetrics(adminCtx, connect.NewRequest(&isshov1.GetCityMetricsRequest{CityId: city, Weeks: 3}))
	if err != nil {
		t.Fatal(err)
	}
	w := res.Msg.Weeks
	if len(w) != 3 || w[0].Week != "2029-12-31" || w[2].Week != "2030-01-14" {
		t.Fatalf("weeks = %v", w)
	}
	show := func(m *isshov1.WeekMetrics) string {
		return fmt.Sprintf("att %d repeat %d first %d groups %d inv %d conf %d went %d rating %.1f/%.1f picks %d/%d mutual %d new %d",
			m.Attendees, m.RepeatAttendees, m.FirstTimers, m.Groups, m.Invited, m.Confirmed, m.Attended,
			m.PlanRating, m.GroupRating, m.PicksAgain, m.Picks, m.MutualPairs, m.NewPeople)
	}
	for i, want := range []string{
		"att 2 repeat 0 first 2 groups 1 inv 4 conf 3 went 2 rating 2.0/2.0 picks 0/1 mutual 0 new 0",
		"att 2 repeat 1 first 1 groups 1 inv 2 conf 2 went 2 rating 3.5/4.0 picks 2/2 mutual 1 new 1",
		"att 0 repeat 0 first 0 groups 0 inv 0 conf 0 went 0 rating 0.0/0.0 picks 0/0 mutual 0 new 0",
	} {
		if got := show(w[i]); got != want {
			t.Errorf("week %s:\n got %s\nwant %s", w[i].Week, got, want)
		}
	}
	if n := res.Msg.Now; n.People != 1 || n.OpenPlans != 1 || n.UpcomingOutings != 0 {
		t.Errorf("now = %v", n)
	}
	if _, err := h.GetCityMetrics(adminCtx, connect.NewRequest(&isshov1.GetCityMetricsRequest{CityId: "nowhere"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("an unknown city: %v", err)
	}
	if _, err := h.GetCityMetrics(adminCtx, connect.NewRequest(&isshov1.GetCityMetricsRequest{CityId: city, Weeks: 53})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("53 weeks: %v", err)
	}
}
