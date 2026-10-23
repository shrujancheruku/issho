package walks

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
)

var center = maps.LatLng{Lat: 35.7, Lng: 139.65}

// near is a point about dm metres north of the centre.
func near(dm float64) maps.LatLng { return maps.LatLng{Lat: center.Lat + dm/111000, Lng: center.Lng} }

// fakePlaces answers Find from a table keyed by query (without the city).
type fakePlaces struct {
	mu      sync.Mutex
	places  map[string]maps.Found
	queries []string
}

func (f *fakePlaces) Find(_ context.Context, req maps.FindRequest) (maps.Found, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.queries = append(f.queries, req.Query)
	name := strings.TrimSuffix(req.Query, ", Tokyo")
	p, ok := f.places[name]
	if p.Name == "" {
		p.Name = name
	}
	return p, ok, nil
}

// fakeRoutes walks the places in order (or reversed in between, to stand
// for Google reordering), each leg taking legs[to place] (default 8 min).
type fakeRoutes struct {
	legs    map[string]time.Duration
	reverse bool
	calls   [][]string
}

func (f *fakeRoutes) WalkThrough(_ context.Context, places []string, optimize bool) (maps.WalkRoute, error) {
	f.calls = append(f.calls, places)
	order := []int{0}
	for i := 1; i < len(places)-1; i++ {
		order = append(order, i)
	}
	if f.reverse && optimize {
		slices.Reverse(order[1:])
	}
	order = append(order, len(places)-1)
	w := maps.WalkRoute{Order: order}
	for _, i := range order[1:] {
		d, ok := f.legs[places[i]]
		if !ok {
			d = 8 * time.Minute
		}
		w.Legs = append(w.Legs, maps.Route{Duration: d, Meters: int(d.Minutes() * 80)})
		w.Duration += d
		w.Meters += int(d.Minutes() * 80)
	}
	w.Polyline = "line"
	return w, nil
}

// fakeProposer returns its proposals in turn and records the feedback.
type fakeProposer struct {
	proposals []Proposal
	feedback  [][]string
}

func (f *fakeProposer) Propose(_ context.Context, _ Request, feedback []string) (Proposal, error) {
	f.feedback = append(f.feedback, feedback)
	if len(f.proposals) == 0 {
		return Proposal{}, errors.New("no more proposals")
	}
	p := f.proposals[0]
	f.proposals = f.proposals[1:]
	return p, nil
}

func place(name, kind string, minutes int) Place {
	return Place{Name: name, Search: name, Kind: kind, Minutes: minutes, Note: "Look at " + name + "."}
}

// temples is a good proposal: station, four temples and shrines, a café.
func temples() Proposal {
	return Proposal{
		Title: "Koenji's temples, ending with coffee", Summary: "A quiet loop of temples.",
		Meet:   place("Koenji Station", "station", 10),
		Stops:  []Place{place("Temple A", "temple", 25), place("Shrine B", "shrine", 25), place("Temple C", "temple", 25), place("Temple D", "temple", 20)},
		Finish: place("Cafe E", "cafe", 45),
	}
}

func templePlaces() map[string]maps.Found {
	found := func(id, typ string, d float64) maps.Found {
		return maps.Found{Place: maps.Place{ID: id, PrimaryType: typ, Location: near(d)}}
	}
	return map[string]maps.Found{
		"Koenji Station": found("p-station", "train_station", 0),
		"Temple A":       found("p-a", "buddhist_temple", 400),
		"Shrine B":       found("p-b", "shinto_shrine", 600),
		"Temple C":       found("p-c", "buddhist_temple", 800),
		"Temple D":       found("p-d", "buddhist_temple", 1000),
		"Cafe E":         found("p-e", "cafe", 1100),
	}
}

func composer(p *fakeProposer, places *fakePlaces, routes *fakeRoutes) *Composer {
	return &Composer{Proposer: p, Places: places, Routes: routes, Rules: DefaultRules()}
}

func req() Request {
	return Request{City: "Tokyo", Area: "Koenji", Center: center, Activity: "Temples and shrines", Theme: "temples", Month: time.November}
}

func roles(w Walk) string {
	var out []string
	for _, s := range w.Stops {
		out = append(out, string(s.Role)+":"+s.PlaceID)
	}
	return strings.Join(out, " ")
}

