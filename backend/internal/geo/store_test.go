package geo

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// TestIntegrationStore runs the store queries against a real, migrated and
// seeded database. It's skipped unless TEST_DATABASE_URL is set (CI sets it).
// It creates its own throwaway city and deletes it afterwards.
func TestIntegrationStore(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	// t.Cleanup, not defer: cleanups run after the test body and in reverse
	// order, so the pool must still be open when the city is deleted below.
	t.Cleanup(pool.Close)
	q := dbgen.New(pool)

	const city = "zz-geo-test"
	cleanup := func() {
		if _, err := pool.Exec(ctx, "delete from app.cities where id = $1", city); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	if err := q.UpsertCity(ctx, dbgen.UpsertCityParams{
		ID: city, Name: []byte(`{"en":"Test"}`), Timezone: "UTC", Currency: "USD",
		Languages: []string{"en"}, TravelModes: []string{"walk"},
	}); err != nil {
		t.Fatal(err)
	}
	for _, a := range areas {
		if err := q.UpsertArea(ctx, dbgen.UpsertAreaParams{ID: city + a.ID, CityID: city, Name: []byte(`{"en":"A"}`), Lat: a.Lat, Lng: a.Lng}); err != nil {
			t.Fatal(err)
		}
	}

	got, modes, err := LoadCity(ctx, q, city)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || len(modes) != 1 || modes[0] != maps.Walk {
		t.Fatalf("LoadCity = %v, %v", got, modes)
	}
	if got[0].Lat != areas[0].Lat || got[0].Lng != areas[0].Lng {
		t.Errorf("coordinates don't round-trip: got %v, want %v", got[0].LatLng, areas[0].LatLng)
	}

	a, b := got[0].ID, got[1].ID
	if _, err := SaveMatrix(ctx, pool, city, []TravelTime{{From: a, To: b, Mode: maps.Walk, Minutes: 9}, {From: b, To: a, Mode: maps.Walk, Minutes: 9}}); err != nil {
		t.Fatal(err)
	}
	// Age one row past the cache limit; the next save must delete it.
	if _, err := pool.Exec(ctx, "update app.area_travel_times set computed_at = now() - interval '31 days' where from_area_id = $1", b); err != nil {
		t.Fatal(err)
	}
	deleted, err := SaveMatrix(ctx, pool, city, []TravelTime{{From: a, To: b, Mode: maps.Walk, Minutes: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Errorf("deleted %d stale rows, want 1", deleted)
	}
	var minutes int32
	if err := pool.QueryRow(ctx, "select minutes from app.area_travel_times where from_area_id = $1 and to_area_id = $2", a, b).Scan(&minutes); err != nil || minutes != 10 {
		t.Errorf("upsert didn't update: %d, %v", minutes, err)
	}

	// The same place in two categories is two rows; re-discovery keeps curation.
	vs := []Venue{{PlaceID: "test-place", AreaID: a, CategoryID: "cafe"}, {PlaceID: "test-place", AreaID: a, CategoryID: "bar"}}
	if err := SaveVenues(ctx, pool, city, vs); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "update app.venues set status = 'approved' where google_place_id = 'test-place'"); err != nil {
		t.Fatal(err)
	}
	if err := SaveVenues(ctx, pool, city, vs[:1]); err != nil {
		t.Fatal(err)
	}
	var n, approved int
	_ = pool.QueryRow(ctx, "select count(*), count(*) filter (where status = 'approved') from app.venues where city_id = $1", city).Scan(&n, &approved)
	if n != 2 || approved != 2 {
		t.Errorf("venues: %d rows, %d approved; want 2, 2", n, approved)
	}

	cats, err := LoadVenueCategories(ctx, q, city, []string{"cafe"})
	if err != nil || len(cats) != 1 || cats[0].ID != "cafe" {
		t.Errorf("LoadVenueCategories = %v, %v", cats, err)
	}
}
