package geo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// ErrCityNotFound is what LoadCity returns for a city that isn't seeded, so
// callers can tell it from a database failure.
var ErrCityNotFound = errors.New("city not found; seed its pack first")

// LoadCity reads a seeded city's areas and travel modes.
func LoadCity(ctx context.Context, q *dbgen.Queries, cityID string) ([]Area, []maps.Mode, error) {
	modes, err := q.GetCityTravelModes(ctx, cityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, fmt.Errorf("city %q: %w", cityID, ErrCityNotFound)
		}
		return nil, nil, err
	}
	rows, err := q.ListCityAreas(ctx, cityID)
	if err != nil {
		return nil, nil, err
	}
	areas := make([]Area, len(rows))
	for i, r := range rows {
		b, err := ParseBoundary(r.Boundary)
		if err != nil {
			return nil, nil, fmt.Errorf("area %s: %w", r.ID, err)
		}
		areas[i] = Area{ID: r.ID, LatLng: maps.LatLng{Lat: r.Lat, Lng: r.Lng}, Boundary: b}
	}
	ms := make([]maps.Mode, len(modes))
	for i, m := range modes {
		ms[i] = maps.Mode(m)
	}
	return areas, ms, nil
}

// ParseBoundary reads an area's boundary as the database returns it
// (st_asgeojson of a MultiPolygon; "" for none). Holes are dropped, as in
// maps.Boundary.
func ParseBoundary(geojson string) (maps.Boundary, error) {
	if geojson == "" {
		return nil, nil
	}
	var g struct {
		Type        string           `json:"type"`
		Coordinates [][][][2]float64 `json:"coordinates"`
	}
	if err := json.Unmarshal([]byte(geojson), &g); err != nil {
		return nil, fmt.Errorf("boundary is not valid GeoJSON: %w", err)
	}
	// Valid JSON of another type has no error to wrap.
	if g.Type != "MultiPolygon" {
		return nil, fmt.Errorf("boundary is %q, not a GeoJSON MultiPolygon", g.Type)
	}
	var b maps.Boundary
	for _, poly := range g.Coordinates {
		if len(poly) == 0 {
			continue
		}
		ring := make([]maps.LatLng, len(poly[0]))
		for i, c := range poly[0] {
			ring[i] = maps.LatLng{Lat: c[1], Lng: c[0]}
		}
		b = append(b, ring)
	}
	return b, nil
}

// LoadVenueCategories returns the categories needed by the city's enabled
// activities. If only is non-empty, it keeps just those ids.
func LoadVenueCategories(ctx context.Context, q *dbgen.Queries, cityID string, only []string) ([]VenueCategory, error) {
	rows, err := q.ListCityVenueCategories(ctx, cityID)
	if err != nil {
		return nil, err
	}
	keep, missing := map[string]bool{}, map[string]bool{}
	for _, id := range only {
		keep[id], missing[id] = true, true
	}
	var out []VenueCategory
	for _, r := range rows {
		if len(keep) == 0 || keep[r.ID] {
			out = append(out, VenueCategory{ID: r.ID, GoogleTypes: r.GoogleTypes, RadiusM: r.SearchRadiusM})
			delete(missing, r.ID)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("unknown or unused venue categories for %s: %v", cityID, keys(missing))
	}
	return out, nil
}

// SaveMatrix replaces a city's travel times with a full rebuild, in one
// transaction: it upserts rows and deletes every row of the city that the
// rebuild didn't write, so nothing outlives the Maps caching limit and a
// pair that lost its route goes at once. Returns the number of rows deleted.
//
// The upsert is one statement over parallel arrays however many rows there
// are (a city of 50 areas has thousands), not a round trip per row (#185).
// A pair listed twice keeps its last row, as the one-by-one upserts did.
func SaveMatrix(ctx context.Context, pool *pgxpool.Pool, cityID string, rows []TravelTime) (int64, error) {
	type pair struct {
		from, to string
		mode     maps.Mode
	}
	last := make(map[pair]int, len(rows))
	for i, r := range rows {
		last[pair{r.From, r.To, r.Mode}] = i
	}
	arg := dbgen.UpsertTravelTimesParams{
		FromAreaIds: make([]string, 0, len(last)), ToAreaIds: make([]string, 0, len(last)),
		Modes: make([]string, 0, len(last)), Minutes: make([]int32, 0, len(last)),
	}
	for i, r := range rows {
		if last[pair{r.From, r.To, r.Mode}] != i {
			continue
		}
		arg.FromAreaIds = append(arg.FromAreaIds, r.From)
		arg.ToAreaIds = append(arg.ToAreaIds, r.To)
		arg.Modes = append(arg.Modes, string(r.Mode))
		arg.Minutes = append(arg.Minutes, r.Minutes)
	}
	var deleted int64
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if err := q.UpsertTravelTimes(ctx, arg); err != nil {
			return fmt.Errorf("save travel times: %w", err)
		}
		n, err := q.DeleteTravelTimesNotRefreshed(ctx, cityID)
		deleted = n
		return err
	})
	return deleted, err
}

