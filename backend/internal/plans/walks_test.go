package plans

import (
	"context"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// walkProposer proposes the same temple walk every time, or an unusable
// one when bad, and counts proposals.
type walkProposer struct {
	bad   bool
	calls int
	reqs  []walks.Request
}

func (p *walkProposer) Propose(_ context.Context, req walks.Request, _ []string) (walks.Proposal, error) {
	p.calls++
	p.reqs = append(p.reqs, req)
	stop := func(name, kind string, minutes int) walks.Place {
		return walks.Place{Name: name, Search: name, Kind: kind, Minutes: minutes, Note: "About " + name + "."}
	}
	w := walks.Proposal{
		Title: "Temple lanes, ending with tea", Summary: "A quiet loop of temples.",
		Meet:   stop("Station", "station", 10),
		Stops:  []walks.Place{stop("Temple A", "temple", 30), stop("Temple B", "temple", 30), stop("Temple C", "temple", 30)},
		Finish: stop("Tea House", "tea_house", 50),
	}
	if p.bad {
		w.Stops = w.Stops[:1] // too few stops
	}
	return w, nil
}

// findAll finds every place near (35.703, 139.58), each with a type that
// fits its name.
type findAll struct{}

func (findAll) Find(_ context.Context, r maps.FindRequest) (maps.Found, bool, error) {
	name, _, _ := strings.Cut(r.Query, ",")
	typ := map[string]string{"Station": "train_station", "Tea House": "tea_house"}[name]
	if typ == "" {
		typ = "buddhist_temple"
	}
	id := "zz-" + strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	return maps.Found{Place: maps.Place{ID: id, PrimaryType: typ, Location: maps.LatLng{Lat: 35.703, Lng: 139.58}}, Name: name}, true, nil
}

// walkRouter walks the places in the order given, 10 minutes a leg, and
// records each walk.
type walkRouter struct{ calls [][]string }

func (w *walkRouter) WalkThrough(_ context.Context, places []string, _ bool) (maps.WalkRoute, error) {
	w.calls = append(w.calls, places)
	r := maps.WalkRoute{Route: maps.Route{Polyline: "walk-line", Duration: time.Duration(len(places)-1) * 10 * time.Minute}}
	for i := range places {
		r.Order = append(r.Order, i)
		if i > 0 {
			r.Legs = append(r.Legs, maps.Route{Duration: 10 * time.Minute})
		}
	}
	return r, nil
}

// TestIntegrationWalks: the fixture's activity as a walk. Without a
// composer nothing opens; a walk that can't be composed isn't opened and
// rests; a good one opens with its stops, title, summary and route, and the
// plan screen shows them; its route is refreshed in the stored order and
// cleared with its legs; it never gets a meeting place of the old kind;
// and scheduling checks every stop is open while the group is there.
func TestIntegrationWalks(t *testing.T) {
	f := newFixture(t, "zz-plans-walks", "Asia/Tokyo", 8)
	ctx := context.Background()
	for _, id := range f.users {
		f.addProfile(t, id, "ja")
	}
	for _, s := range []string{
		`update app.activities set walk_theme = 'temples', name = '{"en":"Temples and shrines"}' where id = '` + f.act + `'`,
		`update app.areas set centroid = 'POINT(139.58 35.703)', name = '{"en":"Kichijoji"}' where id = '` + f.area + `'`,
	} {
		if _, err := f.pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)

	// No composer (no Gemini key): the walk isn't opened in another form.
	r := &Runner{DB: f.pool, Timing: DefaultTiming(), Log: log}
	if opened, err := r.Suggest(ctx, f.city, now); err != nil || opened != 0 {
		t.Fatalf("Suggest without a composer opened %d, %v", opened, err)
	}

	// A walk that can't be composed isn't opened, and the area rests.
	prop, router := &walkProposer{bad: true}, &walkRouter{}
	r.Walks = &walks.Composer{Proposer: prop, Places: findAll{}, Routes: router, Rules: walks.DefaultRules(), Log: log}
	r.WalkRoutes = router
	if opened, err := r.Suggest(ctx, f.city, now); err != nil || opened != 0 || prop.calls != 2 {
		t.Fatalf("Suggest with a bad walk opened %d after %d proposals, %v", opened, prop.calls, err)
	}
	if opened, _ := r.Suggest(ctx, f.city, now.Add(time.Hour)); opened != 0 || prop.calls != 2 {
		t.Fatalf("the area was tried again during its rest (%d proposals)", prop.calls)
	}
	req := prop.reqs[0]
	if req.City != "Test" || req.Area != "Kichijoji" || req.Activity != "Temples and shrines" || req.Theme != "temples" ||
		req.Month != time.January || req.Center.Lat != 35.703 {
		t.Errorf("request = %+v", req)
	}

	// After the rest, a good walk opens.
	prop.bad = false
	later := now.Add(WalkRetryAfter + time.Hour)
	if opened, err := r.Suggest(ctx, f.city, later); err != nil || opened != 1 {
		t.Fatalf("Suggest opened %d, %v", opened, err)
	}
	open, err := ListOpen(ctx, f.q, f.city)
	if err != nil || len(open) != 1 || !open[0].Walk || open[0].FollowUp != "" || open[0].Route == nil || open[0].Route.Polyline != "walk-line" {
		t.Fatalf("open = %+v, %v", open, err)
	}
	plan := open[0]
	stops, err := LoadWalk(ctx, f.q, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range stops {
		got = append(got, string(s.Role)+":"+s.PlaceID)
	}
	if want := "meet:zz-station stop:zz-temple-a stop:zz-temple-b stop:zz-temple-c finish:zz-tea-house"; strings.Join(got, " ") != want ||
		stops[0].Walk != 0 || stops[1].Walk != 10*time.Minute || stops[4].Minutes != 50 {
		t.Fatalf("stops = %s (%+v)", got, stops)
	}

	// The plan screen: the walk's own title and summary, and every stop
	// with its place, time, walk and note.
	svc := &Service{DB: f.pool, Places: fakePlaces{
		"zz-station":   {"ja": {Name: "吉祥寺駅", Location: maps.LatLng{Lat: 35.703, Lng: 139.58}}},
		"zz-tea-house": {"ja": {Name: "茶房", Location: maps.LatLng{Lat: 35.705, Lng: 139.582}}},
	}}
	item, steps, _, err := svc.Plan(ctx, f.users[0], plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Title != "Temple lanes, ending with tea" || item.Summary != "A quiet loop of temples." {
		t.Errorf("title %q, summary %q", item.Title, item.Summary)
	}
	if len(steps) != 5 || !steps[0].Meeting || steps[0].Venue.Name != "吉祥寺駅" || steps[0].Note != "About Station." ||
		steps[1].Activity != "" || steps[1].Minutes != 30 || steps[1].WalkMinutes != 10 || !steps[4].Finish || steps[4].Venue.Name != "茶房" {
		t.Errorf("steps = %+v", steps)
	}
	feed, err := svc.Feed(ctx, f.users[0])
	if err != nil || len(feed) != 1 || feed[0].Title != "Temple lanes, ending with tea" {
		t.Errorf("feed = %+v, %v", feed, err)
	}

	// A walk never gets the old kind of meeting place: it starts at its own.
	r.Places = &fakeSearch{}
	if found, err := r.RefreshMeetingPlaces(ctx, f.city); err != nil || found != 0 {
		t.Errorf("meeting places for a walk: %d, %v", found, err)
	}

	// At 25 days the route is recomputed through the stops in their order;
	// at 30 it's cleared with the legs (and, with a router, recomputed in
	// the same pass; without one it stays cleared).
	router.calls = nil
	if _, err := f.pool.Exec(ctx, "update app.plans set route_computed_at = $2 where id = $1", plan.ID, later.Add(-26*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if computed, _, err := r.RefreshRoutes(ctx, f.city, later); err != nil || computed != 1 || len(router.calls) != 1 ||
		!slices.Equal(router.calls[0], []string{"zz-station", "zz-temple-a", "zz-temple-b", "zz-temple-c", "zz-tea-house"}) {
		t.Fatalf("refresh computed %d with %q, %v", computed, router.calls, err)
	}
	r.WalkRoutes = nil
	if _, cleared, err := r.RefreshRoutes(ctx, f.city, later.Add(31*24*time.Hour)); err != nil || cleared != 1 {
		t.Fatalf("cleared %d, %v", cleared, err)
	}
	if stops, _ := LoadWalk(ctx, f.q, plan.ID); stops[1].Walk != 0 {
		t.Errorf("legs kept after the route was cleared: %+v", stops)
	}

	// Scheduling: the group arrives at the tea house about two and a half
	// hours after a 14:00 start (four 10-minute walks once the legs are
	// cleared, 120 minutes at the stops), so a tea house closing at 16:00
	// rules the time out, and one open until 18:00 doesn't.
	p := plan
	p.Walk = true
	start := time.Date(2030, 1, 12, 14, 0, 0, 0, time.FixedZone("JST", 9*3600))
	r.Hours = &fakeHours{h: everyDay(9, 16)}
	if v := r.checkVenues(ctx, p, start, map[string]*maps.Hours{}); v != venueClosedAt {
		t.Errorf("tea house closing at 16:00: %v", v)
	}
	r.Hours = &fakeHours{h: everyDay(9, 18)}
	if v := r.checkVenues(ctx, p, start, map[string]*maps.Hours{}); v != venuesOK {
		t.Errorf("open until 18:00: %v", v)
	}
	gone := &fakeHours{h: maps.Hours{Closed: true}}
	r.Hours = gone
	if v := r.checkVenues(ctx, p, start, map[string]*maps.Hours{}); v != venueGone || gone.calls != 1 {
		t.Errorf("closed stop: %v after %d lookups", v, gone.calls)
	}
}
