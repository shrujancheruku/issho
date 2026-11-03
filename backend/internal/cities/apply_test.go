package cities

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/geo"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/maps/mapstest"
)

func TestDiffAreas(t *testing.T) {
	was := map[string]maps.LatLng{
		"a": {Lat: 35.7, Lng: 139.6},
		"b": {Lat: 35.71, Lng: 139.61},
	}
	area := func(id string, lat, lng float64) Area { return Area{ID: id, Lat: lat, Lng: lng} }
	for _, tc := range []struct {
		name         string
		areas        []Area
		added, moved []string
	}{
		{"unchanged", []Area{area("a", 35.7, 139.6), area("b", 35.71, 139.61)}, nil, nil},
		{"a new area", []Area{area("a", 35.7, 139.6), area("c", 35.8, 139.7)}, []string{"c"}, nil},
		// 0.0005° of latitude is 55 m: under the threshold.
		{"a small move is not a move", []Area{area("a", 35.7005, 139.6)}, nil, nil},
		// 0.002° of latitude is 222 m.
		{"a real move", []Area{area("a", 35.702, 139.6), area("b", 35.71, 139.61)}, nil, []string{"a"}},
		{"new and moved keep the pack's order", []Area{area("z", 1, 1), area("b", 35.72, 139.61), area("y", 2, 2), area("a", 35.8, 139.6)}, []string{"z", "y"}, []string{"b", "a"}},
		{"an area missing from the pack is not new or moved", []Area{area("a", 35.7, 139.6)}, nil, nil},
		{"nothing known yet", []Area{area("a", 35.7, 139.6)}, []string{"a"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := was
			if tc.name == "nothing known yet" {
				w = nil
			}
			added, moved := DiffAreas(w, tc.areas)
			if !slices.Equal(added, tc.added) || !slices.Equal(moved, tc.moved) {
				t.Errorf("added %v, moved %v; want %v, %v", added, moved, tc.added, tc.moved)
			}
		})
	}
}

// TestIntegrationApply: seeding a pack and rebuilding travel times only for
// new or moved areas; a router failure leaves the pack seeded; discovering
// an area's venues. Skipped unless TEST_DATABASE_URL is set.
func TestIntegrationApply(t *testing.T) {
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

	const city, act, cat = "zz-apply", "zz-apply-act", "zz-apply-cafe"
	cleanup := func() {
		for _, s := range []string{
			"delete from app.cities where id = '" + city + "'",
			"delete from app.activities where id = '" + act + "'",
			"delete from app.venue_categories where id = '" + cat + "'",
		} {
			if _, err := pool.Exec(ctx, s); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	for _, s := range []string{
		`insert into app.venue_categories (id, name, google_types) values ('` + cat + `', '{"en":"Cafe"}', '{cafe}')`,
		`insert into app.activities (id, name, category, indoor, energy, cost_level, conversational, min_group_size, max_group_size, venue_categories)
		 values ('` + act + `', '{"en":"Coffee"}', 'food', true, 1, 1, true, 2, 6, '{` + cat + `}')`,
	} {
		if _, err := pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	fake := &mapstest.Fake{NearbyFunc: func(r maps.NearbyRequest) []maps.Place {
		return []maps.Place{{ID: "zz-apply-place-" + r.Types[0], PrimaryType: r.Types[0], Location: r.Center}}
	}}
	pack := Pack{
		ID: city, Name: Text{"en": "Apply"}, Timezone: "Asia/Tokyo", Currency: "JPY", Languages: []string{"ja"}, TravelModes: []string{"walk"},
		Areas: []Area{
			{ID: city + "-a", Name: Text{"en": "Alpha"}, Lat: 35.70, Lng: 139.60},
			{ID: city + "-b", Name: Text{"en": "Beta"}, Lat: 35.71, Lng: 139.61},
		},
	}

	// New city: both areas are new and the matrix is built.
	d, err := Apply(ctx, pool, fake, pack, Options{})
	if err != nil || len(d.NewAreas) != 2 || len(d.MovedAreas) != 0 || !d.TravelTimesRebuilt || d.TravelTimes == 0 || fake.Matrices() == 0 {
		t.Fatalf("first apply = %+v, %v (%d matrices)", d, err, fake.Matrices())
	}

	// Unchanged: nobody is called.
	calls := fake.Matrices()
	if d, err := Apply(ctx, pool, fake, pack, Options{}); err != nil || len(d.NewAreas)+len(d.MovedAreas) != 0 || d.TravelTimesRebuilt || fake.Matrices() != calls {
		t.Errorf("second apply = %+v, %v", d, err)
	}

	// One area moves 1 km, one is added; a router failure keeps the pack seeded.
	pack.Areas[1].Lat += 0.009
	pack.Areas = append(pack.Areas, Area{ID: city + "-c", Name: Text{"en": "Gamma"}, Lat: 35.72, Lng: 139.62})
	fake.MatrixErr = errors.New("quota exceeded for key AIza-secret")
	d, err = Apply(ctx, pool, fake, pack, Options{})
	if !errors.Is(err, ErrTravelTimes) || len(d.NewAreas) != 1 || len(d.MovedAreas) != 1 || d.TravelTimesRebuilt {
		t.Fatalf("failing apply = %+v, %v", d, err)
	}
	var n int
	if err := pool.QueryRow(ctx, "select count(*) from app.areas where city_id = $1", city).Scan(&n); err != nil || n != 3 {
		t.Errorf("areas after a router failure = %d, %v", n, err)
	}
	// The failed run already seeded the new positions, so a retry sees no
	// change; the worker's refresh is what rebuilds the matrix.
	fake.MatrixErr = nil
	if d, err := Apply(ctx, pool, fake, pack, Options{}); err != nil || len(d.NewAreas)+len(d.MovedAreas) != 0 {
		t.Errorf("retry = %+v, %v", d, err)
	}

	// Without a router (no Maps key) the pack is seeded and nothing is built.
	pack.Areas[0].Lat += 0.009
	if d, err := Apply(ctx, pool, nil, pack, Options{}); err != nil || len(d.MovedAreas) != 1 || d.TravelTimesRebuilt {
		t.Errorf("apply without a router = %+v, %v", d, err)
	}

	// Venues for the first area of a new city: a search per kind of place
	// its activities use, each finding one place here.
	searches, venues, err := DiscoverAreaVenues(ctx, pool, fake, city, city+"-a")
	if err != nil || searches < 1 || venues != searches || fake.Nearbys() != searches {
		t.Fatalf("discover = %d searches, %d venues, %v (%d nearby)", searches, venues, err, fake.Nearbys())
	}
	if _, _, err := DiscoverAreaVenues(ctx, pool, fake, "nowhere", "x"); !errors.Is(err, geo.ErrCityNotFound) {
		t.Errorf("an unknown city: %v", err)
	}
	if _, _, err := DiscoverAreaVenues(ctx, pool, fake, city, "zz-apply-nope"); !errors.Is(err, ErrAreaNotInCity) {
		t.Errorf("an unknown area: %v", err)
	}
	fake.NearbyErr = errors.New("boom")
	if _, _, err := DiscoverAreaVenues(ctx, pool, fake, city, city+"-b"); !errors.Is(err, ErrSearch) {
		t.Errorf("a failing search: %v", err)
	}

	statuses, err := PackStatuses(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	var got *PackStatus
	for i := range statuses {
		if statuses[i].ID == city {
			got = &statuses[i]
		}
	}
	if got == nil || got.Name != "Apply" || got.Areas != 3 || slices.Contains(got.AreasWithoutVenues, city+"-a") {
		t.Errorf("status = %+v", got)
	}
}
