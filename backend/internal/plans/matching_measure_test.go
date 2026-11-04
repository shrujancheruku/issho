package plans

import (
	"context"
	"io"
	"log/slog"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// TestIntegrationMatchingMeasure records what a matching pass costs on a
// city of about 2,000 people with event history (#185): EXPLAIN ANALYZE of
// ListMatchingUsers and the time of LoadCity and a full Match. It is a
// measurement, not a check, so it only runs with ISSHO_MEASURE=1 (and a
// test database, as every integration test). The history is generated in
// SQL, so nothing large is committed:
//
//	ISSHO_MEASURE=1 go test -count=1 ./internal/plans -run MatchingMeasure -v
func TestIntegrationMatchingMeasure(t *testing.T) {
	if os.Getenv("ISSHO_MEASURE") == "" {
		t.Skip("ISSHO_MEASURE not set")
	}
	const people, groups = 2000, 6700 // about 20 outings each
	f := newFixture(t, "zz-plans-measure", "UTC", people)
	ctx := context.Background()
	// One connection, so the temp table below is visible to every statement.
	conn, err := f.pool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`insert into app.profiles (user_id, display_name, birth_year, gender, home_area_id, max_travel_min,
	        energy, budget, size_pref, primary_language, conduct_version)
	      select u, 'Test', 1995, 'unspecified', $2, 30, 2, 1, 'medium', 'en', 1 from unnest($1::uuid[]) u`, f.users, f.area)
	exec(`insert into app.user_languages (user_id, lang, fluent) select u, 'en', true from unnest($1::uuid[]) u`, f.users)
	exec(`insert into app.availability_slots (user_id, weekday, block) select u, 6, 'afternoon' from unnest($1::uuid[]) u`, f.users)
	exec(`insert into app.user_activity_prefs (user_id, activity_id, weight) select u, $2, 2 from unnest($1::uuid[]) u`, f.users, f.act)

	// One plan, event and group per outing; six consecutive people in each,
	// wrapping round. Most events are done, some groups cancelled, and the
	// members' responses, check-ins and feedback are spread so every count
	// has something to do.
	exec(`create temp table measure_ids as
	      select g, gen_random_uuid() as plan, gen_random_uuid() as event, gen_random_uuid() as grp
	      from generate_series(0, $1::int - 1) g`, groups)
	exec(`insert into app.plans (id, city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline,
	                             status, closed_reason, closed_at)
	      select plan, $1, $2, $3, 'en', 6, 1, date '2026-01-01' + (g % 300), 7, timestamptz '2026-01-01' + (g % 300) * interval '1 day',
	             'closed', 'done', timestamptz '2026-01-01'
	      from measure_ids`, f.city, f.area, f.act)
	// Sixty open plans, each with thirty people waiting (a window two weeks
	// out, so Match schedules none of them): what the scheduling stage loads.
	exec(`create temp table measure_open as select n, gen_random_uuid() as plan from generate_series(0, 59) n`)
	exec(`insert into app.plans (id, city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline)
	      select plan, $1, $2, $3, 'en', 30, 1, date '2026-06-12', 14, timestamptz '2026-06-20' from measure_open`, f.city, f.area, f.act)
	exec(`insert into app.plan_signups (plan_id, user_id)
	      select o.plan, ($1::uuid[])[1 + (o.n * 30 + k) % $2] from measure_open o, generate_series(0, 29) k`, f.users, people)
	exec(`insert into app.plan_signup_slots (plan_id, user_id, weekday, block)
	      select plan_id, user_id, 6, 'afternoon' from app.plan_signups where plan_id in (select plan from measure_open)`)
	exec(`insert into app.plan_signup_unavailable (plan_id, user_id, date, block)
	      select plan_id, user_id, date '2026-06-13', 'day' from app.plan_signups where plan_id in (select plan from measure_open)`)
	exec(`insert into app.events (id, plan_id, round, date, block, confirm_by, forced, status)
	      select event, plan, 1, date '2026-01-01' + (g % 300), 'afternoon', timestamptz '2026-01-01' + (g % 300) * interval '1 day', false,
	             case when g % 10 = 0 then 'scheduled' else 'done' end
	      from measure_ids`)
	exec(`insert into app.event_groups (id, event_id, quorum, min_attendance, score, status)
	      select grp, event, 2, 2, 1, case when g % 20 = 1 then 'cancelled' else 'confirmed' end from measure_ids`)
	exec(`insert into app.group_members (group_id, event_id, user_id, response, responded_at, checked_in_at)
	      select i.grp, i.event, ($1::uuid[])[1 + (i.g * 6 + k) % $2],
	             r.response, timestamptz '2026-01-01',
	             case when r.response = 'confirmed' and k % 3 <> 0 then timestamptz '2026-01-01' end
	      from measure_ids i, generate_series(0, 5) k,
	           lateral (select case when k = 5 then 'declined' when k = 4 and i.g % 2 = 0 then 'dropped' else 'confirmed' end as response) r`,
		f.users, people)
	exec(`insert into app.event_feedback (group_id, user_id, attended, plan_rating, group_rating)
	      select m.group_id, m.user_id, m.checked_in_at is null, case when m.checked_in_at is null then 4 end, case when m.checked_in_at is null then 4 end
	      from app.group_members m join measure_ids i on i.grp = m.group_id
	      where m.response = 'confirmed' and i.g % 4 = 0`)
	exec(`analyze`)

	// The statement as sqlc reads it, with its parameter.
	src, err := os.ReadFile("../../db/queries/matching.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, stmt, _ := strings.Cut(string(src), "-- name: ListMatchingUsers :many")
	stmt, _, _ = strings.Cut(stmt, "-- name: ")
	for run := 0; run < 2; run++ {
		rows, err := f.pool.Query(ctx, "explain (analyze, buffers) "+stmt, f.city)
		if err != nil {
			t.Fatal(err)
		}
		var plan []string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, line)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		if run == 1 { // the second run is warm
			t.Logf("EXPLAIN ANALYZE ListMatchingUsers:\n%s", strings.Join(plan, "\n"))
		}
	}

	median := func(what string, n int, fn func()) {
		var ds []time.Duration
		for range n {
			start := time.Now()
			fn()
			ds = append(ds, time.Since(start))
		}
		slices.Sort(ds)
		t.Logf("%s: median %v of %d (min %v, max %v)", what, ds[len(ds)/2], n, ds[0], ds[len(ds)-1])
	}
	median("ListMatchingUsers", 9, func() {
		if _, err := f.q.ListMatchingUsers(ctx, f.city); err != nil {
			t.Fatal(err)
		}
	})
	median("LoadCity", 5, func() {
		if _, err := LoadCity(ctx, f.q, f.city, 0); err != nil {
			t.Fatal(err)
		}
	})
	r := &Runner{DB: f.pool, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	now := time.Date(2026, 6, 2, 9, 0, 0, 0, time.UTC)
	for i := range 3 {
		start := time.Now()
		rep, err := r.Match(ctx, f.city, now.Add(time.Duration(i)*time.Hour))
		t.Logf("Match #%d: %v, %v (%+v)", i+1, time.Since(start), err, rep)
	}
}
