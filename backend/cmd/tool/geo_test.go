package main

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/maps/mapstest"
	"github.com/shrujancheruku/issho/backend/internal/platform/config"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// useFakeMaps makes build-matrix and discover-venues use f instead of Google.
func useFakeMaps(t *testing.T, f *mapstest.Fake) {
	t.Helper()
	old := mapsClient
	mapsClient = func(string) interface {
		maps.Router
		maps.PlaceSearcher
	} {
		return f
	}
	t.Cleanup(func() { mapsClient = old })
}

func count(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// geoCity is a throwaway city of two walkable areas whose only activity needs
// one venue category (Google type "cafe").
func geoCity(t *testing.T) *testfixture.Fixture {
	t.Helper()
	ctx := context.Background()
	f := testfixture.New(t, "zz-tool-geo", "UTC", 0)
	areaB := f.City + "-b"
	if err := f.Q.UpsertArea(ctx, dbgen.UpsertAreaParams{ID: areaB, CityID: f.City, Name: []byte(`{"en":"B"}`), Lat: 0.005, Lng: 0}); err != nil {
		t.Fatal(err)
	}
	const cat = "zz-tool-cafe"
	for _, sql := range []string{
		`insert into app.venue_categories (id, name, google_types) values ('` + cat + `', '{"en":"Cafe"}', '{cafe}')
		 on conflict (id) do nothing`,
		`update app.activities set venue_categories = '{` + cat + `}' where id = '` + f.Act + `'`,
	} {
		if _, err := f.Pool.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	// Registered after the fixture's cleanups, so it runs before them.
	t.Cleanup(func() {
		if _, err := f.Pool.Exec(ctx, "delete from app.venue_categories where id = $1", cat); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	return f
}

func TestIntegrationBuildMatrix(t *testing.T) {
	f := geoCity(t)
	ctx := context.Background()
	fake := &mapstest.Fake{}
	useFakeMaps(t, fake)
	rows := func() int {
		return count(t, f.Pool, "select count(*) from app.area_travel_times where from_area_id like $1", f.City+"%")
	}

	// -dry-run needs no key, writes nothing and makes no Google call.
	useToolCfg(t, config.Tool{})
	if err := buildMatrix(ctx, f.Pool, []string{"-dry-run", f.City}); err != nil {
		t.Fatal(err)
	}
	if rows() != 0 || fake.Matrices() != 0 {
		t.Errorf("dry run: %d rows, %d Google calls; want none", rows(), fake.Matrices())
	}

	// Refusals write nothing: no key, an unseeded city, no city.
	if err := buildMatrix(ctx, f.Pool, []string{f.City}); err == nil {
		t.Error("no GOOGLE_MAPS_API_KEY: want an error")
	}
	useToolCfg(t, config.Tool{GoogleMapsAPIKey: "test-key"})
	if err := buildMatrix(ctx, f.Pool, []string{"zz-tool-nowhere"}); !errors.Is(err, geo.ErrCityNotFound) {
		t.Errorf("unseeded city: err = %v; want ErrCityNotFound", err)
	}
	if err := buildMatrix(ctx, f.Pool, nil); err == nil {
		t.Error("no city: want a usage error")
	}
	if rows() != 0 || fake.Matrices() != 0 {
		t.Errorf("refusals: %d rows, %d Google calls; want none", rows(), fake.Matrices())
	}

	// An apply writes both directions, and a second run changes nothing.
	if err := buildMatrix(ctx, f.Pool, []string{f.City}); err != nil {
		t.Fatal(err)
	}
	first := rows()
	if first < 2 || fake.Matrices() == 0 {
		t.Fatalf("apply: %d rows, %d Google calls; want rows for both areas", first, fake.Matrices())
	}
	var minutes int32
	if err := f.Pool.QueryRow(ctx, "select minutes from app.area_travel_times where from_area_id = $1 and to_area_id = $2",
		f.Area, f.City+"-b").Scan(&minutes); err != nil || minutes != 10 {
		t.Errorf("a→b = %d minutes, %v; want the fake's 10", minutes, err)
	}
	if err := buildMatrix(ctx, f.Pool, []string{f.City}); err != nil {
		t.Fatal(err)
	}
	if rows() != first {
		t.Errorf("second run: %d rows, want %d", rows(), first)
	}

	// A failing Google writes nothing and keeps what is there.
	fake.MatrixErr = errors.New("quota")
	if err := buildMatrix(ctx, f.Pool, []string{f.City}); err == nil {
		t.Error("Google failure: want an error")
	}
	if rows() != first {
		t.Errorf("after a Google failure: %d rows, want %d kept", rows(), first)
	}
}

func TestIntegrationDiscoverVenues(t *testing.T) {
	f := geoCity(t)
	ctx := context.Background()
	// One café, found from every search, next to the first area.
	fake := &mapstest.Fake{NearbyFunc: func(maps.NearbyRequest) []maps.Place {
		return []maps.Place{{ID: "zz-tool-place", PrimaryType: "cafe", Location: maps.LatLng{Lat: 0.0001, Lng: 0}}}
	}}
	useFakeMaps(t, fake)
	venues := func() int {
		return count(t, f.Pool, "select count(*) from app.venues where city_id = $1", f.City)
	}

	// -dry-run needs no key, writes nothing and makes no Google call.
	useToolCfg(t, config.Tool{})
	if err := discoverVenues(ctx, f.Pool, []string{"-dry-run", "-categories", "zz-tool-cafe", f.City}); err != nil {
		t.Fatal(err)
	}
	if venues() != 0 || fake.Nearbys() != 0 {
		t.Errorf("dry run: %d venues, %d Google calls; want none", venues(), fake.Nearbys())
	}

	// Refusals write nothing: no key, an unknown category, an unseeded city.
	if err := discoverVenues(ctx, f.Pool, []string{"-categories", "zz-tool-cafe", f.City}); err == nil {
		t.Error("no GOOGLE_MAPS_API_KEY: want an error")
	}
	useToolCfg(t, config.Tool{GoogleMapsAPIKey: "test-key"})
	if err := discoverVenues(ctx, f.Pool, []string{"-categories", "zz-tool-nope", f.City}); err == nil {
		t.Error("unknown category: want an error")
	}
	if err := discoverVenues(ctx, f.Pool, []string{"zz-tool-nowhere"}); !errors.Is(err, geo.ErrCityNotFound) {
		t.Errorf("unseeded city: err = %v; want ErrCityNotFound", err)
	}
	// A budget below the first searches is refused before anything is saved.
	if err := discoverVenues(ctx, f.Pool, []string{"-categories", "zz-tool-cafe", "-max-requests", "0", f.City}); err == nil {
		t.Error("budget too small: want an error")
	}
	if venues() != 0 || fake.Nearbys() != 0 {
		t.Errorf("refusals: %d venues, %d Google calls; want none", venues(), fake.Nearbys())
	}

	// An apply stores the place once, as a candidate in its area.
	if err := discoverVenues(ctx, f.Pool, []string{"-categories", "zz-tool-cafe", f.City}); err != nil {
		t.Fatal(err)
	}
	var area, status string
	if err := f.Pool.QueryRow(ctx, "select area_id, status from app.venues where city_id = $1 and google_place_id = 'zz-tool-place'", f.City).
		Scan(&area, &status); err != nil || area != f.Area || status != "candidate" {
		t.Fatalf("venue: area %q status %q err %v; want the first area, candidate", area, status, err)
	}
	if venues() != 1 {
		t.Fatalf("%d venues, want 1", venues())
	}

	// A second run changes nothing, and keeps an admin's decision.
	if _, err := f.Pool.Exec(ctx, "update app.venues set status = 'approved' where city_id = $1", f.City); err != nil {
		t.Fatal(err)
	}
	if err := discoverVenues(ctx, f.Pool, []string{"-categories", "zz-tool-cafe", f.City}); err != nil {
		t.Fatal(err)
	}
	if venues() != 1 || count(t, f.Pool, "select count(*) from app.venues where city_id = $1 and status = 'approved'", f.City) != 1 {
		t.Error("second run changed the venues or lost the approval")
	}

	// A Google failure stores nothing new.
	fake.NearbyErr = errors.New("quota")
	if err := discoverVenues(ctx, f.Pool, []string{"-categories", "zz-tool-cafe", f.City}); err == nil {
		t.Error("Google failure: want an error")
	}
	if venues() != 1 {
		t.Errorf("after a Google failure: %d venues, want 1", venues())
	}
}
