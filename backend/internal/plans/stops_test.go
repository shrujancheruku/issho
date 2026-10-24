package plans

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// fakeMap knows places around (0, 0), the fixture's area centre, and walks
// 10 minutes between any two unless told otherwise.
type fakeMap struct {
	places map[string]maps.SearchResult
	legs   map[[2]string]time.Duration
	walks  [][]string
}

func (f *fakeMap) Details(_ context.Context, id, _ string) (maps.PlaceDetails, error) {
	p, ok := f.places[id]
	if !ok {
		return maps.PlaceDetails{}, errors.New("not found")
	}
	return p.Details, nil
}

func (f *fakeMap) Search(_ context.Context, req maps.SearchRequest) ([]maps.SearchResult, error) {
	var out []maps.SearchResult
	for _, id := range []string{"meet", "shrine", "far"} {
		out = append(out, f.places[id])
	}
	return out, nil
}

func (f *fakeMap) Locate(_ context.Context, id string) (maps.SearchResult, error) {
	p, ok := f.places[id]
	if !ok {
		return p, errors.New("not found")
	}
	return p, nil
}

func (f *fakeMap) WalkThrough(_ context.Context, places []string, optimize bool) (maps.WalkRoute, error) {
	if optimize {
		return maps.WalkRoute{}, errors.New("an admin's order must be kept")
	}
	f.walks = append(f.walks, places)
	r := maps.WalkRoute{Route: maps.Route{Polyline: "poly"}}
	for i := 1; i < len(places); i++ {
		d, ok := f.legs[[2]string{places[i-1], places[i]}]
		if !ok {
			d = 10 * time.Minute
		}
		r.Legs = append(r.Legs, maps.Route{Duration: d})
		r.Duration += d
	}
	return r, nil
}

