package plans

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/platform/pgconv"
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

// RouteReport counts what a routes pass did.
type RouteReport struct {
	Computed, Cleared int
	Failed            int // plans whose route could not be computed or saved
}

// MeetingReport counts what a meeting-places pass did.
type MeetingReport struct {
	Found  int
	Failed int // plans whose meeting place could not be found or saved
}

// RefreshRoutes keeps a city's plan routes within the Maps caching terms:
// it clears the city's routes older than RouteMaxAge (a walk's legs too),
// then (with a finder) computes routes that are missing or older than
// RouteRefreshAge for plans still open or with an event to come. One failed route is
// counted in Failed and doesn't stop the others (the policy on Match).
func (r *Runner) RefreshRoutes(ctx context.Context, cityID string, now time.Time) (RouteReport, error) {
	var rep RouteReport
	q := dbgen.New(r.DB)
	n, err := q.ClearStaleRoutes(ctx, dbgen.ClearStaleRoutesParams{CityID: cityID, Cutoff: pgtype.Timestamptz{Time: now.Add(-RouteMaxAge), Valid: true}})
	if err != nil {
		return rep, err
	}
	rep.Cleared = int(n)
	var errs []error
	if r.WalkRoutes != nil {
		computed, failed, err := r.refreshWalkRoutes(ctx, q, cityID, now)
		rep.Computed += computed
		rep.Failed += failed
		if err != nil {
			errs = append(errs, err)
		}
	}
	if r.Routes == nil {
		return rep, errors.Join(errs...)
	}
	plans, err := q.ListPlansNeedingRoute(ctx, dbgen.ListPlansNeedingRouteParams{
		CityID: cityID, OlderThan: pgtype.Timestamptz{Time: now.Add(-RouteRefreshAge), Valid: true}, Max: routeBatch,
	})
	if err != nil {
		return rep, errors.Join(append(errs, err)...)
	}
	for _, p := range plans {
		// Cancelled (timeout, shutdown): stop, don't count the rest as failed.
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		route, err := r.Routes.WalkingRoute(ctx, p.FromPlace, pgconv.Deref(p.FollowUpVenuePlaceID))
		if err == nil {
			err = q.SetPlanRoute(ctx, dbgen.SetPlanRouteParams{
				ID: p.ID, Polyline: &route.Polyline, Seconds: ptrInt32(int32(route.Duration.Seconds())),
				At: pgtype.Timestamptz{Time: now, Valid: true},
			})
		}
		if err != nil {
			rep.Failed++
			errs = append(errs, fmt.Errorf("plan %s: %w", p.ID, err))
			r.itemFailed(ctx, "routes", cityID, "plan", p.ID, err)
			continue
		}
		rep.Computed++
	}
	return rep, errors.Join(errs...)
}

func ptrInt32(v int32) *int32 { return &v }

// RefreshMeetingPlaces finds a meeting place for a city's plans whose first
// activity has no venue (a photo walk): one Nearby Search per plan, once.
// Only the place id is stored. Without a searcher (no Maps key) it does
// nothing; the app then says the meeting point is shared later.
func (r *Runner) RefreshMeetingPlaces(ctx context.Context, cityID string) (MeetingReport, error) {
	var rep MeetingReport
	if r.Places == nil {
		return rep, nil
	}
	q := dbgen.New(r.DB)
	// The city's order of searches (its pack's meeting_places), or the
	// station-then-park default.
	raw, err := q.GetCityMeetingPlaces(ctx, cityID)
	if err != nil {
		return rep, fmt.Errorf("city %q: %w", cityID, err)
	}
	searches, err := cities.ParseMeetingPlaces(raw)
	if err != nil {
		return rep, fmt.Errorf("city %q: %w", cityID, err)
	}
	plans, err := q.ListPlansNeedingMeetingPlace(ctx, dbgen.ListPlansNeedingMeetingPlaceParams{CityID: cityID, Max: routeBatch})
	if err != nil {
		return rep, err
	}
	var errs []error
	for _, p := range plans {
		// Cancelled (timeout, shutdown): stop, don't count the rest as failed.
		if err := ctx.Err(); err != nil {
			return rep, err
		}
		var place string
		var searchErr error
		for _, s := range searches {
			got, err := r.Places.Nearby(ctx, maps.NearbyRequest{
				Center: maps.LatLng{Lat: p.Lat, Lng: p.Lng}, RadiusM: float64(s.RadiusM), Types: s.Types, Max: 1,
			})
			if err != nil {
				searchErr = err
				break
			}
			if len(got) > 0 {
				place = got[0].ID
				break
			}
		}
		if searchErr == nil && place != "" {
			searchErr = q.SetMeetingPlace(ctx, dbgen.SetMeetingPlaceParams{ID: p.ID, PlaceID: &place})
		}
		if searchErr != nil {
			rep.Failed++
			errs = append(errs, fmt.Errorf("plan %s: %w", p.ID, searchErr))
			r.itemFailed(ctx, "meeting places", cityID, "plan", p.ID, searchErr)
			continue
		}
		if place != "" {
			rep.Found++
		}
	}
	return rep, errors.Join(errs...)
}
