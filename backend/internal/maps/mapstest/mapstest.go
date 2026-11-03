// Package mapstest is an in-memory maps provider for tests: one Fake that
// implements every maps interface the backend uses, so tests share a fake
// instead of each growing its own (a new fake goes here, not in a test file).
// Tests must never call the paid Google APIs.
package mapstest

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// Fake implements maps.Router, maps.PlaceSearcher, maps.PlaceSearch (and so
// maps.PlaceLookup), maps.PlaceFinder, maps.RouteFinder, maps.WalkRouter and
// maps.HoursLookup. Set its fields before use; the call counters and recorded
// requests are safe to read after the calls return. Errors are returned by
// every call of that kind. A *Func field, when set, replaces that call's
// default answer.
type Fake struct {
	// TravelTime is every origin-destination pair's duration (default 10 minutes).
	TravelTime time.Duration
	// Places are the known places by id, for Details, Locate and Search.
	Places map[string]maps.SearchResult
	// Found is what Search returns near a point, as place ids in order
	// (matched on SearchRequest.Near exactly).
	Found map[maps.LatLng][]string
	// NearbyFunc answers Nearby; nil finds nothing.
	NearbyFunc func(maps.NearbyRequest) []maps.Place
	// MatrixFunc answers Matrix; nil gives every pair TravelTime.
	MatrixFunc func(maps.MatrixRequest) []maps.MatrixElement
	// Localized answers Details per place id and language; checked before
	// Places. A place or language missing from both is an error.
	Localized map[string]map[string]maps.PlaceDetails
	// SearchFunc answers Search; nil looks Found up by SearchRequest.Near.
	SearchFunc func(maps.SearchRequest) []maps.SearchResult
	// FindFunc answers Find (ok false = nothing matched); nil finds nothing.
	FindFunc func(maps.FindRequest) (maps.Found, bool)
	// RouteFunc answers WalkingRoute; nil gives a 10-minute route whose
	// polyline names its ends.
	RouteFunc func(from, to string) maps.Route
	// WalkFunc answers WalkThrough; nil walks the places in order (see Legs).
	WalkFunc func(places []string, optimize bool) (maps.WalkRoute, error)
	// Legs overrides single legs of the default WalkThrough, by the leg's
	// destination place.
	Legs map[string]time.Duration
	// LegDuration is a default leg's duration (default 10 minutes).
	LegDuration time.Duration
	// ReverseOptimized makes the default WalkThrough reverse the places in
	// between when asked to optimize, to stand for the provider reordering.
	ReverseOptimized bool
	// HoursFunc answers Hours for a place id; nil is unknown hours.
	HoursFunc func(id string) maps.Hours

	MatrixErr, NearbyErr, SearchErr, FindErr, RouteErr, WalkErr, HoursErr, DetailsErr error

	mu                                          sync.Mutex
	matrices, nearbys, searches, details, hours int
	nearbyReqs                                  []maps.NearbyRequest
	findQueries                                 []string
	routeCalls                                  [][2]string
	walkCalls                                   [][]string
}

var (
	_ maps.Router        = (*Fake)(nil)
	_ maps.PlaceSearcher = (*Fake)(nil)
	_ maps.PlaceSearch   = (*Fake)(nil)
	_ maps.PlaceFinder   = (*Fake)(nil)
	_ maps.RouteFinder   = (*Fake)(nil)
	_ maps.WalkRouter    = (*Fake)(nil)
	_ maps.HoursLookup   = (*Fake)(nil)
)

func (f *Fake) count(n *int) {
	f.mu.Lock()
	*n++
	f.mu.Unlock()
}

// Matrices counts the Matrix calls made so far.
func (f *Fake) Matrices() int { f.mu.Lock(); defer f.mu.Unlock(); return f.matrices }

// Nearbys counts the Nearby calls made so far.
func (f *Fake) Nearbys() int { f.mu.Lock(); defer f.mu.Unlock(); return f.nearbys }

// Searches counts the Search calls made so far.
func (f *Fake) Searches() int { f.mu.Lock(); defer f.mu.Unlock(); return f.searches }

// DetailsCalls counts the Details calls made so far.
func (f *Fake) DetailsCalls() int { f.mu.Lock(); defer f.mu.Unlock(); return f.details }

// HoursCalls counts the Hours calls made so far.
func (f *Fake) HoursCalls() int { f.mu.Lock(); defer f.mu.Unlock(); return f.hours }

// NearbyRequests returns the Nearby requests made so far, in order.
func (f *Fake) NearbyRequests() []maps.NearbyRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.nearbyReqs)
}

// FindQueries returns the Find queries made so far, in order.
func (f *Fake) FindQueries() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.findQueries)
}

// RouteCalls returns the WalkingRoute (from, to) pairs asked for so far.
func (f *Fake) RouteCalls() [][2]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.routeCalls)
}

// WalkCalls returns the places of each WalkThrough call made so far.
func (f *Fake) WalkCalls() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.walkCalls)
}