func TestCompose(t *testing.T) {
	places := &fakePlaces{places: templePlaces()}
	routes := &fakeRoutes{reverse: true}
	w, err := composer(&fakeProposer{proposals: []Proposal{temples()}}, places, routes).Compose(context.Background(), req())
	if err != nil {
		t.Fatal(err)
	}
	// Google's order for the stops in between; meet first, finish last.
	if got := roles(w); got != "meet:p-station stop:p-d stop:p-c stop:p-b stop:p-a finish:p-e" {
		t.Errorf("stops = %s", got)
	}
	// 10 + 95 at the stops + 45, and 5 legs of 8 minutes.
	if w.Minutes() != 190 || w.Stops[0].Walk != 0 || w.Stops[1].Walk != 8*time.Minute || w.Route.Polyline != "line" {
		t.Errorf("walk = %+v (%d min)", w, w.Minutes())
	}
	if w.Title != "Koenji's temples, ending with coffee" || w.Stops[1].Note != "Look at Temple D." {
		t.Errorf("text = %q / %q", w.Title, w.Stops[1].Note)
	}
	// The city is added to every search, so a common name finds the right place.
	if !slices.Contains(places.queries, "Temple A, Tokyo") {
		t.Errorf("queries = %q", places.queries)
	}
}

func TestComposeDropsBadStops(t *testing.T) {
	p := temples()
	p.Stops = append(p.Stops,
		place("Made-up Temple", "temple", 20), // not found
		place("Old Shrine", "shrine", 20),     // closed
		place("Far Temple", "temple", 20),     // 5 km away
		place("A Bank", "temple", 20),         // not a temple
		place("Temple A again", "temple", 20), // the same place as Temple A
		place("Hase-ji", "temple", 20),        // made up: Google returns Temple C
	)
	places := templePlaces()
	places["Old Shrine"] = maps.Found{Place: maps.Place{ID: "p-old", PrimaryType: "shinto_shrine", Location: near(300)}, Closed: true}
	places["Far Temple"] = maps.Found{Place: maps.Place{ID: "p-far", PrimaryType: "buddhist_temple", Location: near(5000)}}
	places["A Bank"] = maps.Found{Place: maps.Place{ID: "p-bank", PrimaryType: "bank", Location: near(200)}}
	places["Temple A again"] = places["Temple A"]
	places["Hase-ji"] = maps.Found{Place: places["Temple C"].Place, Name: "Temple C"}
	c := composer(&fakeProposer{proposals: []Proposal{p}}, &fakePlaces{places: places}, &fakeRoutes{})
	var attempts []Attempt
	c.OnAttempt = func(a Attempt) { attempts = append(attempts, a) }
	w, err := c.Compose(context.Background(), req())
	if err != nil {
		t.Fatal(err)
	}
	if got := roles(w); got != "meet:p-station stop:p-a stop:p-b stop:p-c stop:p-d finish:p-e" {
		t.Errorf("stops = %s", got)
	}
	// Each dropped place is reported with why, for benchmarks.
	reasons := map[string]string{}
	for _, d := range attempts[0].Drops {
		reasons[d.Name] = d.Reason
	}
	want := map[string]string{"Made-up Temple": DropNotFound, "Hase-ji": DropNotFound, "Old Shrine": DropClosed, "Far Temple": DropFar, "A Bank": DropWrongKind, "Temple A again": DropDuplicate}
	if len(attempts) != 1 || attempts[0].Walk == nil || len(reasons) != len(want) {
		t.Fatalf("attempts = %+v", attempts)
	}
	for name, r := range want {
		if reasons[name] != r {
			t.Errorf("%s dropped as %q, want %q", name, reasons[name], r)
		}
	}
}

func TestComposeDropsOutlierStops(t *testing.T) {
	// Temple C is 30 minutes from Shrine B: left out, and the walk routed
	// again without it.
	routes := &fakeRoutes{legs: map[string]time.Duration{"p-c": 30 * time.Minute}}
	w, err := composer(&fakeProposer{proposals: []Proposal{temples()}}, &fakePlaces{places: templePlaces()}, routes).Compose(context.Background(), req())
	if err != nil {
		t.Fatal(err)
	}
	if got := roles(w); got != "meet:p-station stop:p-a stop:p-b stop:p-d finish:p-e" || len(routes.calls) != 2 {
		t.Errorf("stops = %s after %d routes", got, len(routes.calls))
	}
}

