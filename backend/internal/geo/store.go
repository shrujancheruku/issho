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

// SaveMatrix upserts travel times and then deletes the city's rows older than
// MaxCacheAge, in one transaction. Returns the number of stale rows deleted.
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
		n, err := q.DeleteStaleTravelTimes(ctx, dbgen.DeleteStaleTravelTimesParams{
			CityID: cityID, Cutoff: pgtype.Timestamptz{Time: time.Now().Add(-MaxCacheAge), Valid: true},
		})
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
