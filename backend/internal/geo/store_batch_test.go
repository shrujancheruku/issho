package geo

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// statementCounter is a pgx tracer that counts the statements a pool sends.
type statementCounter struct{ n atomic.Int64 }

func (c *statementCounter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}

func (c *statementCounter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// TestIntegrationSaveBatches: SaveMatrix and SaveVenues send a constant
// number of statements however many rows they save (#185). A city of 50
// areas has 2,450 walking pairs; one statement per pair made a rebuild thousands
// of round trips to a database that is far from the worker.
func TestIntegrationSaveBatches(t *testing.T) {
	ctx := context.Background()
	// A second pool on the same database, with the counting tracer.
	cfg := testfixture.Pool(t).Config()
	counter := &statementCounter{}
	cfg.ConnConfig.Tracer = counter
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := dbgen.New(pool)

	const city = "zz-geo-batch"
	cleanup := func() {
		if _, err := pool.Exec(ctx, "delete from app.cities where id = $1", city); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	if err := q.UpsertCity(ctx, dbgen.UpsertCityParams{
		ID: city, Name: []byte(`{"en":"Test"}`), Timezone: "UTC", Currency: "USD",
		Languages: []string{"en"}, TravelModes: []string{"walk"}, Matching: []byte("{}"), MeetingPlaces: []byte("[]"),
	}); err != nil {
		t.Fatal(err)
	}
	const nAreas = 50
	ids := make([]string, nAreas)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s-a%02d", city, i)
		if err := q.UpsertArea(ctx, dbgen.UpsertAreaParams{ID: ids[i], CityID: city, Name: []byte(`{"en":"A"}`), Lat: 35 + float64(i)/100, Lng: 139}); err != nil {
			t.Fatal(err)
		}
	}

	var rows []TravelTime
	for _, from := range ids {
		for _, to := range ids {
			if from != to {
				rows = append(rows, TravelTime{From: from, To: to, Mode: maps.Walk, Minutes: 12})
			}
		}
	}
	statements := func(save func()) int64 {
		before := counter.n.Load()
		save()
		return counter.n.Load() - before
	}
	save := func(rows []TravelTime) func() {
		return func() {
			if _, err := SaveMatrix(ctx, pool, city, rows); err != nil {
				t.Fatal(err)
			}
		}
	}
	small := statements(save(rows[:10]))
	full := statements(save(rows))
	if full != small {
		t.Errorf("SaveMatrix sent %d statements for %d rows but %d for 10: not constant", full, len(rows), small)
	}
	// BEGIN, the upsert, the delete of rows not refreshed, COMMIT.
	if full > 4 {
		t.Errorf("SaveMatrix sent %d statements, want at most 4", full)
	}
	var stored int
	if err := pool.QueryRow(ctx, `select count(*) from app.area_travel_times t join app.areas a on a.id = t.from_area_id
		where a.city_id = $1`, city).Scan(&stored); err != nil || stored != len(rows) {
		t.Errorf("stored %d travel times, want %d (%v)", stored, len(rows), err)
	}

	// A pair listed twice keeps its last row, and a row not in the rebuild goes.
	dup := []TravelTime{
		{From: ids[0], To: ids[1], Mode: maps.Walk, Minutes: 5},
		{From: ids[0], To: ids[1], Mode: maps.Walk, Minutes: 7},
	}
	deleted, err := SaveMatrix(ctx, pool, city, dup)
	if err != nil || deleted != int64(len(rows)-1) {
		t.Errorf("SaveMatrix(dup) deleted %d, %v; want %d", deleted, err, len(rows)-1)
	}
	var minutes int32
	if err := pool.QueryRow(ctx, "select minutes from app.area_travel_times where from_area_id = $1 and to_area_id = $2",
		ids[0], ids[1]).Scan(&minutes); err != nil || minutes != 7 {
		t.Errorf("duplicate pair kept %d minutes (%v), want the last, 7", minutes, err)
	}

	var venues []Venue
	for i := range 300 {
		venues = append(venues, Venue{PlaceID: fmt.Sprintf("zz-batch-%03d", i), AreaID: ids[i%nAreas], CategoryID: "cafe"})
	}
	venues = append(venues, venues[0]) // listed twice
	small = statements(func() {
		if err := SaveVenues(ctx, pool, city, venues[:5]); err != nil {
			t.Fatal(err)
		}
	})
	full = statements(func() {
		if err := SaveVenues(ctx, pool, city, venues); err != nil {
			t.Fatal(err)
		}
	})
	if full != small || full != 1 {
		t.Errorf("SaveVenues sent %d statements for %d venues and %d for 5, want 1 each", full, len(venues), small)
	}
	var n int
	if err := pool.QueryRow(ctx, "select count(*) from app.venues where city_id = $1", city).Scan(&n); err != nil || n != 300 {
		t.Errorf("stored %d venues, want 300 (%v)", n, err)
	}
	if err := SaveVenues(ctx, pool, city, nil); err != nil {
		t.Errorf("SaveVenues(nil) = %v", err)
	}
}
