package plans

import (
	"context"
	"io"
	"log/slog"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// fakeSearch finds a station for station searches and records them.
type fakeSearch struct{ reqs []maps.NearbyRequest }

func (f *fakeSearch) Nearby(_ context.Context, r maps.NearbyRequest) ([]maps.Place, error) {
	f.reqs = append(f.reqs, r)
	if slices.Contains(r.Types, "train_station") {
		return []maps.Place{{ID: "zz-station"}}, nil
	}
	return nil, nil
}

type recordingRoutes struct{ from, to string }

func (f *recordingRoutes) WalkingRoute(_ context.Context, from, to string) (maps.Route, error) {
	f.from, f.to = from, to
	return maps.Route{Polyline: "enc", Duration: 20 * time.Minute}, nil
}

// TestIntegrationMeetingPlace: a photo walk (no venue) ending at a café
// gets a station near the area's centre as its meeting place, once; its
// steps start there and its route runs from the station to the café (for
// the map; no walking time after the walk itself). A plan that starts at a
// venue doesn't search.
func TestIntegrationMeetingPlace(t *testing.T) {
	f := newFixture(t, "zz-plans-meet", "UTC", 1)
	ctx := context.Background()
	f.addProfile(t, f.users[0], "ja")
	const cafeAct = "zz-plans-meet-cafe"
	t.Cleanup(func() {
		// The fixture deletes the city (and its plans) after this runs.
		_, _ = f.pool.Exec(context.Background(), "delete from app.plans where follow_up_id = $1 or activity_id = $1", cafeAct)
		_, _ = f.pool.Exec(context.Background(), "delete from app.activities where id = $1", cafeAct)
	})
	for _, s := range []string{
		`insert into app.activities (id, name, category, indoor, energy, cost_level, conversational, min_group_size, max_group_size, venue_categories)
		 values ('` + cafeAct + `', '{"en":"Café"}', 'food', true, 1, 0, true, 2, 6, '{cafe}')`,
		// The fixture's area is at (0, 0); put it somewhere real.
		`update app.areas set centroid = 'POINT(139.58 35.703)' where id = '` + f.area + `'`,
		`insert into app.venues (google_place_id, city_id, area_id, category_id) values ('zz-meet-cafe', '` + f.city + `', '` + f.area + `', 'cafe')`,
	} {
		if _, err := f.pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	walk := &matching.Activity{ID: matching.ActivityID(f.act)} // no venue categories: a walk
	cafe := &matching.Activity{ID: cafeAct, VenueCategories: []matching.CategoryID{"cafe"}}
	acts := map[matching.ActivityID]*matching.Activity{walk.ID: walk, cafe.ID: cafe}
	w := Window{Start: time.Date(2030, 1, 7, 0, 0, 0, 0, time.UTC), Days: 14, Deadline: time.Date(2030, 1, 14, 0, 0, 0, 0, time.UTC)}
	planID, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: walk.ID, FollowUp: cafe.ID, Primary: "ja"}, Audience: 6, Score: 1}, w, acts)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Create(ctx, f.q, f.city, matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: cafe.ID, Primary: "ja"}, Audience: 6, Score: 1}, w, acts); err != nil {
		t.Fatal(err)
	}

	search, routes := &fakeSearch{}, &recordingRoutes{}
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Places: search, Routes: routes}
	if found, err := r.RefreshMeetingPlaces(ctx, f.city); err != nil || found != 1 || len(search.reqs) != 1 {
		t.Fatalf("found=%d searches=%d err=%v; want one station for the walk only", found, len(search.reqs), err)
	}
	if c := search.reqs[0].Center; math.Abs(c.Lat-35.703) > 1e-9 || math.Abs(c.Lng-139.58) > 1e-9 {
		t.Errorf("search centred on %v; want the area's centre", c)
	}
	if found, _ := r.RefreshMeetingPlaces(ctx, f.city); found != 0 || len(search.reqs) != 1 {
		t.Errorf("second pass searched again (%d searches)", len(search.reqs))
	}
	if _, _, err := r.RefreshRoutes(ctx, f.city, time.Now()); err != nil {
		t.Fatal(err)
	}
	if routes.from != "zz-station" || routes.to != "zz-meet-cafe" {
		t.Errorf("route %s → %s; want from the station to the café", routes.from, routes.to)
	}

	station := maps.PlaceDetails{Name: "吉祥寺駅", Location: maps.LatLng{Lat: 35.703, Lng: 139.58}}
	svc := &Service{DB: f.pool, Places: fakePlaces{"zz-station": {"ja": station}, "zz-meet-cafe": {"ja": {Name: "カフェ", Location: maps.LatLng{Lat: 35.705, Lng: 139.582}}}}}
	_, steps, _, err := svc.Plan(ctx, f.users[0], planID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 3 || !steps[0].Meeting || steps[0].Venue.Name != "吉祥寺駅" || steps[1].Activity != walk.ID || steps[2].Venue.Name != "カフェ" || steps[2].WalkMinutes != 0 {
		t.Errorf("steps = %+v; want meeting point, walk, café (no walking time after the walk)", steps)
	}
}
