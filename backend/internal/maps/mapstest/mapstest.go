// Package mapstest is an in-memory maps provider for tests: one Fake that
// implements the interfaces the admin modules (citypacks, curation) use, so
// they share a fake instead of each growing its own. Tests must never call
// the paid Google APIs.
package mapstest

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// Fake implements maps.Router, maps.PlaceSearcher and maps.PlaceSearch.
// Set its fields before use; the call counters are safe to read after the
// calls return. Errors are returned by every call of that kind.
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

	MatrixErr, NearbyErr, SearchErr error

	mu                          sync.Mutex
	matrices, nearbys, searches int
}

var (
	_ maps.Router        = (*Fake)(nil)
	_ maps.PlaceSearcher = (*Fake)(nil)
	_ maps.PlaceSearch   = (*Fake)(nil)
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

func (f *Fake) Matrix(_ context.Context, r maps.MatrixRequest) ([]maps.MatrixElement, error) {
	f.count(&f.matrices)
	if f.MatrixErr != nil {
		return nil, f.MatrixErr
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
	f.count(&f.nearbys)
	if f.NearbyErr != nil {
		return nil, f.NearbyErr
	}
	if f.NearbyFunc == nil {
		return nil, nil
	}
	return f.NearbyFunc(r), nil
}

func (f *Fake) Details(_ context.Context, id, _ string) (maps.PlaceDetails, error) {
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
