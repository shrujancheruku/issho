package plans

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// fakeHours answers every venue with the same hours and counts lookups.
type fakeHours struct {
	h     maps.Hours
	calls int
}

func (f *fakeHours) Hours(context.Context, string) (maps.Hours, error) {
	f.calls++
	return f.h, nil
}

// everyDay is open the same hours every day.
func everyDay(open, close int) maps.Hours {
	h := maps.Hours{Known: true}
	for d := time.Sunday; d <= time.Saturday; d++ {
		h.Periods = append(h.Periods, maps.Period{Open: maps.WeekTime{Day: d, Hour: open}, Close: maps.WeekTime{Day: d, Hour: close}})
	}
	return h
}

// TestIntegrationVenueHours: eight people free only on Saturday afternoons
// and a plan at a café. While the café opens only in the morning, no time
// works and the plan waits; once it's open in the afternoon, the round is
// scheduled; when it closes for good, the next round is withdrawn and the
// café is never picked again.
func TestIntegrationVenueHours(t *testing.T) {
	f := newFixture(t, "zz-plans-hours", "Asia/Tokyo", 8)
	ctx := context.Background()
	for _, u := range f.users {
		f.addProfile(t, u, "ja")
	}
	if _, err := f.pool.Exec(ctx, `insert into app.venues (google_place_id, city_id, area_id, category_id) values ('zz-hours-cafe', $1, $2, 'cafe')`, f.city, f.area); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	tm := DefaultTiming()
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	act := &matching.Activity{ID: matching.ActivityID(f.act), VenueCategories: []matching.CategoryID{"cafe"}}
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: act.ID, Primary: "ja"}, Audience: 8, Score: 1},
		tm.Round(now, tokyo), map[matching.ActivityID]*matching.Activity{act.ID: act})
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range f.users {
		if err := SaveSignup(ctx, f.pool, planID, matching.Signup{User: matching.UserID(u.String())}); err != nil {
			t.Fatal(err)
		}
	}
	hours := &fakeHours{h: everyDay(8, 12)}
	r := &Runner{DB: f.pool, Timing: tm, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Hours: hours}

	// Open 8–12 only: neither Saturday afternoon works. Hours are looked up
	// once for the pass, not once per occasion tried.
	if rep, err := r.Schedule(ctx, f.city, now); err != nil || rep != (ScheduleReport{Checked: 1}) {
		t.Fatalf("café closed in the afternoon: %+v, %v", rep, err)
	}
	if hours.calls != 1 {
		t.Errorf("hours looked up %d times in one pass, want 1", hours.calls)
	}

	hours.h = everyDay(10, 22)
	if rep, err := r.Schedule(ctx, f.city, now); err != nil || rep.Scheduled != 1 {
		t.Fatalf("café open: %+v, %v", rep, err)
	}

	// Two people roll over to round 2. At its deadline the café has closed for good.
	open, err := ListOpen(ctx, f.q, f.city)
	if err != nil || len(open) != 1 {
		t.Fatalf("round 2: %+v, %v", open, err)
	}
	hours.h = maps.Hours{Closed: true, PermanentlyClosed: true}
	if rep, err := r.Schedule(ctx, f.city, open[0].Window.Deadline); err != nil || rep != (ScheduleReport{Checked: 1, Closed: 1}) {
		t.Fatalf("café closed for good: %+v, %v", rep, err)
	}
	p, err := f.q.GetPlan(ctx, planID)
	if err != nil || p.ClosedReason == nil || *p.ClosedReason != ClosedWithdrawn {
		t.Errorf("plan after the café closed: %+v, %v", p, err)
	}
	var status string
	if err := f.pool.QueryRow(ctx, "select status from app.venues where google_place_id = 'zz-hours-cafe'").Scan(&status); err != nil || status != "rejected" {
		t.Errorf("closed café status = %q, %v; want rejected", status, err)
	}
}