// SaveVenues upserts discovered venues. Existing venues keep their area and
// curation status; only last_seen_at is refreshed. One statement over
// parallel arrays, not a round trip per venue (#185); a (place, category)
// listed twice counts once, the first listing's area winning as it did.
func SaveVenues(ctx context.Context, pool *pgxpool.Pool, cityID string, venues []Venue) error {
	type key struct{ place, category string }
	seen := make(map[key]bool, len(venues))
	arg := dbgen.UpsertVenuesParams{CityID: cityID}
	for _, v := range venues {
		if k := (key{v.PlaceID, v.CategoryID}); !seen[k] {
			seen[k] = true
			arg.GooglePlaceIds = append(arg.GooglePlaceIds, v.PlaceID)
			arg.AreaIds = append(arg.AreaIds, v.AreaID)
			arg.CategoryIds = append(arg.CategoryIds, v.CategoryID)
		}
	}
	if len(arg.GooglePlaceIds) == 0 {
		return nil
	}
	if err := dbgen.New(pool).UpsertVenues(ctx, arg); err != nil {
		return fmt.Errorf("save venues: %w", err)
	}
	return nil
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ErrNoRouter is Refresh without a Maps router.
var ErrNoRouter = errors.New("geo: no Maps router to rebuild the travel times")

// RefreshAge is when a city's travel times are rebuilt: well inside
// Google's 30-day limit on caching Routes content, so a failed run has days to be retried.
const RefreshAge = 25 * 24 * time.Hour

// Refresh rebuilds a city's travel-time matrix when its newest row is older
// than RefreshAge (or it has none), as the Google Maps caching terms
// require. Without a router (no Maps key) it returns ErrNoRouter and touches
// nothing: a missing key never deletes data, and rebuilding is the fix
// (production can't start without the key). It reports whether it rebuilt and how many rows
// it deleted.
func Refresh(ctx context.Context, pool *pgxpool.Pool, r maps.Router, cityID string, now time.Time) (rebuilt bool, deleted int64, err error) {
	q := dbgen.New(pool)
	if r == nil {
		return false, 0, ErrNoRouter
	}
	newest, err := q.NewestTravelTime(ctx, cityID)
	if err != nil {
		return false, 0, err
	}
	if newest.Valid && now.Sub(newest.Time) < RefreshAge {
		return false, 0, nil
	}
	_, deleted, err = RebuildMatrix(ctx, pool, r, cityID)
	return err == nil, deleted, err
}

// RebuildMatrix loads a seeded city, asks the router for every pair's
// travel time and saves them (SaveMatrix). It is the one place that does
// these three steps: the admin SPA's apply, the worker's refresh and
// `tool build-matrix` all call it. It returns the rows written and the
// stale rows deleted.
func RebuildMatrix(ctx context.Context, pool *pgxpool.Pool, r maps.Router, cityID string) (rows int, deleted int64, err error) {
	areas, modes, err := LoadCity(ctx, dbgen.New(pool), cityID)
	if err != nil {
		return 0, 0, err
	}
	built, err := BuildMatrix(ctx, r, areas, modes)
	if err != nil {
		return 0, 0, err
	}
	deleted, err = SaveMatrix(ctx, pool, cityID, built)
	return len(built), deleted, err
}
