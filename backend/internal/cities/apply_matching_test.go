package cities

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// TestIntegrationApplyMatching: a pack's matching overlay and meeting places
// reach the city's row, removing them resets to the defaults, and the admin
// toggles (review, approved_venues_only) are never written.
func TestIntegrationApplyMatching(t *testing.T) {
	url := testfixture.DatabaseURL(t)
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	const city = "zz-apply-matching"
	cleanup := func() {
		if _, err := pool.Exec(ctx, "delete from app.cities where id = '"+city+"'"); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	pack := Pack{
		ID: city, Name: Text{"en": "Matching"}, Timezone: "Europe/Madrid", Currency: "EUR", Languages: []string{"es"}, TravelModes: []string{"walk"},
		Areas:         []Area{{ID: city + "-a", Name: Text{"en": "Alpha"}, Lat: 40.4, Lng: -3.7}},
		Matching:      []byte(`{"timing": {"block_starts": {"evening": "21:00"}}}`),
		MeetingPlaces: []MeetingPlace{{Types: []string{"cafe"}, RadiusM: 600}, {Types: []string{"park"}, RadiusM: 900}},
	}
	if err := ValidatePack(pack, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, pool, nil, pack, Options{}); err != nil {
		t.Fatal(err)
	}
	// An admin turns review on; later applies must leave it alone.
	if _, err := pool.Exec(ctx, "update app.cities set review = true, approved_venues_only = true where id = $1", city); err != nil {
		t.Fatal(err)
	}
	pack.Matching = []byte(`{"timing": {"block_starts": {"evening": "21:30"}}}`)
	if _, err := Apply(ctx, pool, nil, pack, Options{}); err != nil {
		t.Fatal(err)
	}
	var raw, places []byte
	var review, approved bool
	read := func() {
		t.Helper()
		if err := pool.QueryRow(ctx, "select matching, meeting_places, review, approved_venues_only from app.cities where id = $1", city).Scan(&raw, &places, &review, &approved); err != nil {
			t.Fatal(err)
		}
	}
	read()
	cfg, err := matching.ParseConfig(raw)
	if err != nil || cfg.Timing.BlockStarts[matching.Evening] != "21:30" {
		t.Errorf("matching = %s, %v; want the evening at 21:30", raw, err)
	}
	got, err := ParseMeetingPlaces(places)
	if err != nil || len(got) != 2 || got[0].Types[0] != "cafe" || got[0].RadiusM != 600 || got[1].Types[0] != "park" {
		t.Errorf("meeting_places = %s, %v", places, err)
	}
	if !review || !approved {
		t.Errorf("review=%v approved_venues_only=%v: Apply overwrote an admin toggle", review, approved)
	}

	// Dropping both fields from the pack puts the city back on the defaults.
	pack.Matching, pack.MeetingPlaces = nil, nil
	if _, err := Apply(ctx, pool, nil, pack, Options{}); err != nil {
		t.Fatal(err)
	}
	read()
	if cfg, err = matching.ParseConfig(raw); err != nil || cfg.Timing.BlockStarts[matching.Evening] != "18:30" {
		t.Errorf("matching after removal = %s, %v", raw, err)
	}
	if got, err = ParseMeetingPlaces(places); err != nil || len(got) != 2 || got[0].Types[0] != "train_station" {
		t.Errorf("meeting_places after removal = %s, %v; want the defaults", places, err)
	}
}
