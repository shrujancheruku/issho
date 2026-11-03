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

// TestIntegrationCityTiming: a city's own timing (from its pack's matching
// block) sets its round's window and an event's start time; a city without
// one keeps DefaultTiming.
func TestIntegrationCityTiming(t *testing.T) {
	f := newFixture(t, "zz-plans-timing", "Asia/Tokyo", 8)
	ctx := context.Background()
	for _, id := range f.users {
		f.addProfile(t, id, "ja")
	}
	if tm, _, err := cityTiming(ctx, f.q, f.city); err != nil ||
		tm.LeadDays != DefaultTiming().LeadDays || tm.BlockStarts[matching.Evening] != DefaultTiming().BlockStarts[matching.Evening] {
		t.Fatalf("city without an override: %+v, %v; want DefaultTiming", tm, err)
	}
	// Lead 3 days (not 2); afternoons at 15:30 (not 14:00).
	if _, err := f.pool.Exec(ctx, `update app.cities set matching = '{"timing": {"lead_days": 3, "block_starts": {"afternoon": "15:30"}}}' where id = $1`, f.city); err != nil {
		t.Fatal(err)
	}
	r := &Runner{DB: f.pool, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	// Tuesday 1 January, 18:00 in Tokyo; with a 3-day lead the window starts
	// on Friday 4 (the default would be Thursday 3).
	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	if rep, err := r.Suggest(ctx, f.city, now); err != nil || rep.Opened != 1 {
		t.Fatalf("Suggest = %+v, %v", rep, err)
	}
	open, err := ListOpen(ctx, f.q, f.city)
	if err != nil || len(open) != 1 {
		t.Fatalf("open = %+v, %v", open, err)
	}
	if want := time.Date(2030, 1, 4, 0, 0, 0, 0, time.UTC); !open[0].Window.Start.Equal(want) {
		t.Errorf("window starts %v, want %v", open[0].Window.Start, want)
	}
	for _, id := range f.users {
		if err := SaveSignup(ctx, f.pool, open[0].ID, matching.Signup{User: matching.UserID(id.String())}); err != nil {
			t.Fatal(err)
		}
	}
	if rep, err := r.Schedule(ctx, f.city, now); err != nil || rep.Scheduled != 1 {
		t.Fatalf("Schedule = %+v, %v", rep, err)
	}
	var block string
	var startsAt time.Time
	if err := f.pool.QueryRow(ctx, `select block, starts_at from app.events where plan_id = $1`, open[0].ID).Scan(&block, &startsAt); err != nil {
		t.Fatal(err)
	}
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	if h, m, _ := startsAt.In(tokyo).Clock(); block != "afternoon" || h != 15 || m != 30 {
		t.Errorf("event starts %v (%s); want 15:30 Tokyo time", startsAt.In(tokyo), block)
	}
}

// TestIntegrationCityMeetingPlaces: the city's meeting_places set the order
// and shape of the searches; with none it searches stations, then parks.
func TestIntegrationCityMeetingPlaces(t *testing.T) {
	f := newFixture(t, "zz-plans-meetcfg", "UTC", 1)
	ctx := context.Background()
	f.addProfile(t, f.users[0], "ja")
	walk := &matching.Activity{ID: matching.ActivityID(f.act)} // no venue categories: a walk
	w := Window{Start: time.Date(2030, 1, 7, 0, 0, 0, 0, time.UTC), Days: 14, Deadline: time.Date(2030, 1, 14, 0, 0, 0, 0, time.UTC)}
	if _, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: walk.ID, Primary: "ja"}, Audience: 6, Score: 1},
		w, map[matching.ActivityID]*matching.Activity{walk.ID: walk}); err != nil {
		t.Fatal(err)
	}
	run := func() []maps.NearbyRequest {
		t.Helper()
		if _, err := f.pool.Exec(ctx, `update app.plans set meeting_place_id = null where city_id = $1`, f.city); err != nil {
			t.Fatal(err)
		}
		search := stationSearch()
		r := &Runner{DB: f.pool, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Places: search}
		if _, err := r.RefreshMeetingPlaces(ctx, f.city); err != nil {
			t.Fatal(err)
		}
		return search.NearbyRequests()
	}
	// stationSearch finds only train stations, so the searches before one run out.
	got := run()
	if len(got) != 1 || got[0].RadiusM != 1000 || got[0].Types[0] != "train_station" {
		t.Errorf("default searches = %+v; want the station first (and, finding one, only it)", got)
	}
	if _, err := f.pool.Exec(ctx, `update app.cities set meeting_places = '[{"types": ["park"], "radius_m": 700}, {"types": ["cafe"], "radius_m": 400}, {"types": ["train_station"], "radius_m": 300}]' where id = $1`, f.city); err != nil {
		t.Fatal(err)
	}
	got = run()
	if len(got) != 3 || got[0].Types[0] != "park" || got[0].RadiusM != 700 || got[1].Types[0] != "cafe" || got[2].RadiusM != 300 {
		t.Errorf("city searches = %+v; want park 700, cafe 400, station 300 in that order", got)
	}
}
