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
	jobs := PlanMatrix(areas, []maps.Mode{maps.Transit, maps.Walk})
	if got := Elements(jobs); got != 6+2 { // transit: 3×2; walk: only a↔b
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

// fakeRouter: 90 s per km of latitude difference, no route to x-c by transit.
type fakeRouter struct{ calls int }

func (f *fakeRouter) Matrix(_ context.Context, r maps.MatrixRequest) ([]maps.MatrixElement, error) {
	f.calls++
	var out []maps.MatrixElement
	for oi, o := range r.Origins {
		for di, d := range r.Destinations {
			e := maps.MatrixElement{Origin: oi, Destination: di, OK: true,
				Duration: time.Duration(maps.DistanceM(o, d)*0.09) * time.Second}
			if r.Mode == maps.Transit && (d.Lat == 35.9 || o.Lat == 35.9) {
				e = maps.MatrixElement{Origin: oi, Destination: di}
			}
			out = append(out, e)
		}
	}
	return out, nil
}

func TestBuildMatrix(t *testing.T) {
	r := &fakeRouter{}
	got, err := BuildMatrix(context.Background(), r, areas, []maps.Mode{maps.Transit, maps.Walk}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]int32{}
	for _, tt := range got {
		byKey[tt.From+">"+tt.To+">"+string(tt.Mode)] = tt.Minutes
	}
	if m, ok := byKey["x-a>x-a>transit"]; !ok || m != 0 {
		t.Error("same-area rows must be stored as 0 minutes")
	}
	if _, ok := byKey["x-a>x-c>transit"]; ok {
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

func TestNextDeparture(t *testing.T) {
	tokyo := time.FixedZone("JST", 9*3600)
	cases := map[string]string{
		"2026-10-20T09:00:00+09:00": "2026-10-24T11:00:00+09:00", // Tuesday
		"2026-10-24T10:59:00+09:00": "2026-10-24T11:00:00+09:00", // Saturday morning
		"2026-10-24T11:00:00+09:00": "2026-10-31T11:00:00+09:00", // Saturday, too late
		"2026-10-23T20:00:00Z":      "2026-10-24T11:00:00+09:00", // Friday in UTC, Saturday 05:00 in Tokyo
	}
	for in, want := range cases {
		now, _ := time.Parse(time.RFC3339, in)
		if got := NextDeparture(now, tokyo).Format(time.RFC3339); got != want {
			t.Errorf("NextDeparture(%s) = %s, want %s", in, got, want)
		}
	}
}

type fakeSearcher struct{ reqs []maps.NearbyRequest }

func (f *fakeSearcher) Nearby(_ context.Context, r maps.NearbyRequest) ([]maps.Place, error) {
	f.reqs = append(f.reqs, r)
	// Every search finds the same café, located right next to x-b.
	return []maps.Place{{ID: "p1", Location: maps.LatLng{Lat: 35.7101, Lng: 139.58}}}, nil
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
		t.Fatalf("got %d venues, want one per category: %+v", len(got), got)
	}
	for _, v := range got {
		if v.AreaID != "x-b" {
			t.Errorf("venue %s/%s assigned to %s, want nearest area x-b", v.PlaceID, v.CategoryID, v.AreaID)
		}
	}
}
