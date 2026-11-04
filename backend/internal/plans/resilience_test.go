package plans

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// failWhen makes the database reject an insert or update on a table when
// cond (SQL over NEW) holds, until the test ends: the way these tests make
// one item fail inside its transaction.
func failWhen(t *testing.T, pool *pgxpool.Pool, name, event, table, cond string) {
	t.Helper()
	ctx := context.Background()
	fn := "app.zz_plans_" + name
	stmts := []string{
		fmt.Sprintf(`create or replace function %s() returns trigger language plpgsql as $$ begin raise exception 'injected failure'; end $$`, fn),
		fmt.Sprintf(`create trigger zz_plans_%s before %s on %s for each row when (%s) execute function %s()`, name, event, table, cond, fn),
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, s := range []string{
			fmt.Sprintf(`drop trigger if exists zz_plans_%s on %s`, name, table),
			fmt.Sprintf(`drop function if exists %s()`, fn),
		} {
			if _, err := pool.Exec(ctx, s); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	})
}

// cancelledCtx reports itself cancelled (Err) but leaves queries working
// (Done), so a pass's setup succeeds and only its per-item check sees it.
type cancelledCtx struct{ context.Context }

func (cancelledCtx) Err() error { return context.Canceled }

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// threePlans opens three plans in the city, each with three people signed
// up (nine people), and returns them and the time all three are due: their
// signup deadline, when a plan goes ahead with whoever can form a group.
func threePlans(t *testing.T, f *fixture, r *Runner) ([]uuid.UUID, time.Time) {
	t.Helper()
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	w := DefaultTiming().Round(time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC), tokyo)
	var ids []uuid.UUID
	for i := range 3 {
		plan := matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}
		id, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: plan, Audience: 3, Score: 1}, w, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range f.users[i*3 : i*3+3] {
			if err := SaveSignup(ctx, f.pool, id, matching.Signup{User: matching.UserID(u.String())}); err != nil {
				t.Fatal(err)
			}
		}
		ids = append(ids, id)
	}
	return ids, w.Deadline
}

// TestIntegrationScheduleSkipsFailedPlan: with three open plans, the middle
// one failing doesn't stop the other two. The report counts 2 scheduled
// and 1 failed, and the pass succeeds. When every plan fails it is an error.
func TestIntegrationScheduleSkipsFailedPlan(t *testing.T) {
	f := newFixture(t, "zz-plans-skipfail", "Asia/Tokyo", 9)
	ctx := context.Background()
	r := &Runner{DB: f.pool, Log: quiet()}
	ids, due := threePlans(t, f, r)

	// A cancelled pass (River timeout or shutdown) stops with the context's
	// error, and doesn't count the plans it never reached as failed.
	if rep, err := r.Schedule(cancelledCtx{ctx}, f.city, due); !errors.Is(err, context.Canceled) || rep.Failed != 0 || rep.Scheduled != 0 {
		t.Errorf("cancelled Schedule = %+v, %v; want context.Canceled and nothing failed", rep, err)
	}

	failWhen(t, f.pool, "event", "insert", "app.events", fmt.Sprintf("new.plan_id = '%s'", ids[1]))
	rep, err := r.Schedule(ctx, f.city, due)
	if err != nil {
		t.Fatalf("Schedule = %+v, %v; some plans failing is still success", rep, err)
	}
	if rep != (ScheduleReport{Checked: 3, Scheduled: 2, Closed: 2, Failed: 1}) {
		t.Errorf("report = %+v, want 3 checked, 2 scheduled (and closed, being forced), 1 failed", rep)
	}
	var events int
	if err := f.pool.QueryRow(ctx, "select count(*) from app.events where plan_id = any($1)", ids).Scan(&events); err != nil || events != 2 {
		t.Errorf("%d events (%v), want 2: the failed plan has none", events, err)
	}

	// Every plan failing is an error, so the job is retried. The failed plan
	// from before is still due; make the other two fail too.
	failWhen(t, f.pool, "event2", "insert", "app.events", "true")
	if rep, err := r.Schedule(ctx, f.city, due); err == nil {
		t.Errorf("every plan failed: %+v, nil; want an error", rep)
	} else if rep.Failed != rep.Checked || rep.Checked == 0 {
		t.Errorf("report = %+v, want every plan failed", rep)
	}
}

