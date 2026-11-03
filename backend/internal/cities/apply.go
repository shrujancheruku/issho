package cities

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	"github.com/shrujancheruku/issho/backend/internal/maps"
)

const (
	// MovedM is how far an area's centre must move for it to need venues
	// and travel times again.
	MovedM = 100
	// AreaSearchBudget caps one area's venue search from the Cities page. The
	// request waits for it, so it stays at a few minutes of searches; an area
	// with a boundary that needs more gets the rest from tool discover-venues.
	AreaSearchBudget = 400
)

// Diff is what applying a pack changed that needs Google again.
type Diff struct {
	NewAreas, MovedAreas []string
	// TravelTimesRebuilt is set when the matrix was rebuilt, with TravelTimes
	// rows written.
	TravelTimesRebuilt bool
	TravelTimes        int
}

// ErrTravelTimes wraps a router failure in Apply. The pack is already
// seeded when it happens; the worker's refresh retries the matrix.
var ErrTravelTimes = errors.New("seeded, but the travel times failed")

// DiffAreas says which of a pack's areas are new (not in was) and which
// moved more than MovedM from where was has them. It is pure so the rule
// can be tested without a database; results follow the pack's order.
func DiffAreas(was map[string]maps.LatLng, areas []Area) (added, moved []string) {
	for _, a := range areas {
		at, ok := was[a.ID]
		switch {
		case !ok:
			added = append(added, a.ID)
		case maps.DistanceM(at, maps.LatLng{Lat: a.Lat, Lng: a.Lng}) > MovedM:
			moved = append(moved, a.ID)
		}
	}
	return added, moved
}

// Apply seeds a validated pack, works out which areas are new or moved
// (compared with the database before seeding) and, when there are any and a
// router is given, rebuilds the city's travel times. A nil router (no Maps
// key) skips the rebuild. If the router fails, the pack stays seeded and the
// error wraps ErrTravelTimes; the Diff is still returned.
func Apply(ctx context.Context, pool *pgxpool.Pool, router maps.Router, p Pack) (Diff, error) {
	before, err := dbgen.New(pool).ListCityAreas(ctx, p.ID)
	if err != nil {
		return Diff{}, err
	}
	was := make(map[string]maps.LatLng, len(before))
	for _, a := range before {
		was[a.ID] = maps.LatLng{Lat: a.Lat, Lng: a.Lng}
	}
	if err := seedPack(ctx, pool, p); err != nil {
		return Diff{}, err
	}
	var d Diff
	d.NewAreas, d.MovedAreas = DiffAreas(was, p.Areas)
	if (len(d.NewAreas) == 0 && len(d.MovedAreas) == 0) || router == nil {
		return d, nil
	}
	rows, _, err := geo.RebuildMatrix(ctx, pool, router, p.ID)
	if err != nil {
		// A database failure after the router answered lands here too;
		// telling them apart isn't worth it, as the retry is the same.
		return d, fmt.Errorf("%w: %w", ErrTravelTimes, err)
	}
	d.TravelTimesRebuilt, d.TravelTimes = true, rows
	return d, nil
}

// ErrAreaNotInCity is DiscoverAreaVenues' error for an area the city doesn't have.
var ErrAreaNotInCity = errors.New("area not in that city")

// ErrSearch wraps a failure of the place-search provider in DiscoverAreaVenues.
var ErrSearch = errors.New("google search failed")

// DiscoverAreaVenues finds one area's venues with Nearby Search, within
// AreaSearchBudget, and saves them. A new city has no city-wide venues yet
// (beaches, trails), so those are found with its first area. It returns the
// searches made and the venues found. geo.ErrCityNotFound and
// ErrAreaNotInCity say what's wrong with the request.
func DiscoverAreaVenues(ctx context.Context, pool *pgxpool.Pool, places maps.PlaceSearcher, cityID, areaID string) (searches, venues int, err error) {
	q := dbgen.New(pool)
	areas, _, err := geo.LoadCity(ctx, q, cityID)
	if err != nil {
		return 0, 0, err
	}
	if !slices.ContainsFunc(areas, func(a geo.Area) bool { return a.ID == areaID }) {
		return 0, 0, ErrAreaNotInCity
	}
	cats, err := geo.LoadVenueCategories(ctx, q, cityID, nil)
	if err != nil {
		return 0, 0, err
	}
	n, err := q.CountCityVenues(ctx, cityID)
	if err != nil {
		return 0, 0, err
	}
	langs, err := q.GetCityLanguages(ctx, cityID)
	if err != nil {
		return 0, 0, err
	}
	if len(langs) == 0 {
		return 0, 0, fmt.Errorf("city %s has no languages", cityID)
	}
	found, made, err := geo.DiscoverAreaVenues(ctx, places, areas, areaID, cats, n == 0, langs[0], AreaSearchBudget)
	if err != nil {
		return made.Made, 0, fmt.Errorf("%w: %w", ErrSearch, err)
	}
	if made.Skipped > 0 {
		slog.WarnContext(ctx, "area venue search ran out of budget; run tool discover-venues for finer coverage",
			"area", areaID, "searches", made.Made, "skipped", made.Skipped)
	}
	if err := geo.SaveVenues(ctx, pool, cityID, found); err != nil {
		return made.Made, 0, err
	}
	return made.Made, len(found), nil
}

// PackStatus is a city as the admin SPA's list shows it: what the database
// has, and which areas still lack Google's data.
type PackStatus struct {
	ID, Name                                    string
	Areas                                       int
	AreasWithoutTravelTimes, AreasWithoutVenues []string
}

// PackStatuses lists every seeded city, by id.
func PackStatuses(ctx context.Context, pool *pgxpool.Pool) ([]PackStatus, error) {
	q := dbgen.New(pool)
	rows, err := q.ListCities(ctx)
	if err != nil {
		return nil, err
	}
	areas, err := q.ListCityPackAreas(ctx)
	if err != nil {
		return nil, err
	}
	var out []PackStatus
	byID := map[string]int{}
	for _, r := range rows {
		s := PackStatus{ID: r.ID, Name: r.ID}
		var name Text
		if json.Unmarshal(r.Name, &name) == nil && name["en"] != "" {
			s.Name = name["en"]
		}
		byID[r.ID] = len(out)
		out = append(out, s)
	}
	for _, a := range areas {
		i, ok := byID[a.CityID]
		if !ok {
			continue // the foreign key makes this impossible
		}
		out[i].Areas++
		if !a.HasTravelTimes {
			out[i].AreasWithoutTravelTimes = append(out[i].AreasWithoutTravelTimes, a.ID)
		}
		if !a.HasVenues {
			out[i].AreasWithoutVenues = append(out[i].AreasWithoutVenues, a.ID)
		}
	}
	return out, nil
}

// CatalogActivity is an activity of the global catalogue, by English name.
type CatalogActivity struct{ ID, Name, Category string }

// CatalogActivities lists the catalogue, for the pack editor and to check
// that a pack's overrides name real activities.
func CatalogActivities(ctx context.Context, pool *pgxpool.Pool) ([]CatalogActivity, error) {
	rows, err := dbgen.New(pool).ListCatalogActivities(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CatalogActivity, len(rows))
	for i, r := range rows {
		out[i] = CatalogActivity{ID: r.ID, Name: r.Name, Category: r.Category}
	}
	return out, nil
}