// ResetWalkCalls forgets the WalkThrough calls recorded so far.
func (f *Fake) ResetWalkCalls() { f.mu.Lock(); f.walkCalls = nil; f.mu.Unlock() }

func (f *Fake) Matrix(_ context.Context, r maps.MatrixRequest) ([]maps.MatrixElement, error) {
	f.count(&f.matrices)
	if f.MatrixErr != nil {
		return nil, f.MatrixErr
	}
	if f.MatrixFunc != nil {
		return f.MatrixFunc(r), nil
	}
	d := f.TravelTime
	if d == 0 {
		d = 10 * time.Minute
	}
	var out []maps.MatrixElement
	for i := range r.Origins {
		for j := range r.Destinations {
			out = append(out, maps.MatrixElement{Origin: i, Destination: j, Duration: d, OK: true})
		}
	}
	return out, nil
}

func (f *Fake) Nearby(_ context.Context, r maps.NearbyRequest) ([]maps.Place, error) {
	f.mu.Lock()
	f.nearbys++
	f.nearbyReqs = append(f.nearbyReqs, r)
	f.mu.Unlock()
	if f.NearbyErr != nil {
		return nil, f.NearbyErr
	}
	if f.NearbyFunc == nil {
		return nil, nil
	}
	return f.NearbyFunc(r), nil
}

func (f *Fake) Details(_ context.Context, id, lang string) (maps.PlaceDetails, error) {
	f.count(&f.details)
	if f.DetailsErr != nil {
		return maps.PlaceDetails{}, f.DetailsErr
	}
	if d, ok := f.Localized[id][lang]; ok {
		return d, nil
	}
	p, ok := f.Places[id]
	if !ok {
		return maps.PlaceDetails{}, errors.New("mapstest: no such place")
	}
	return p.Details, nil
}

func (f *Fake) Search(_ context.Context, req maps.SearchRequest) ([]maps.SearchResult, error) {
	f.count(&f.searches)
	if f.SearchErr != nil {
		return nil, f.SearchErr
	}
	if f.SearchFunc != nil {
		return f.SearchFunc(req), nil
	}
	var out []maps.SearchResult
	for _, id := range f.Found[req.Near] {
		out = append(out, f.Places[id])
	}
	return out, nil
}

func (f *Fake) Locate(_ context.Context, id string) (maps.SearchResult, error) {
	p, ok := f.Places[id]
	if !ok {
		return p, errors.New("mapstest: no such place")
	}
	return p, nil
}

func (f *Fake) Find(_ context.Context, req maps.FindRequest) (maps.Found, bool, error) {
	f.mu.Lock()
	f.findQueries = append(f.findQueries, req.Query)
	f.mu.Unlock()
	if f.FindErr != nil {
		return maps.Found{}, false, f.FindErr
	}
	if f.FindFunc == nil {
		return maps.Found{}, false, nil
	}
	p, ok := f.FindFunc(req)
	return p, ok, nil
}

func (f *Fake) WalkingRoute(_ context.Context, from, to string) (maps.Route, error) {
	f.mu.Lock()
	f.routeCalls = append(f.routeCalls, [2]string{from, to})
	f.mu.Unlock()
	if f.RouteErr != nil {
		return maps.Route{}, f.RouteErr
	}
	if f.RouteFunc != nil {
		return f.RouteFunc(from, to), nil
	}
	return maps.Route{Polyline: "route:" + from + ">" + to, Duration: 10 * time.Minute, Meters: 800}, nil
}

func (f *Fake) WalkThrough(_ context.Context, places []string, optimize bool) (maps.WalkRoute, error) {
	f.mu.Lock()
	f.walkCalls = append(f.walkCalls, slices.Clone(places))
	f.mu.Unlock()
	if f.WalkErr != nil {
		return maps.WalkRoute{}, f.WalkErr
	}
	if f.WalkFunc != nil {
		return f.WalkFunc(places, optimize)
	}
	order := []int{0}
	for i := 1; i < len(places)-1; i++ {
		order = append(order, i)
	}
	if f.ReverseOptimized && optimize {
		slices.Reverse(order[1:])
	}
	if len(places) > 1 {
		order = append(order, len(places)-1)
	}
	w := maps.WalkRoute{Order: order, Route: maps.Route{Polyline: "walk-line"}}
	for _, i := range order[1:] {
		d, ok := f.Legs[places[i]]
		if !ok {
			d = f.LegDuration
			if d == 0 {
				d = 10 * time.Minute
			}
		}
		w.Legs = append(w.Legs, maps.Route{Duration: d, Meters: int(d.Minutes() * 80)})
		w.Duration += d
		w.Meters += int(d.Minutes() * 80)
	}
	return w, nil
}

func (f *Fake) Hours(_ context.Context, id string) (maps.Hours, error) {
	f.count(&f.hours)
	if f.HoursErr != nil {
		return maps.Hours{}, f.HoursErr
	}
	if f.HoursFunc == nil {
		return maps.Hours{}, nil
	}
	return f.HoursFunc(id), nil
}
