package plans

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
)

const (
	// RouteRefreshAge is when a plan's walking route is recomputed, and
	// RouteMaxAge when it's cleared: Routes content may be cached for at
	// most 30 days (Maps terms).
	RouteRefreshAge = 25 * 24 * time.Hour
	RouteMaxAge     = 30 * 24 * time.Hour
	// routeBatch caps the Routes calls in one pass.
	routeBatch = 200
)

// Route is a plan's walk from its activity's venue to its follow-up's.
type Route struct {
	Polyline string // Google's encoded polyline
	Seconds  int
}

func routeFrom(polyline *string, seconds *int32) *Route {
	if polyline == nil || seconds == nil {
		return nil
	}
	return &Route{Polyline: *polyline, Seconds: int(*seconds)}
}

// Minutes is the walk rounded up to whole minutes, at least 1.
func (r *Route) Minutes() int { return max(1, int(math.Ceil(float64(r.Seconds)/60))) }

// RefreshRoutes keeps a city's plan routes within the Maps caching terms:
// it clears the city's routes older than RouteMaxAge (a walk's legs too),
// then (with a finder) computes routes that are missing or older than
// RouteRefreshAge for plans still open or with an event to come. One failed route doesn't stop the
// others.
func (r *Runner) RefreshRoutes(ctx context.Context, cityID string, now time.Time) (computed, cleared int, err error) {
	q := dbgen.New(r.DB)
	n, err := q.ClearStaleRoutes(ctx, dbgen.ClearStaleRoutesParams{CityID: cityID, Cutoff: pgtype.Timestamptz{Time: now.Add(-RouteMaxAge), Valid: true}})
	if err != nil {
		return 0, 0, err
	}
	cleared = int(n)
	var errs []error
	if r.WalkRoutes != nil {
		n, err := r.refreshWalkRoutes(ctx, q, cityID, now)
		computed += n
		if err != nil {
			errs = append(errs, err)
		}
	}
	if r.Routes == nil {
		return computed, cleared, errors.Join(errs...)
	}
	plans, err := q.ListPlansNeedingRoute(ctx, dbgen.ListPlansNeedingRouteParams{
		CityID: cityID, OlderThan: pgtype.Timestamptz{Time: now.Add(-RouteRefreshAge), Valid: true}, Max: routeBatch,
	})
	if err != nil {
		return computed, cleared, err
	}
	for _, p := range plans {
		route, err := r.Routes.WalkingRoute(ctx, p.FromPlace, deref(p.FollowUpVenuePlaceID))
		if err != nil {
			errs = append(errs, fmt.Errorf("plan %s: %w", p.ID, err))
			continue
		}
		if err := q.SetPlanRoute(ctx, dbgen.SetPlanRouteParams{
			ID: p.ID, Polyline: &route.Polyline, Seconds: ptrInt32(int32(route.Duration.Seconds())),
			At: pgtype.Timestamptz{Time: now, Valid: true},
		}); err != nil {
			return computed, cleared, err
		}
		computed++
	}
	return computed, cleared, errors.Join(errs...)
}

func ptrInt32(v int32) *int32 { return &v }

// Meeting places are searched for near the area's centre: a station first
// (in Tokyo every area is named after one), then a park.
var meetingSearches = []maps.NearbyRequest{
	{RadiusM: 1000, Types: []string{"train_station", "subway_station"}, Max: 1},
	{RadiusM: 1500, Types: []string{"park"}, Max: 1},
}

// RefreshMeetingPlaces finds a meeting place for a city's plans whose first
// activity has no venue (a photo walk): one Nearby Search per plan, once.
// Only the place id is stored. Without a searcher (no Maps key) it does
// nothing; the app then says the meeting point is shared later.
func (r *Runner) RefreshMeetingPlaces(ctx context.Context, cityID string) (found int, err error) {
	if r.Places == nil {
		return 0, nil
	}
	q := dbgen.New(r.DB)
	plans, err := q.ListPlansNeedingMeetingPlace(ctx, dbgen.ListPlansNeedingMeetingPlaceParams{CityID: cityID, Max: routeBatch})
	if err != nil {
		return 0, err
	}
	var errs []error
	for _, p := range plans {
		var place string
		for _, s := range meetingSearches {
			s.Center = maps.LatLng{Lat: p.Lat, Lng: p.Lng}
			got, err := r.Places.Nearby(ctx, s)
			if err != nil {
				errs = append(errs, fmt.Errorf("plan %s: %w", p.ID, err))
				break
			}
			if len(got) > 0 {
				place = got[0].ID
				break
			}
		}
		if place == "" {
			continue
		}
		if err := q.SetMeetingPlace(ctx, dbgen.SetMeetingPlaceParams{ID: p.ID, PlaceID: &place}); err != nil {
			return found, err
		}
		found++
	}
	return found, errors.Join(errs...)
}