func TestComposeRetriesWithFeedback(t *testing.T) {
	bad := temples()
	bad.Meet = place("Somewhere Nice", "park", 10)
	bad.Stops = []Place{place("Temple A", "temple", 25), place("Made-up Temple", "temple", 20), place("Shrine B", "shrine", 20)}
	prop := &fakeProposer{proposals: []Proposal{bad, temples()}}
	w, err := composer(prop, &fakePlaces{places: templePlaces()}, &fakeRoutes{}).Compose(context.Background(), req())
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Stops) != 6 || len(prop.feedback) != 2 || prop.feedback[0] != nil {
		t.Fatalf("walk %s, feedback %q", roles(w), prop.feedback)
	}
	fb := strings.Join(prop.feedback[1], "\n")
	for _, want := range []string{"Somewhere Nice: the meeting place must be a station or a café", "Made-up Temple: not found on Google Maps", "too few places"} {
		if !strings.Contains(fb, want) {
			t.Errorf("feedback %q lacks %q", fb, want)
		}
	}
}

func TestComposeRejects(t *testing.T) {
	long := temples()
	for i := range long.Stops {
		long.Stops[i].Minutes = 60
	}
	long.Finish.Minutes = 90
	short := temples()
	for i := range short.Stops {
		short.Stops[i].Minutes = 5
	}
	short.Finish.Minutes = 30
	wrongFinish := temples()
	wrongFinish.Finish = place("Temple D", "temple", 30)
	missingCafe := templePlaces()
	delete(missingCafe, "Cafe E")
	farApart := map[string]time.Duration{"p-a": 25 * time.Minute, "p-b": 25 * time.Minute, "p-c": 25 * time.Minute, "p-d": 25 * time.Minute}

	for name, tc := range map[string]struct {
		p      Proposal
		places map[string]maps.Found
		legs   map[string]time.Duration
		want   string
	}{
		"too long":            {long, templePlaces(), nil, "too long: aim for about 3 hours"},
		"too short":           {short, templePlaces(), nil, "too short"},
		"finish not to sit":   {wrongFinish, templePlaces(), nil, "must end at a café"},
		"finish not found":    {temples(), missingCafe, nil, "Cafe E: not found"},
		"stops too far apart": {temples(), templePlaces(), farApart, "too long: keep every walk"},
	} {
		t.Run(name, func(t *testing.T) {
			prop := &fakeProposer{proposals: []Proposal{tc.p, tc.p}}
			_, err := composer(prop, &fakePlaces{places: tc.places}, &fakeRoutes{legs: tc.legs}).Compose(context.Background(), req())
			var rej *Rejected
			if !errors.As(err, &rej) || !strings.Contains(rej.Error(), tc.want) {
				t.Fatalf("err = %v, want a rejection with %q", err, tc.want)
			}
			if len(prop.feedback) != 2 {
				t.Errorf("%d proposals, want 2", len(prop.feedback))
			}
		})
	}
}

func TestComposeErrors(t *testing.T) {
	// A model or API failure isn't a rejection: the worker tries again later.
	_, err := composer(&fakeProposer{}, &fakePlaces{places: templePlaces()}, &fakeRoutes{}).Compose(context.Background(), req())
	var rej *Rejected
	if err == nil || errors.As(err, &rej) {
		t.Errorf("err = %v", err)
	}
}

func TestIs(t *testing.T) {
	ramen := maps.Found{Place: maps.Place{PrimaryType: "ramen_restaurant"}}
	izakaya := maps.Found{Place: maps.Place{PrimaryType: "japanese_izakaya_restaurant"}}
	bank := maps.Found{Place: maps.Place{PrimaryType: "bank", Types: []string{"bank", "finance"}}}
	shrine := maps.Found{Place: maps.Place{PrimaryType: "tourist_attraction", Types: []string{"shinto_shrine", "tourist_attraction"}}}
	for _, tc := range []struct {
		f    maps.Found
		kind string
		want bool
	}{
		{ramen, "restaurant", true}, {izakaya, "bar", true}, {izakaya, "restaurant", true},
		{bank, "temple", false}, {bank, "street", true}, {shrine, "shrine", true}, {ramen, "cafe", false},
	} {
		if got := is(tc.f, tc.kind); got != tc.want {
			t.Errorf("is(%s, %s) = %v", tc.f.PrimaryType, tc.kind, got)
		}
	}
}