// TestIntegrationSettleSkipsFailedGroup: with two groups past their
// deadline, one failing is rolled back and not counted, the other is
// settled, and the pass succeeds. Both failing is an error.
func TestIntegrationSettleSkipsFailedGroup(t *testing.T) {
	f := newFixture(t, "zz-plans-settlefail", "UTC", 4)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	day := func(d, h int) time.Time { return time.Date(2030, 1, d, h, 0, 0, 0, time.UTC) }
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}, Audience: 4, Score: 1},
		Window{Start: day(7, 0), Days: 14, Deadline: day(14, 0)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := func(us ...uuid.UUID) []matching.UserID {
		out := make([]matching.UserID, len(us))
		for i, u := range us {
			out[i] = matching.UserID(u.String())
		}
		return out
	}
	eventID, _, err := SaveEvent(ctx, f.pool, Event{
		PlanID: planID, Round: 1, StartsAt: day(12, 14), ConfirmBy: day(11, 0), Now: day(5, 0),
		Next: Window{Start: day(21, 0), Days: 14, Deadline: day(28, 0)},
		Scheduled: matching.Scheduled{
			Occasion: matching.Occasion{Date: day(12, 0), Block: matching.Afternoon},
			Groups: []matching.Group{
				{Invited: ids(f.users[0], f.users[1]), Quorum: 2, MinAttendance: 2, Score: 1},
				{Invited: ids(f.users[2], f.users[3]), Quorum: 2, MinAttendance: 2, Score: 1},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var groups []uuid.UUID
	rows, err := f.pool.Query(ctx, "select id from app.event_groups where event_id = $1 order by id", eventID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		groups = append(groups, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatalf("%d groups", len(groups))
	}

	if rep, err := (&Runner{DB: f.pool, Log: quiet()}).Settle(cancelledCtx{ctx}, f.city, day(11, 0)); !errors.Is(err, context.Canceled) || rep != (SettleReport{}) {
		t.Errorf("cancelled Settle = %+v, %v; want context.Canceled, nothing counted or failed", rep, err)
	}

	// Nobody confirmed, so both would be cancelled at the deadline. The
	// first one's cancelling fails after the counter would have gone up.
	r := &Runner{DB: f.pool, Log: quiet()}
	failWhen(t, f.pool, "group", "update", "app.event_groups", fmt.Sprintf("new.id = '%s' and new.status = 'cancelled'", groups[0]))
	rep, err := r.Settle(ctx, f.city, day(11, 0))
	if err != nil {
		t.Fatalf("Settle = %+v, %v; one group failing is still success", rep, err)
	}
	if rep != (SettleReport{Cancelled: 1, Failed: 1}) {
		t.Errorf("report = %+v, want 1 cancelled (the rolled-back group isn't counted) and 1 failed", rep)
	}
	status := func(id uuid.UUID) string {
		var s string
		if err := f.pool.QueryRow(ctx, "select status from app.event_groups where id = $1", id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	if got := status(groups[0]); got != GroupInviting {
		t.Errorf("failed group is %s, want it left inviting by the rollback", got)
	}
	if got := status(groups[1]); got != GroupCancelled {
		t.Errorf("other group is %s, want cancelled", got)
	}

	// The failed group is the only one left and fails again: every item of
	// the stage failed.
	rep, err = r.Settle(ctx, f.city, day(11, 0))
	if err == nil || !strings.Contains(err.Error(), "every item failed") || rep.Failed != 1 || rep.Cancelled != 0 {
		t.Errorf("Settle with only a failing group = %+v, %v; want an error and no counts", rep, err)
	}
}

// TestIntegrationSetupFailure: a city that can't be loaded is an error from
// every stage and from Match, with no work done.
func TestIntegrationSetupFailure(t *testing.T) {
	f := newFixture(t, "zz-plans-setupfail", "UTC", 1)
	ctx := context.Background()
	r := &Runner{DB: f.pool, Log: quiet()}
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := r.Match(ctx, "zz-no-such-city", now); err == nil {
		t.Error("Match of an unknown city: no error")
	}
	if _, err := r.Settle(ctx, "zz-no-such-city", now); err == nil {
		t.Error("Settle of an unknown city: no error")
	}
	if _, err := r.Schedule(ctx, "zz-no-such-city", now); err == nil {
		t.Error("Schedule of an unknown city: no error")
	}
	if _, err := r.Suggest(ctx, "zz-no-such-city", now); err == nil {
		t.Error("Suggest of an unknown city: no error")
	}
}

// TestIntegrationSeasonalActivity: Match loads the city once with every
// activity, and Suggest still judges seasons by its round's month, while
// Schedule keeps a plan whose activity has gone out of season.
func TestIntegrationSeasonalActivity(t *testing.T) {
	f := newFixture(t, "zz-plans-season", "UTC", 8)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	// In season in June only.
	if _, err := f.pool.Exec(ctx, `insert into app.city_activity_overrides (city_id, activity_id, enabled, months) values ($1, $2, true, '{6}')`, f.city, f.act); err != nil {
		t.Fatal(err)
	}
	r := &Runner{DB: f.pool, Log: quiet()}

	// A January round: the activity is out of season, so nothing opens.
	winter := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	if m, err := r.Match(ctx, f.city, winter); err != nil || m.Opened != 0 {
		t.Fatalf("Match in January = %+v, %v; want nothing opened", m, err)
	}
	// A late-June round starts in June, in season, but its signup deadline
	// falls in July.
	summer := time.Date(2030, 6, 24, 9, 0, 0, 0, time.UTC)
	m, err := r.Match(ctx, f.city, summer)
	if err != nil || m.Opened != 1 {
		t.Fatalf("Match in June = %+v, %v; want the plan opened", m, err)
	}

	// The plan keeps its round when the season ends: Schedule loads every
	// activity (month 0), so at its deadline in July it still goes ahead.
	open, err := ListOpen(ctx, f.q, f.city)
	if err != nil || len(open) != 1 {
		t.Fatalf("open plans = %+v, %v", open, err)
	}
	if open[0].Window.Start.Month() != time.June || open[0].Window.Deadline.Month() != time.July {
		t.Fatalf("window %v to %v; the test needs it to start in June and end in July", open[0].Window.Start, open[0].Window.Deadline)
	}
	for _, u := range f.users {
		if err := SaveSignup(ctx, f.pool, open[0].ID, matching.Signup{User: matching.UserID(u.String())}); err != nil {
			t.Fatal(err)
		}
	}
	m, err = r.Match(ctx, f.city, open[0].Window.Deadline)
	if err != nil || m.Scheduled.Scheduled != 1 {
		t.Errorf("Match after the season = %+v, %v; want the plan still scheduled", m, err)
	}
	if m.Opened != 0 {
		t.Errorf("Match after the season opened %d plans; the activity is out of season", m.Opened)
	}
}

// TestIntegrationSuggestFailedItem: opening a suggestion that fails in the
// database is counted in Failed, and when every suggestion tried failed the
// pass is an error. (Walks the composer couldn't make don't count: see
// Runner.suggest.)
func TestIntegrationSuggestFailedItem(t *testing.T) {
	f := newFixture(t, "zz-plans-suggestfail", "UTC", 8)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	r := &Runner{DB: f.pool, Log: quiet()}
	failWhen(t, f.pool, "plan", "insert", "app.plans", fmt.Sprintf("new.city_id = '%s'", f.city))
	rep, err := r.Suggest(ctx, f.city, time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC))
	if err == nil || rep.Failed != 1 || rep.Opened != 0 {
		t.Errorf("Suggest = %+v, %v; want one failed and an error, as every suggestion tried failed", rep, err)
	}
}