// TestIntegrationEditStops: an admin edits a walk's stops (reordered, one
// added, times and notes changed) and the route is worked out again in
// their order, with warnings for rules it breaks; bad stops are refused;
// another plan's venue is swapped for one found there.
func TestIntegrationEditStops(t *testing.T) {
	f := newFixture(t, "zz-plans-stops", "Asia/Tokyo", 0)
	ctx := context.Background()
	const cat = "zz-plans-stops-cafe"
	cleanup := func() {
		if _, err := f.pool.Exec(ctx, "delete from app.venue_categories where id = $1", cat); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	at := func(id, name string, lng float64, kind string) maps.SearchResult {
		loc := maps.LatLng{Lat: 0, Lng: lng}
		return maps.SearchResult{
			Place:   maps.Place{ID: id, PrimaryType: kind, Types: []string{kind}, Location: loc},
			Details: maps.PlaceDetails{Name: name, Address: name + " street", MapsURL: "https://maps.example/" + id, Location: loc},
		}
	}
	// 0.01° of longitude at the equator is about 1.1 km.
	fm := &fakeMap{places: map[string]maps.SearchResult{
		"meet":   at("meet", "Station", 0, "train_station"),
		"temple": at("temple", "Old Temple", 0.005, "buddhist_temple"),
		"shrine": at("shrine", "Shrine", 0.01, "shinto_shrine"),
		"street": at("street", "Old Street", 0.012, "route"),
		"cafe":   at("cafe", "Tea House", 0.015, "cafe"),
		"far":    at("far", "Far Away", 0.05, "park"),
		"gone":   at("gone", "Gone Bar", 0.002, "bar"),
		"cafe2":  at("cafe2", "Second Cafe", 0.003, "cafe"),
	}, legs: map[[2]string]time.Duration{{"shrine", "street"}: 25 * time.Minute}}
	gone := fm.places["gone"]
	gone.Closed = true
	fm.places["gone"] = gone

	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	svc := &Service{DB: f.pool, Places: fm}
	h := &ReviewHandler{Svc: svc, Timing: DefaultTiming(), Now: func() time.Time { return now }, Places: fm, Routes: fm}
	adminCtx := auth.WithCaller(ctx, auth.Caller{UserID: uuid.New(), Admin: true})

	walk := walks.Walk{Title: "Temples", Summary: "A walk.", Route: maps.Route{Polyline: "old", Duration: 30 * time.Minute}, Stops: []walks.WalkStop{
		{Role: walks.Meet, PlaceID: "meet", Minutes: 10, Note: "By the gate"},
		{Role: walks.Stop, PlaceID: "temple", Minutes: 30, Note: "Old wood", Walk: 10 * time.Minute},
		{Role: walks.Stop, PlaceID: "shrine", Minutes: 20, Note: "Foxes", Walk: 10 * time.Minute},
		{Role: walks.Finish, PlaceID: "cafe", Minutes: 60, Note: "Matcha", Walk: 10 * time.Minute},
	}}
	sugg := matching.Suggestion{Plan: matching.Plan{Area: matching.AreaID(f.area), Activity: matching.ActivityID(f.act), Primary: "ja"}, Audience: 6, Score: 1}
	planID, err := CreateWalk(ctx, f.pool, f.city, sugg, Window{Start: now, Days: 14, Deadline: now.Add(7 * 24 * time.Hour)}, walk, PlanDraft, now)
	if err != nil {
		t.Fatal(err)
	}
	get := func(id uuid.UUID) *isshov1.GetReviewPlanResponse {
		t.Helper()
		res, err := h.GetReviewPlan(adminCtx, connect.NewRequest(&isshov1.GetReviewPlanRequest{PlanId: id.String()}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg
	}
	got := get(planID)
	if len(got.Stops) != 4 || got.Stops[1].PlaceId != "temple" || got.Stops[1].Note != "Old wood" || got.Stops[3].Role != "finish" {
		t.Fatalf("stops = %v", got.Stops)
	}

	// Found near the area, with the distance.
	found, err := h.FindPlanPlaces(adminCtx, connect.NewRequest(&isshov1.FindPlanPlacesRequest{PlanId: planID.String(), Query: "shrine"}))
	if err != nil {
		t.Fatal(err)
	}
	if p := found.Msg.Places[1]; p.Name != "Shrine" || p.Meters < 1000 || p.Meters > 1200 || p.PrimaryType != "shinto_shrine" {
		t.Errorf("found = %v", p)
	}

	set := func(stops ...*isshov1.ReviewStop) (*isshov1.SetPlanStopsResponse, error) {
		res, err := h.SetPlanStops(adminCtx, connect.NewRequest(&isshov1.SetPlanStopsRequest{PlanId: planID.String(), Stops: stops}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}
	stop := func(id string, minutes int32, note string) *isshov1.ReviewStop {
		return &isshov1.ReviewStop{PlaceId: id, Minutes: minutes, Note: note}
	}
	for name, stops := range map[string][]*isshov1.ReviewStop{
		"two places":     {stop("meet", 10, ""), stop("cafe", 60, "")},
		"a place twice":  {stop("meet", 10, ""), stop("shrine", 20, ""), stop("shrine", 20, ""), stop("cafe", 60, "")},
		"too far":        {stop("meet", 10, ""), stop("far", 20, ""), stop("cafe", 60, "")},
		"closed":         {stop("meet", 10, ""), stop("gone", 20, ""), stop("cafe", 60, "")},
		"unknown":        {stop("meet", 10, ""), stop("nowhere", 20, ""), stop("cafe", 60, "")},
		"too long there": {stop("meet", 10, ""), stop("shrine", 121, ""), stop("cafe", 60, "")},
	} {
		if _, err := set(stops...); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}

	// Shrine before temple, a street added, the temple's time and note changed.
	res, err := set(stop("meet", 10, "By the north gate"), stop("shrine", 20, "Foxes"), stop("street", 15, " Old shops "),
		stop("temple", 45, "Look up at the roof"), stop("cafe", 60, "Matcha"))
	if err != nil {
		t.Fatal(err)
	}
	if last := fm.walks[len(fm.walks)-1]; !slices.Equal(last, []string{"meet", "shrine", "street", "temple", "cafe"}) {
		t.Errorf("routed through %v", last)
	}
	// 150 minutes at the stops + 55 walking; the shrine to the street is 25.
	if res.Minutes != 205 || len(res.Warnings) != 1 || res.Warnings[0] != "The walk from stop 2 to stop 3 takes 25 minutes (the composer allows 20)." {
		t.Errorf("result = %v", res)
	}
	got = get(planID)
	if ids := []string{got.Stops[0].PlaceId, got.Stops[2].PlaceId, got.Stops[3].PlaceId}; !slices.Equal(ids, []string{"meet", "street", "temple"}) ||
		got.Stops[2].Role != "stop" || got.Stops[2].Note != "Old shops" || got.Stops[3].Minutes != 45 || got.Stops[4].Role != "finish" {
		t.Fatalf("after editing = %v", got.Stops)
	}
	if s := got.Steps[3]; s.WalkMinutes != 10 || s.VenueName != "Temple" && s.VenueName != "Old Temple" {
		t.Errorf("third step = %v", s)
	}
	var secs int32
	var poly string
	if err := f.pool.QueryRow(ctx, "select route_seconds, route_polyline from app.plans where id = $1", planID).Scan(&secs, &poly); err != nil || secs != 55*60 || poly != "poly" {
		t.Errorf("route = %d %q, %v", secs, poly, err)
	}

	// Not on a closed plan.
	if _, err := f.pool.Exec(ctx, "update app.plans set status = 'closed', closed_reason = 'withdrawn', closed_at = now() where id = $1", planID); err != nil {
		t.Fatal(err)
	}
	if _, err := set(stop("meet", 10, ""), stop("shrine", 20, ""), stop("cafe", 60, "")); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("a closed plan: %v", err)
	}

	// Another plan: its venue swapped for one found for the activity.
	for _, s := range []string{
		`insert into app.venue_categories (id, name, google_types) values ('` + cat + `', '{"en":"Cafe"}', '{cafe}')`,
		`update app.activities set venue_categories = '{` + cat + `}' where id = '` + f.act + `'`,
		`insert into app.venues (google_place_id, city_id, area_id, category_id) values ('cafe', '` + f.city + `', '` + f.area + `', '` + cat + `'), ('cafe2', '` + f.city + `', '` + f.area + `', '` + cat + `')`,
	} {
		if _, err := f.pool.Exec(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	city, err := loadPlaces(ctx, f.q, f.city, 0)
	if err != nil {
		t.Fatal(err)
	}
	acts := map[matching.ActivityID]*matching.Activity{}
	for i := range city.Activities {
		acts[city.Activities[i].ID] = &city.Activities[i]
	}
	other, err := create(ctx, f.q, f.city, sugg, Window{Start: now, Days: 14, Deadline: now.Add(7 * 24 * time.Hour)}, acts, PlanOpen)
	if err != nil {
		t.Fatal(err)
	}
	venue := func(id string, followUp bool) error {
		_, err := h.SetPlanVenue(adminCtx, connect.NewRequest(&isshov1.SetPlanVenueRequest{PlanId: other.String(), PlaceId: id, FollowUp: followUp}))
		return err
	}
	if err := venue("cafe2", false); err != nil {
		t.Fatal(err)
	}
	if v := get(other).VenuePlaceId; v != "cafe2" {
		t.Errorf("venue = %q", v)
	}
	if err := venue("shrine", false); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a place not found for the activity: %v", err)
	}
	if err := venue("cafe", true); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a follow-up the plan doesn't have: %v", err)
	}
	if _, err := h.SetPlanVenue(adminCtx, connect.NewRequest(&isshov1.SetPlanVenueRequest{PlanId: planID.String(), PlaceId: "cafe"})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("a walk's venue: %v", err)
	}
	if _, err := set(); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("no stops: %v", err)
	}
}
