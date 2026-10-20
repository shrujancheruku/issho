// Package maps defines the provider-neutral geo interfaces Issho needs: travel
// time matrices and nearby place search. The Google implementation lives in
// maps/google; anything else (an OSM fallback, test fakes) implements the same
// interfaces so callers never depend on a vendor.
package maps

import (
	"context"
	"math"
	"time"
)

// Mode is a travel mode. Values match app.area_travel_times.mode. Issho uses
// walking and driving only (no public transport; see the ROADMAP decision log).
type Mode string

const (
	Walk  Mode = "walk"
	Drive Mode = "drive"
)

type LatLng struct {
	Lat, Lng float64
}

// MatrixRequest asks for travel times from every origin to every destination.
type MatrixRequest struct {
	Origins      []LatLng
	Destinations []LatLng
	Mode         Mode
}

// MatrixElement is one origin→destination result. OK is false when the
// provider found no route (e.g. across water with no road); Duration is then zero.
type MatrixElement struct {
	Origin, Destination int // indexes into the request slices
	Duration            time.Duration
	OK                  bool
}

type Router interface {
	// Matrix returns one element per origin/destination pair, in any order.
	// Implementations split large requests to respect provider limits.
	Matrix(ctx context.Context, req MatrixRequest) ([]MatrixElement, error)
}

// NearbyRequest searches a circle for places of the given provider types.
type NearbyRequest struct {
	Center   LatLng
	RadiusM  float64
	Types    []string // provider place types, e.g. "cafe"
	Max      int      // provider caps apply (Google: 20)
	Language string   // BCP 47; affects ranking only, we store no names
}

// Place is the minimum we ask for about a place. Only ID may be stored (see the
// Google Maps caching terms); Location is for in-memory use such as picking
// the nearest area, and must not be persisted. Names and addresses are
// fetched live when shown.
type Place struct {
	ID          string
	PrimaryType string
	Types       []string
	Location    LatLng
}

type PlaceSearcher interface {
	Nearby(ctx context.Context, req NearbyRequest) ([]Place, error)
}

// DistanceM is the great-circle (haversine) distance in metres. Used to skip
// provider calls that can't be useful, e.g. walking between areas 20 km apart.
func DistanceM(a, b LatLng) float64 {
	const earthRadiusM = 6371000
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat, dLng := rad(b.Lat-a.Lat), rad(b.Lng-a.Lng)
	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(a.Lat))*math.Cos(rad(b.Lat))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * earthRadiusM * math.Asin(math.Sqrt(h))
}
