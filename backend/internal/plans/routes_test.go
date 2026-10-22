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

type fakeRoutes struct{ calls int }

func (f *fakeRoutes) WalkingRoute(_ context.Context, from, to string) (maps.Route, error) {
	f.calls++
	return maps.Route{Polyline: "enc:" + from + ">" + to, Duration: 9*time.Minute + 10*time.Second, Meters: 700}, nil
}

// TestIntegrationRoutes: a plan with two venues gets its walking route on
// the next pass and nothing more until it's 25 days old; past 30 days
// without a finder it's cleared; a plan with one venue never asks Google.
// The plan screen's walk comes from the route.
func TestIntegrationRoutes(t *testing.T) {
	f := newFixture(t, "zz-plans-routes", "UTC", 1)
	ctx := context.Background()
	f.addProfile(t, f.users[0], "ja")
	for _, v := range []string{"zz-route-cafe", "zz-route-bar"} {
		if _, err := f.pool.Exec(ctx, `insert into app.venues (google_place_id, city_id, area_id, category_id) values ($1, $2, $3, 'cafe')`, v, f.city, f.area); err != nil {
			t.Fatal(err)
		}
	}
	day := func(d int) time.Time { return time.Date(2030, 1, d, 0, 0, 0, 0, time.UTC) }
	w := Window{Start: day(7), Days: 14, Deadline: day(14)}
	act := &matching.Activity{ID: matching.ActivityID(f.act), VenueCategories: []matching.CategoryID{"cafe"}}
	acts := map[matching.ActivityID]*matching.Activity{act.ID: act}
	two, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: act.ID, FollowUp: act.ID, Primary: "ja"}, Audience: 6, Score: 1}, w, acts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: act.ID, Primary: "en"}, Audience: 6, Score: 1}, w, acts); err != nil {
		t.Fatal(err)
	}
	finder := &fakeRoutes{}
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Routes: finder}

	now := time.Now()
	if computed, cleared, err := r.RefreshRoutes(ctx, f.city, now); err != nil || computed != 1 || cleared != 0 || finder.calls != 1 {
		t.Fatalf("first pass: computed=%d cleared=%d calls=%d err=%v; want only the two-venue plan", computed, cleared, finder.calls, err)
	}
	if computed, _, _ := r.RefreshRoutes(ctx, f.city, now); computed != 0 || finder.calls != 1 {
		t.Errorf("second pass computed %d (calls %d); a fresh route isn't recomputed", computed, finder.calls)
	}
	open, err := ListOpen(ctx, f.q, f.city)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range open {
		if p.ID == two && (p.Route == nil || p.Route.Seconds != 550 || p.Route.Minutes() != 10) {
			t.Errorf("route on the open plan = %+v; want 550 s, 10 min", p.Route)
		}
	}
	// The plan screen's walk is the route's (no Places key in this test).
	_, steps, _, err := (&Service{DB: f.pool}).Plan(ctx, f.users[0], two)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 2 || steps[1].WalkMinutes != 10 {
		t.Errorf("steps = %+v; want the second stop 10 min away", steps)
	}

	recomputedAt := now.Add(RouteRefreshAge + time.Hour)
	if computed, _, _ := r.RefreshRoutes(ctx, f.city, recomputedAt); computed != 1 {
		t.Errorf("after %s: computed %d, want 1", RouteRefreshAge, computed)
	}
	// Without a finder (no Maps key), a route past the limit is cleared.
	noKey := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: r.Log}
	if computed, cleared, err := noKey.RefreshRoutes(ctx, f.city, recomputedAt.Add(RouteMaxAge+time.Hour)); err != nil || computed != 0 || cleared != 1 {
		t.Errorf("past the limit without a key: computed=%d cleared=%d err=%v; want 1 cleared", computed, cleared, err)
	}
}
