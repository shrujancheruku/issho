package geo

import (
	"context"
	"fmt"
	"math"
	"slices"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// Three areas: a and b are ~1.1 km apart, c is ~22 km away.
var areas = []Area{
	{ID: "x-a", LatLng: maps.LatLng{Lat: 35.700, Lng: 139.580}},
	{ID: "x-b", LatLng: maps.LatLng{Lat: 35.710, Lng: 139.580}},
	{ID: "x-c", LatLng: maps.LatLng{Lat: 35.900, Lng: 139.580}},
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
	got, n, err := DiscoverVenues(context.Background(), s, areas, cats, "ja", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.reqs) != 3+1 || n.Made != 4 { // cafe per area, beach once
		t.Errorf("got %d searches (%d counted), want 4", len(s.reqs), n.Made)
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

func TestDiscoverAreaVenues(t *testing.T) {
	cats := []VenueCategory{
		{ID: "cafe", GoogleTypes: []string{"cafe"}, RadiusM: 1500},
		{ID: "beach", GoogleTypes: []string{"beach"}, RadiusM: 50000},
	}
	// One area's own searches; the city-wide ones only for a new city.
	for wide, want := range map[bool]int{false: 1, true: 2} {
		s := &fakeSearcher{}
		got, n, err := DiscoverAreaVenues(context.Background(), s, areas, "x-a", cats, wide, "ja", 100)
		if err != nil || n.Made != want || len(s.reqs) != want {
			t.Fatalf("wide %v: %d searches (%d made), %v; want %d", wide, n.Made, len(s.reqs), err, want)
		}
		// Searched from x-a, but placed in the nearest area.
		if s.reqs[0].Center != areas[0].LatLng || got[0].AreaID != "x-b" {
			t.Errorf("wide %v: searched at %v, venue in %s", wide, s.reqs[0].Center, got[0].AreaID)
		}
	}
}

// square is a boundary about 2·half metres across, centred on c.
func square(c maps.LatLng, half float64) maps.Boundary {
	dLat, dLng := half/110574, half/(111320*math.Cos(c.Lat*math.Pi/180))
	return maps.Boundary{{
		{Lat: c.Lat - dLat, Lng: c.Lng - dLng}, {Lat: c.Lat - dLat, Lng: c.Lng + dLng},
		{Lat: c.Lat + dLat, Lng: c.Lng + dLng}, {Lat: c.Lat + dLat, Lng: c.Lng - dLng},
	}}
}

// denseSearcher returns a full page for any circle reaching the dense spot
// and two places otherwise, all at the circle's centre, plus one place far
// outside every area.
type denseSearcher struct {
	dense maps.LatLng
	reqs  []maps.NearbyRequest
}

func (f *denseSearcher) Nearby(_ context.Context, r maps.NearbyRequest) ([]maps.Place, error) {
	f.reqs = append(f.reqs, r)
	n := 2
	if maps.DistanceM(r.Center, f.dense) <= r.RadiusM {
		n = r.Max
	}
	out := []maps.Place{{ID: "far-away", PrimaryType: r.Types[0], Location: maps.LatLng{Lat: 36.5, Lng: 139.58}}}
	for i := range n - 1 {
		out = append(out, maps.Place{ID: fmt.Sprintf("%v-%d", r.Center, i), PrimaryType: r.Types[0], Location: r.Center})
	}
	return out, nil
}

func TestDiscoverVenuesWithinBoundaries(t *testing.T) {
	// Two 6 km square wards side by side; ward w's point is near its east
	// edge, so a place in w's west is nearer e's point than w's own.
	wc, ec := maps.LatLng{Lat: 16.0, Lng: 108.2}, maps.LatLng{Lat: 16.0, Lng: 108.2562}
	wards := []Area{
		{ID: "x-w", LatLng: maps.LatLng{Lat: 16.0, Lng: 108.22}, Boundary: square(wc, 3000)},
		{ID: "x-e", LatLng: ec, Boundary: square(ec, 3000)},
	}
	cats := []VenueCategory{{ID: "cafe", GoogleTypes: []string{"cafe"}, RadiusM: 800}}
	s := &denseSearcher{dense: maps.LatLng{Lat: 16.0, Lng: 108.19}} // in w's west
	got, n, err := DiscoverVenues(context.Background(), s, wards, cats, "vi", 1000)
	if err != nil {
		t.Fatal(err)
	}
	jobs := PlanVenues(wards, cats)
	if n.Made != len(s.reqs) || n.Made <= len(jobs) || n.Made >= MaxSearches(jobs) || n.Skipped != 0 {
		t.Errorf("%+v: want more than the %d first searches (the dense spot splits), fewer than the %d bound, none skipped", n, len(jobs), MaxSearches(jobs))
	}

	// A small budget: every area is still searched, never more than it allows.
	for _, budget := range []int{2, 9, 12} {
		s := &denseSearcher{dense: s.dense}
		_, n, err := DiscoverVenues(context.Background(), s, wards, cats, "vi", budget)
		if err != nil || n.Made > budget || n.Made != len(s.reqs) || n.Skipped == 0 {
			t.Errorf("budget %d: %+v, %d made, %v; want at most the budget and some skipped", budget, n, len(s.reqs), err)
		}
	}
	if _, _, err := DiscoverVenues(context.Background(), s, wards, cats, "vi", 1); err == nil {
		t.Error("a budget below the first searches ran")
	}
	smallest := math.Inf(1)
	for _, r := range s.reqs {
		smallest = min(smallest, r.RadiusM)
		if r.Max != pageSize {
			t.Errorf("search asked for %d places, want %d", r.Max, pageSize)
		}
	}
	// Splitting stops at the category's radius, not before.
	if smallest > 800 || smallest < 440 {
		t.Errorf("smallest circle %.0f m, want between 440 and 800", smallest)
	}
	// However the dense spot falls between circles, some smallest one
	// still reaches it.
	reached := false
	for _, r := range s.reqs {
		reached = reached || r.RadiusM == smallest && maps.DistanceM(r.Center, s.dense) <= r.RadiusM
	}
	if !reached {
		t.Error("no smallest circle reaches the dense spot")
	}
	for _, v := range got {
		if v.PlaceID == "far-away" {
			t.Errorf("a place outside every area was kept (in %s)", v.AreaID)
		}
	}
	// The dense spot's places are in w, whatever point is nearer.
	where, _ := Locate(wards, s.dense)
	if where.ID != "x-w" {
		t.Errorf("dense spot located in %s", where.ID)
	}
	west := maps.LatLng{Lat: 16.0, Lng: 108.175}
	if a, in := Locate(wards, west); a.ID != "x-w" || !in {
		t.Errorf("west of w located in %s (%v)", a.ID, in)
	}
	if a, in := Locate(wards, maps.LatLng{Lat: 17, Lng: 108.2}); in || a.ID != "x-w" {
		t.Errorf("outside: got %s, %v; want the nearest area and false", a.ID, in)
	}
}

func TestSplitKeepsCirclesReachingTheArea(t *testing.T) {
	c := maps.LatLng{Lat: 16.0, Lng: 108.2}
	j := VenueJob{Category: VenueCategory{RadiusM: 500}, Center: c, RadiusM: 4000, Within: square(c, 3000)}
	if got := len(split(j)); got != 7 {
		t.Errorf("a circle over the whole area split into %d, want 7", got)
	}
	// A circle mostly outside the area keeps only the children that reach it.
	j.Center = offset(c, 6000, math.Pi/2) // 6 km east: the area's edge is 3 km east of c
	kids := split(j)
	if len(kids) == 0 || len(kids) >= 7 {
		t.Errorf("an edge circle split into %d, want some but not all 7", len(kids))
	}
	for _, k := range kids {
		if j.Within.DistanceM(k.Center) > k.RadiusM {
			t.Errorf("kept a child %.0f m from the area with radius %.0f", j.Within.DistanceM(k.Center), k.RadiusM)
		}
	}
	j.RadiusM = 500
	if split(j) != nil {
		t.Error("split below the category radius")
	}
	j.Within, j.RadiusM = nil, 4000
	if split(j) != nil {
		t.Error("split a search without a boundary")
	}
}

// If every search came back full, the smallest circles must still cover
// every point of the area: nothing falls between them.
func TestSplitCoversTheArea(t *testing.T) {
	c := maps.LatLng{Lat: 35.7, Lng: 139.6}
	b := maps.Boundary{{ // an L shape, 8 km across
		{Lat: 35.664, Lng: 139.556}, {Lat: 35.664, Lng: 139.644}, {Lat: 35.682, Lng: 139.644},
		{Lat: 35.682, Lng: 139.578}, {Lat: 35.736, Lng: 139.578}, {Lat: 35.736, Lng: 139.556},
	}}
	jobs := PlanVenues([]Area{{ID: "x-l", LatLng: c, Boundary: b}}, []VenueCategory{{ID: "cafe", GoogleTypes: []string{"cafe"}, RadiusM: 800}})
	var leaves []VenueJob
	for len(jobs) > 0 {
		j := jobs[len(jobs)-1]
		jobs = jobs[:len(jobs)-1]
		if kids := split(j); kids != nil {
			jobs = append(jobs, kids...)
		} else {
			leaves = append(leaves, j)
		}
	}
	checked := 0
	for lat := 35.664; lat <= 35.736; lat += 0.0005 {
		for lng := 139.556; lng <= 139.644; lng += 0.0005 {
			p := maps.LatLng{Lat: lat, Lng: lng}
			if !b.Contains(p) {
				continue
			}
			checked++
			if !slices.ContainsFunc(leaves, func(l VenueJob) bool { return maps.DistanceM(l.Center, p) <= l.RadiusM }) {
				t.Fatalf("%v is in the area but in no circle", p)
			}
		}
	}
	if checked < 1000 {
		t.Fatalf("only %d points checked", checked)
	}
}
