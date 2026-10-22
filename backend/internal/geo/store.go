package geo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// LoadCity reads a seeded city's areas and travel modes.
func LoadCity(ctx context.Context, q *dbgen.Queries, cityID string) ([]Area, []maps.Mode, error) {
	modes, err := q.GetCityTravelModes(ctx, cityID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, fmt.Errorf("city %q not found; seed its pack first", cityID)
		}
		return nil, nil, err
	}
	rows, err := q.ListCityAreas(ctx, cityID)
	if err != nil {
		return nil, nil, err
	}
	areas := make([]Area, len(rows))
	for i, r := range rows {
		areas[i] = Area{ID: r.ID, LatLng: maps.LatLng{Lat: r.Lat, Lng: r.Lng}}
	}
	ms := make([]maps.Mode, len(modes))
	for i, m := range modes {
		ms[i] = maps.Mode(m)
	}
	return areas, ms, nil
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
func SaveMatrix(ctx context.Context, pool *pgxpool.Pool, cityID string, rows []TravelTime) (int64, error) {
	var deleted int64
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		for _, r := range rows {
			if err := q.UpsertTravelTime(ctx, dbgen.UpsertTravelTimeParams{
				FromAreaID: r.From, ToAreaID: r.To, Mode: string(r.Mode), Minutes: r.Minutes,
			}); err != nil {
				return fmt.Errorf("%s→%s %s: %w", r.From, r.To, r.Mode, err)
			}
		}
		n, err := q.DeleteTravelTimesNotRefreshed(ctx, cityID)
		deleted = n
		return err
	})
	return deleted, err
}

// SaveVenues upserts discovered venues. Existing venues keep their area,
// curation status and tags; only last_seen_at is refreshed.
func SaveVenues(ctx context.Context, pool *pgxpool.Pool, cityID string, venues []Venue) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		for _, v := range venues {
			if err := q.UpsertVenue(ctx, dbgen.UpsertVenueParams{
				GooglePlaceID: v.PlaceID, CityID: cityID, AreaID: v.AreaID, CategoryID: v.CategoryID,
			}); err != nil {
				return fmt.Errorf("venue %s: %w", v.PlaceID, err)
			}
		}
		return nil
	})
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// RefreshAge is when a city's travel times are rebuilt: well inside
// MaxCacheAge, so a failed run has days to be retried before the limit.
const RefreshAge = 25 * 24 * time.Hour

// Refresh rebuilds a city's travel-time matrix when its newest row is older
// than RefreshAge (or it has none), as the Google Maps caching terms
// require. Without a router (no Maps key) it can't rebuild, so it deletes
// rows past MaxCacheAge instead: staying within the terms matters more
// than keeping stale data. It reports whether it rebuilt and how many rows
// it deleted.
func Refresh(ctx context.Context, pool *pgxpool.Pool, r maps.Router, cityID string, now time.Time) (rebuilt bool, deleted int64, err error) {
	q := dbgen.New(pool)
	if r == nil {
		deleted, err = q.DeleteStaleTravelTimes(ctx, dbgen.DeleteStaleTravelTimesParams{
			CityID: cityID, Cutoff: pgtype.Timestamptz{Time: now.Add(-MaxCacheAge), Valid: true},
		})
		return false, deleted, err
	}
	newest, err := q.NewestTravelTime(ctx, cityID)
	if err != nil {
		return false, 0, err
	}
	if newest.Valid && now.Sub(newest.Time) < RefreshAge {
		return false, 0, nil
	}
	areas, modes, err := LoadCity(ctx, q, cityID)
	if err != nil {
		return false, 0, err
	}
	rows, err := BuildMatrix(ctx, r, areas, modes)
	if err != nil {
		return false, 0, err
	}
	deleted, err = SaveMatrix(ctx, pool, cityID, rows)
	return err == nil, deleted, err
}
