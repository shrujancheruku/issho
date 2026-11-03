package mapstest

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

func TestMatrixDefaultAndFunc(t *testing.T) {
	ctx := context.Background()
	req := maps.MatrixRequest{Origins: make([]maps.LatLng, 2), Destinations: make([]maps.LatLng, 3)}
	f := &Fake{}
	got, err := f.Matrix(ctx, req)
	if err != nil || len(got) != 6 || got[0].Duration != 10*time.Minute || !got[0].OK {
		t.Fatalf("default Matrix = %v, %v", got, err)
	}
	f.MatrixFunc = func(maps.MatrixRequest) []maps.MatrixElement { return []maps.MatrixElement{{Duration: time.Second}} }
	if got, _ := f.Matrix(ctx, req); len(got) != 1 || got[0].Duration != time.Second {
		t.Errorf("MatrixFunc ignored: %v", got)
	}
	if f.Matrices() != 2 {
		t.Errorf("Matrices = %d, want 2", f.Matrices())
	}
}

func TestNearbyRecordsRequests(t *testing.T) {
	f := &Fake{}
	for _, r := range []int{100, 200} {
		if _, err := f.Nearby(context.Background(), maps.NearbyRequest{RadiusM: float64(r)}); err != nil {
			t.Fatal(err)
		}
	}
	reqs := f.NearbyRequests()
	if len(reqs) != 2 || reqs[1].RadiusM != 200 || f.Nearbys() != 2 {
		t.Errorf("NearbyRequests = %v (%d calls)", reqs, f.Nearbys())
	}
}

func TestDetailsLocalizedThenPlaces(t *testing.T) {
	ctx := context.Background()
	f := &Fake{
		Localized: map[string]map[string]maps.PlaceDetails{"a": {"ja": {Name: "ja-a"}}},
		Places:    map[string]maps.SearchResult{"b": {Details: maps.PlaceDetails{Name: "b"}}},
	}
	if d, err := f.Details(ctx, "a", "ja"); err != nil || d.Name != "ja-a" {
		t.Errorf("localized = %v, %v", d, err)
	}
	if _, err := f.Details(ctx, "a", "en"); err == nil {
		t.Error("a language that isn't there: no error")
	}
	if d, err := f.Details(ctx, "b", "en"); err != nil || d.Name != "b" {
		t.Errorf("places fallback = %v, %v", d, err)
	}
	f.DetailsErr = errors.New("boom")
	if _, err := f.Details(ctx, "b", "en"); err == nil {
		t.Error("DetailsErr ignored")
	}
	if f.DetailsCalls() != 4 {
		t.Errorf("DetailsCalls = %d, want 4", f.DetailsCalls())
	}
}

func TestFindRecordsQueries(t *testing.T) {
	ctx := context.Background()
	f := &Fake{}
	if _, ok, err := f.Find(ctx, maps.FindRequest{Query: "x"}); ok || err != nil {
		t.Errorf("default Find = %v, %v; want nothing", ok, err)
	}
	f.FindFunc = func(r maps.FindRequest) (maps.Found, bool) { return maps.Found{Name: r.Query}, true }
	if p, ok, _ := f.Find(ctx, maps.FindRequest{Query: "y"}); !ok || p.Name != "y" {
		t.Errorf("FindFunc ignored: %v %v", p, ok)
	}
	if got := f.FindQueries(); !slices.Equal(got, []string{"x", "y"}) {
		t.Errorf("FindQueries = %v", got)
	}
}

func TestWalkingRouteRecordsAndOverrides(t *testing.T) {
	ctx := context.Background()
	f := &Fake{}
	r, err := f.WalkingRoute(ctx, "a", "b")
	if err != nil || r.Polyline != "route:a>b" || r.Duration != 10*time.Minute {
		t.Fatalf("default route = %v, %v", r, err)
	}
	f.RouteFunc = func(from, to string) maps.Route { return maps.Route{Polyline: to} }
	if r, _ := f.WalkingRoute(ctx, "a", "c"); r.Polyline != "c" {
		t.Errorf("RouteFunc ignored: %v", r)
	}
	if got := f.RouteCalls(); len(got) != 2 || got[1] != [2]string{"a", "c"} {
		t.Errorf("RouteCalls = %v", got)
	}
}

func TestWalkThrough(t *testing.T) {
	ctx := context.Background()
	places := []string{"s", "a", "b", "c", "e"}
	f := &Fake{Legs: map[string]time.Duration{"c": 30 * time.Minute}, LegDuration: 8 * time.Minute, ReverseOptimized: true}

	w, err := f.WalkThrough(ctx, places, false)
	if err != nil || !slices.Equal(w.Order, []int{0, 1, 2, 3, 4}) {
		t.Fatalf("in order: %v, %v", w.Order, err)
	}
	// Four legs: three of 8 minutes and one of 30 (into "c").
	if len(w.Legs) != 4 || w.Duration != 54*time.Minute || w.Legs[2].Duration != 30*time.Minute {
		t.Errorf("legs = %v, total %v", w.Legs, w.Duration)
	}
	// Optimizing reverses the places in between, never the first or last.
	if w, _ := f.WalkThrough(ctx, places, true); !slices.Equal(w.Order, []int{0, 3, 2, 1, 4}) {
		t.Errorf("optimized order = %v", w.Order)
	}
	if got := f.WalkCalls(); len(got) != 2 || !slices.Equal(got[0], places) {
		t.Errorf("WalkCalls = %v", got)
	}
	f.ResetWalkCalls()
	if len(f.WalkCalls()) != 0 {
		t.Error("ResetWalkCalls left calls")
	}
	f.WalkFunc = func([]string, bool) (maps.WalkRoute, error) { return maps.WalkRoute{}, errors.New("no") }
	if _, err := f.WalkThrough(ctx, places, false); err == nil {
		t.Error("WalkFunc ignored")
	}
}

func TestHours(t *testing.T) {
	ctx := context.Background()
	f := &Fake{}
	if h, err := f.Hours(ctx, "v"); err != nil || h.Known {
		t.Errorf("default hours = %v, %v; want unknown", h, err)
	}
	f.HoursFunc = func(id string) maps.Hours { return maps.Hours{Known: id == "v"} }
	if h, _ := f.Hours(ctx, "v"); !h.Known {
		t.Error("HoursFunc ignored")
	}
	if f.HoursCalls() != 2 {
		t.Errorf("HoursCalls = %d, want 2", f.HoursCalls())
	}
}

func TestSearchFunc(t *testing.T) {
	f := &Fake{SearchFunc: func(maps.SearchRequest) []maps.SearchResult { return []maps.SearchResult{{}, {}} }}
	if got, _ := f.Search(context.Background(), maps.SearchRequest{}); len(got) != 2 {
		t.Errorf("SearchFunc ignored: %v", got)
	}
}
