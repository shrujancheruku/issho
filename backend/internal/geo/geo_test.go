package geo

import (
	"context"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// Three areas: a and b are ~1.1 km apart, c is ~22 km away.
var areas = []Area{
	{"x-a", maps.LatLng{Lat: 35.700, Lng: 139.580}},
	{"x-b", maps.LatLng{Lat: 35.710, Lng: 139.580}},
	{"x-c", maps.LatLng{Lat: 35.900, Lng: 139.580}},
}

func TestPlanMatrixSkipsSelfAndFarWalks(t *testing.T) {
	jobs := PlanMatrix(areas, []maps.Mode{maps.Drive, maps.Walk})
	if got := Elements(jobs); got != 6+2 { // drive: 3×2; walk: only a↔b
		t.Errorf("got %d elements, want 8", got)
	}
	for _, j := range jobs {
		for _, d := range j.Dests {
			if d.ID == j.Origin.ID {
				t.Errorf("%s job requests %s to itself", j.Mode, d.ID)
			}
			if j.Mode == maps.Walk && (d.ID == "x-c" || j.Origin.ID == "x-c") {
				t.Errorf("walk to/from far area requested: %s→%s", j.Origin.ID, d.ID)
			}
		}
	}
}

// fakeRouter: 90 s per km, and no driving route to or from x-c (e.g. an island).
type fakeRouter struct{ calls int }

func (f *fakeRouter) Matrix(_ context.Context, r maps.MatrixRequest) ([]maps.MatrixElement, error) {
	f.calls++
	var out []maps.MatrixElement
	for oi, o := range r.Origins {
		for di, d := range r.Destinations {
			e := maps.MatrixElement{Origin: oi, Destination: di, OK: true,
				Duration: time.Duration(maps.DistanceM(o, d)*0.09) * time.Second}
			if r.Mode == maps.Drive && (d.Lat == 35.9 || o.Lat == 35.9) {
				e = maps.MatrixElement{Origin: oi, Destination: di}
			}
			out = append(out, e)
		}
	}
	return out, nil
}

func TestBuildMatrix(t *testing.T) {
	r := &fakeRouter{}
	got, err := BuildMatrix(context.Background(), r, areas, []maps.Mode{maps.Drive, maps.Walk})
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]int32{}
	for _, tt := range got {
		byKey[tt.From+">"+tt.To+">"+string(tt.Mode)] = tt.Minutes
	}
	if m, ok := byKey["x-a>x-a>drive"]; !ok || m != 0 {
		t.Error("same-area rows must be stored as 0 minutes")
	}
	if _, ok := byKey["x-a>x-c>drive"]; ok {
		t.Error("pairs without a route must be left out")
	}
	// 1112 m × 0.09 s/m = 100 s → rounded up to 2 minutes.
	if m := byKey["x-a>x-b>walk"]; m != 2 {
		t.Errorf("x-a→x-b walk = %d min, want 2 (rounded up)", m)
	}
	if r.calls != 3+2 {
		t.Errorf("got %d provider calls, want 5", r.calls)
	}
}

type fakeSearcher struct{ reqs []maps.NearbyRequest }

func (f *fakeSearcher) Nearby(_ context.Context, r maps.NearbyRequest) ([]maps.Place, error) {
	f.reqs = append(f.reqs, r)
	// Every search finds the same place right next to x-b, plus one whose
	// primary type doesn't match the category (it must be dropped).
	return []maps.Place{
		{ID: "p1", PrimaryType: r.Types[0], Location: maps.LatLng{Lat: 35.7101, Lng: 139.58}},
		{ID: "bar-in-karaoke-results", PrimaryType: "bar", Location: maps.LatLng{Lat: 35.7101, Lng: 139.58}},
	}, nil
}

func TestDiscoverVenues(t *testing.T) {
	cats := []VenueCategory{
		{ID: "cafe", GoogleTypes: []string{"cafe"}, RadiusM: 1500},
		{ID: "beach", GoogleTypes: []string{"beach"}, RadiusM: 50000},
	}
	s := &fakeSearcher{}
	got, err := DiscoverVenues(context.Background(), s, areas, cats, "ja")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.reqs) != 3+1 { // cafe per area, beach once
		t.Errorf("got %d searches, want 4", len(s.reqs))
	}
	if len(got) != 2 {
		t.Fatalf("got %d venues, want one per category (wrong primary types dropped): %+v", len(got), got)
	}
	for _, v := range got {
		if v.AreaID != "x-b" {
			t.Errorf("venue %s/%s assigned to %s, want nearest area x-b", v.PlaceID, v.CategoryID, v.AreaID)
		}
	}
}
