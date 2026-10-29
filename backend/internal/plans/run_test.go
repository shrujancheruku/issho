package plans

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

func TestTiming(t *testing.T) {
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	tm := DefaultTiming()
	if err := tm.Validate(); err != nil {
		t.Fatal(err)
	}
	// 20:00 UTC on Tuesday 1 January is already Wednesday 2 January in Tokyo.
	now := time.Date(2030, 1, 1, 20, 0, 0, 0, time.UTC)
	w := tm.Round(now, tokyo)
	if want := time.Date(2030, 1, 4, 0, 0, 0, 0, time.UTC); !w.Start.Equal(want) || w.Days != 14 {
		t.Errorf("window = %v + %d days, want %v + 14", w.Start, w.Days, want)
	}
	if want := time.Date(2030, 1, 11, 0, 0, 0, 0, tokyo); !w.Deadline.Equal(want) {
		t.Errorf("deadline = %v, want %v", w.Deadline, want)
	}
	// At the deadline, the occasions left start LeadDays later and run to
	// the window's end.
	got := tm.occasions(w, w.Deadline, tokyo)
	if first, last := got[0].Date, got[len(got)-1].Date; !first.Equal(time.Date(2030, 1, 13, 0, 0, 0, 0, time.UTC)) ||
		!last.Equal(time.Date(2030, 1, 17, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("occasions at the deadline: %v to %v, want 13 to 17 January", first, last)
	}
	if got := confirmBy(matching.Occasion{Date: time.Date(2030, 1, 5, 0, 0, 0, 0, time.UTC)}, tokyo); !got.Equal(time.Date(2030, 1, 4, 0, 0, 0, 0, tokyo)) {
		t.Errorf("confirmBy = %v", got)
	}
	for _, bad := range []Timing{{LeadDays: 0, WindowDays: 14, SignupDays: 7}, {LeadDays: 2, WindowDays: 9, SignupDays: 7}} {
		if bad.Validate() == nil {
			t.Errorf("%+v: no error", bad)
		}
	}
}

// TestIntegrationRunner opens a plan for a city of eight people who love the
// same activity, schedules it once they all sign up (six fit; two roll over
// to the next round), then checks that the next round closes at its deadline
// when it can't form, and isn't suggested again.
func TestIntegrationRunner(t *testing.T) {
	f := newFixture(t, "zz-plans-runner", "Asia/Tokyo", 8)
	ctx := context.Background()
	for _, id := range f.users {
		f.addProfile(t, id, "ja")
	}
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	// Tuesday 1 January, 18:00 in Tokyo: the window runs 3–16 January.
	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)

	opened, err := r.Suggest(ctx, f.city, now)
	if err != nil {
		t.Fatal(err)
	}
	open, err := ListOpen(ctx, f.q, f.city)
	if err != nil {
		t.Fatal(err)
	}
	// One plan: the city's cap is one per ten people.
	if opened != 1 || len(open) != 1 || open[0].Activity != matching.ActivityID(f.act) || open[0].Primary != "ja" {
		t.Fatalf("Suggest opened %d: %+v", opened, open)
	}
	plan := open[0]

	// Nobody has signed up yet: nothing happens before the deadline.
	if rep, err := r.Schedule(ctx, f.city, now); err != nil || rep != (ScheduleReport{Checked: 1}) {
		t.Fatalf("Schedule with no signups = %+v, %v", rep, err)
	}
	for _, id := range f.users {
		if err := SaveSignup(ctx, f.pool, plan.ID, matching.Signup{User: matching.UserID(id.String())}); err != nil {
			t.Fatal(err)
		}
	}
	rep, err := r.Schedule(ctx, f.city, now)
	if err != nil {
		t.Fatal(err)
	}
	// The activity takes at most six, so two are left waiting and the plan
	// moves to its next round.
	if rep != (ScheduleReport{Checked: 1, Scheduled: 1}) {
		t.Fatalf("Schedule = %+v", rep)
	}
	var date, confirm time.Time
	var block string
	var forced bool
	var invited int
	err = f.pool.QueryRow(ctx, `
		select e.date, e.block, e.confirm_by, e.forced,
		       (select count(*) from app.group_members m where m.event_id = e.id)
		from app.events e where e.plan_id = $1`, plan.ID).
		Scan(&date, &block, &confirm, &forced, &invited)
	if err != nil {
		t.Fatal(err)
	}
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	// The first Saturday afternoon in the window.
	if !date.Equal(time.Date(2030, 1, 5, 0, 0, 0, 0, time.UTC)) || block != "afternoon" || forced || invited != 6 ||
		!confirm.Equal(time.Date(2030, 1, 4, 0, 0, 0, 0, tokyo)) {
		t.Errorf("event %v %s confirm by %v forced=%v invited=%d", date, block, confirm, forced, invited)
	}
	open, err = ListOpen(ctx, f.q, f.city)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].Round != 2 || open[0].Signups != 2 {
		t.Fatalf("after scheduling: %+v, want round 2 with 2 waiting", open)
	}
	// The city's cap is reached, so nothing new opens.
	if opened, err := r.Suggest(ctx, f.city, now); err != nil || opened != 0 {
		t.Fatalf("second Suggest opened %d, %v", opened, err)
	}

	// One of the two withdraws; at the deadline one person can't make a group.
	signups, err := LoadSignups(ctx, f.q, plan.ID, open[0].Window.Start)
	if err != nil {
		t.Fatal(err)
	}
	if err := Withdraw(ctx, f.q, plan.ID, uuid.MustParse(string(signups[0].User))); err != nil {
		t.Fatal(err)
	}
	deadline := open[0].Window.Deadline
	if rep, err := r.Schedule(ctx, f.city, deadline); err != nil || rep != (ScheduleReport{Checked: 1, Closed: 1}) {
		t.Fatalf("Schedule at the deadline = %+v, %v", rep, err)
	}
	if p, err := f.q.GetPlan(ctx, plan.ID); err != nil || p.ClosedReason == nil || *p.ClosedReason != ClosedNoGroup {
		t.Fatalf("plan after the deadline: %+v, %v", p, err)
	}
	// The one still waiting hears it closed (not the one who withdrew).
	var told []uuid.UUID
	rows, err := f.pool.Query(ctx, "select user_id from app.notifications where plan_id = $1 and kind = $2", plan.ID, NotifyNoGroup)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var u uuid.UUID
		if err := rows.Scan(&u); err != nil {
			t.Fatal(err)
		}
		told = append(told, u)
	}
	if rows.Err() != nil || len(told) != 1 || told[0].String() != string(signups[1].User) {
		t.Errorf("told %v, want only %s (%v)", told, signups[1].User, rows.Err())
	}
	// It's in its cool-down now, so it isn't suggested again.
	if opened, err := r.Suggest(ctx, f.city, deadline); err != nil || opened != 0 {
		t.Errorf("Suggest during the cool-down opened %d, %v", opened, err)
	}
	// Its screen still opens for two weeks, saying why it closed; then it's gone.
	svc := &Service{DB: f.pool}
	waiting := uuid.MustParse(string(signups[1].User))
	if it, _, _, err := svc.Plan(ctx, waiting, plan.ID, time.Now()); err != nil || it.ClosedReason != ClosedNoGroup || !it.SignedUp || it.Title == "" {
		t.Errorf("closed plan's screen = %+v, %v", it, err)
	}
	if _, err := f.pool.Exec(ctx, "update app.plans set closed_at = now() - interval '15 days' where id = $1", plan.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := svc.Plan(ctx, waiting, plan.ID, time.Now()); !errors.Is(err, ErrNotFound) {
		t.Errorf("after 15 days: %v, want ErrNotFound", err)
	}
}
