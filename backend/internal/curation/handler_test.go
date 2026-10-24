package curation

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	isshov1 "github.com/shrujancheruku/issho/backend/gen/issho/v1"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/platform/auth"
)

func TestNonAdminRefused(t *testing.T) {
	// admin() runs before any query, so no database is needed.
	h := &Handler{}
	ctx := auth.WithCaller(context.Background(), auth.Caller{UserID: uuid.New()})
	if _, err := h.ListVenues(ctx, connect.NewRequest(&isshov1.ListVenuesRequest{})); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("ListVenues as a user: %v", err)
	}
	if _, err := h.AddVenue(context.Background(), connect.NewRequest(&isshov1.AddVenueRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("AddVenue without a caller: %v", err)
	}
}

// fakePlaces knows a few places around two areas, by id.
type fakePlaces struct {
	places   map[string]maps.SearchResult
	byArea   map[maps.LatLng][]string // what a search near each area returns
	searches int
}

func (f *fakePlaces) Details(_ context.Context, id, _ string) (maps.PlaceDetails, error) {
	p, ok := f.places[id]
	if !ok {
		return maps.PlaceDetails{}, errors.New("not found")
	}
	return p.Details, nil
}

func (f *fakePlaces) Search(_ context.Context, req maps.SearchRequest) ([]maps.SearchResult, error) {
	f.searches++
	var out []maps.SearchResult
	for _, id := range f.byArea[req.Near] {
		out = append(out, f.places[id])
	}
	return out, nil
}

func (f *fakePlaces) Locate(_ context.Context, id string) (maps.SearchResult, error) {
	p, ok := f.places[id]
	if !ok {
		return p, errors.New("not found")
	}
	return p, nil
}

// TestIntegrationCuration: listing a city's places with live names and
// flags, filters and pages; approving and rejecting a place in all its
// categories; the city's switch as the matcher sees it; a new category
// inheriting the place's status; searching by name; adding a place.
// Skipped unless TEST_DATABASE_URL is set.
func TestIntegrationCuration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	const city, bar, cafe = "zz-curation", "zz-curation-bar", "zz-curation-cafe"
	areaA, areaB := city+"-a", city+"-b"
	// Areas about 4.5 km apart (0.05° of longitude at 35.7°N).
	locA, locB := maps.LatLng{Lat: 35.7, Lng: 139.65}, maps.LatLng{Lat: 35.7, Lng: 139.70}
	cleanup := func() {
		// The city cascades to its areas and venues; categories last.
		for _, s := range [][]any{
			{"delete from app.cities where id = $1", city},
			{"delete from app.venue_categories where id = any($1)", []string{bar, cafe}},
		} {
			if _, err := pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
				t.Errorf("cleanup: %v", err)
			}
		}
	}
	cleanup()
	t.Cleanup(cleanup)
	for _, s := range [][]any{
		{`insert into app.cities (id, name, timezone, currency, languages, travel_modes)
		  values ($1, '{"en":"Curation"}', 'Asia/Tokyo', 'JPY', '{ja}', '{walk}')`, city},
		{`insert into app.areas (id, city_id, name, centroid) values ($1, $2, '{"en":"Alpha"}', st_point($3, $4)), ($5, $2, '{"en":"Beta"}', st_point($6, $4))`,
			areaA, city, locA.Lng, locA.Lat, areaB, locB.Lng},
		{`insert into app.venue_categories (id, name, google_types, search_radius_m)
		  values ($1, '{"en":"Bar"}', '{bar}', 1500), ($2, '{"en":"Cafe"}', '{cafe,coffee_shop}', 1500)`, bar, cafe},
		// p1 is a snack bar that's also typed a café; p2 a bar in Beta; p3 a café.
		{`insert into app.venues (google_place_id, city_id, area_id, category_id) values
		  ('zz-p1', $1, $2, $4), ('zz-p1', $1, $2, $5), ('zz-p2', $1, $3, $4), ('zz-p3', $1, $2, $5)`, city, areaA, areaB, bar, cafe},
	} {
		if _, err := pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
			t.Fatal(err)
		}
	}
	near := func(p maps.LatLng, dLng float64) maps.LatLng { return maps.LatLng{Lat: p.Lat, Lng: p.Lng + dLng} }
	place := func(id, name string, loc maps.LatLng, types ...string) maps.SearchResult {
		return maps.SearchResult{
			Place:   maps.Place{ID: id, PrimaryType: types[0], Types: types, Location: loc},
			Details: maps.PlaceDetails{Name: name, Address: name + " street", MapsURL: "https://maps.example/" + id, Location: loc},
		}
	}
	fake := &fakePlaces{places: map[string]maps.SearchResult{
		"zz-p1": place("zz-p1", "スナック ゆき", locA, "bar", "cafe"),
		"zz-p2": place("zz-p2", "Bar Beta", locB, "bar"),
		"zz-p3": place("zz-p3", "Kissa Alpha", near(locA, 0.001), "cafe"),
		// New: a bar near Alpha, a closed one, and a café 3 km out of Beta.
		"zz-p4": place("zz-p4", "Snack Mimi", near(locA, 0.002), "bar", "establishment"),
		"zz-p5": place("zz-p5", "Gone Bar", near(locA, 0.003), "bar"),
		"zz-p6": place("zz-p6", "Far Cafe", near(locB, 0.033), "cafe"),
	}, byArea: map[maps.LatLng][]string{
		locA: {"zz-p1", "zz-p4", "zz-p5"},
		locB: {"zz-p4", "zz-p6"},
	}}
	gone := fake.places["zz-p5"]
	gone.Closed = true
	fake.places["zz-p5"] = gone
	now := time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)
	h := &Handler{DB: pool, Places: fake, Now: func() time.Time { return now }}
	adminID := uuid.New()
	adminCtx := auth.WithCaller(ctx, auth.Caller{UserID: adminID, Admin: true})
	q := dbgen.New(pool)

	cityRow := func() *isshov1.VenueCity {
		t.Helper()
		res, err := h.ListVenueCities(adminCtx, connect.NewRequest(&isshov1.ListVenueCitiesRequest{}))
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range res.Msg.Cities {
			if c.Id == city {
				return c
			}
		}
		t.Fatal("city missing")
		return nil
	}
	if c := cityRow(); c.Candidates != 3 || c.Approved != 0 || len(c.Areas) != 2 || c.Areas[0].Name != "Alpha" || c.ApprovedVenuesOnly {
		t.Fatalf("city = %v", c)
	}
	list := func(status isshov1.VenueStatus, area, category string, offset int32) *isshov1.ListVenuesResponse {
		t.Helper()
		res, err := h.ListVenues(adminCtx, connect.NewRequest(&isshov1.ListVenuesRequest{
			CityId: city, Status: status, AreaId: area, CategoryId: category, Offset: offset,
		}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg
	}
	ids := func(vs []*isshov1.Venue) []string {
		var out []string
		for _, v := range vs {
			out = append(out, v.PlaceId)
		}
		return out
	}
	candidates := isshov1.VenueStatus_VENUE_STATUS_CANDIDATE
	all := list(candidates, "", "", 0)
	if all.Total != 3 || !slices.Equal(ids(all.Venues), []string{"zz-p1", "zz-p3", "zz-p2"}) {
		t.Fatalf("candidates = %v (total %d)", ids(all.Venues), all.Total)
	}
	if v := all.Venues[0]; v.Name != "スナック ゆき" || v.Flag != "snack bar?" || v.AreaName != "Alpha" || len(v.CategoryIds) != 2 || v.MapsUrl == "" {
		t.Errorf("p1 = %v", v)
	}
	if got := ids(list(candidates, "", bar, 0).Venues); !slices.Equal(got, []string{"zz-p1", "zz-p2"}) {
		t.Errorf("bars = %v", got)
	}
	if got := ids(list(candidates, areaB, "", 0).Venues); !slices.Equal(got, []string{"zz-p2"}) {
		t.Errorf("in Beta = %v", got)
	}
	if page := list(candidates, "", "", 2); page.Total != 3 || !slices.Equal(ids(page.Venues), []string{"zz-p2"}) {
		t.Errorf("second page = %v (total %d)", ids(page.Venues), page.Total)
	}

	// Reject the snack bar (both its rows), approve the café.
	set := func(status isshov1.VenueStatus, places ...string) int32 {
		t.Helper()
		res, err := h.SetVenueStatus(adminCtx, connect.NewRequest(&isshov1.SetVenueStatusRequest{CityId: city, PlaceIds: places, Status: status}))
		if err != nil {
			t.Fatal(err)
		}
		return res.Msg.Changed
	}
	if n := set(isshov1.VenueStatus_VENUE_STATUS_REJECTED, "zz-p1", "elsewhere"); n != 1 {
		t.Errorf("rejected %d places, want 1", n)
	}
	if n := set(isshov1.VenueStatus_VENUE_STATUS_APPROVED, "zz-p3"); n != 1 {
		t.Errorf("approved %d places, want 1", n)
	}
	var rows int
	if err := pool.QueryRow(ctx, "select count(*) from app.venues where google_place_id = 'zz-p1' and status = 'rejected' and reviewed_by = $1", adminID).Scan(&rows); err != nil || rows != 2 {
		t.Errorf("p1 rows rejected by the admin = %d, %v", rows, err)
	}
	if c := cityRow(); c.Candidates != 1 || c.Approved != 1 || c.Rejected != 1 {
		t.Errorf("counts = %v", c)
	}
	if _, err := h.SetVenueStatus(adminCtx, connect.NewRequest(&isshov1.SetVenueStatusRequest{CityId: city, PlaceIds: []string{"zz-p2"}})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("no status: %v", err)
	}

	// The matcher: candidates are used until the switch is on.
	pick := func(area, category string) string {
		t.Helper()
		id, err := q.PickVenue(ctx, dbgen.PickVenueParams{AreaID: area, Categories: []string{category}})
		if errors.Is(err, pgx.ErrNoRows) {
			return ""
		}
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	counts := func() map[string]int32 {
		t.Helper()
		rows, err := q.ListCityVenueCounts(ctx, city)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]int32{}
		for _, r := range rows {
			out[r.AreaID+"/"+r.CategoryID] = r.Venues
		}
		return out
	}
	if pick(areaB, bar) != "zz-p2" || pick(areaA, cafe) != "zz-p3" || pick(areaA, bar) != "" {
		t.Errorf("picks with the switch off: %q %q %q", pick(areaB, bar), pick(areaA, cafe), pick(areaA, bar))
	}
	if c := counts(); c[areaB+"/"+bar] != 1 || c[areaA+"/"+cafe] != 1 {
		t.Errorf("counts with the switch off = %v", c)
	}
	if _, err := h.SetApprovedVenuesOnly(adminCtx, connect.NewRequest(&isshov1.SetApprovedVenuesOnlyRequest{CityId: city, ApprovedVenuesOnly: true})); err != nil {
		t.Fatal(err)
	}
	if pick(areaB, bar) != "" || pick(areaA, cafe) != "zz-p3" {
		t.Errorf("picks with the switch on: %q %q", pick(areaB, bar), pick(areaA, cafe))
	}
	if c := counts(); len(c) != 1 || c[areaA+"/"+cafe] != 1 {
		t.Errorf("counts with the switch on = %v", c)
	}
	if _, err := h.SetApprovedVenuesOnly(adminCtx, connect.NewRequest(&isshov1.SetApprovedVenuesOnlyRequest{CityId: "nowhere"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("an unknown city: %v", err)
	}

	// Rediscovery: the approved café found as a bar too is approved there.
	if err := q.UpsertVenue(ctx, dbgen.UpsertVenueParams{GooglePlaceID: "zz-p3", CityID: city, AreaID: areaA, CategoryID: bar}); err != nil {
		t.Fatal(err)
	}
	if pick(areaA, bar) != "zz-p3" {
		t.Error("a place's new category should take its approval")
	}

	// Search: one per area, each place once, known ones marked, new ones
	// with the categories they could go in.
	res, err := h.SearchVenues(adminCtx, connect.NewRequest(&isshov1.SearchVenuesRequest{CityId: city, Query: " スナック "}))
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]*isshov1.FoundVenue{}
	for _, v := range res.Msg.Venues {
		found[v.PlaceId] = v
	}
	if res.Msg.Searches != 2 || len(res.Msg.Venues) != 4 {
		t.Fatalf("search = %v", res.Msg)
	}
	if v := found["zz-p1"]; !v.Known || v.Status != isshov1.VenueStatus_VENUE_STATUS_REJECTED || len(v.AddableCategoryIds) != 0 {
		t.Errorf("known p1 = %v", v)
	}
	// Only the test's categories are checked: a seeded database has others.
	if v := found["zz-p4"]; v.Known || !slices.Contains(v.AddableCategoryIds, bar) || slices.Contains(v.AddableCategoryIds, cafe) || v.AreaId != areaA || v.Flag != "snack bar?" {
		t.Errorf("new p4 = %v", v)
	}
	if v := found["zz-p5"]; !v.Closed || len(v.AddableCategoryIds) != 0 {
		t.Errorf("closed p5 = %v", v)
	}
	if v := found["zz-p6"]; slices.Contains(v.AddableCategoryIds, cafe) {
		t.Errorf("far p6 addable in %v", v.AddableCategoryIds)
	}
	fake.searches = 0
	if _, err := h.SearchVenues(adminCtx, connect.NewRequest(&isshov1.SearchVenuesRequest{CityId: city, AreaId: areaB, Query: "bar"})); err != nil || fake.searches != 1 {
		t.Errorf("one area: %d searches, %v", fake.searches, err)
	}
	if _, err := h.SearchVenues(adminCtx, connect.NewRequest(&isshov1.SearchVenuesRequest{CityId: city, Query: "  "})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("an empty query: %v", err)
	}

	// Adding: checked with Google again, approved, in the nearest area.
	add := func(id, category string) error {
		_, err := h.AddVenue(adminCtx, connect.NewRequest(&isshov1.AddVenueRequest{CityId: city, PlaceId: id, CategoryId: category}))
		return err
	}
	for name, err := range map[string]error{
		"a café as a bar": add("zz-p6", bar), "too far": add("zz-p6", cafe), "closed": add("zz-p5", bar),
		"unknown category": add("zz-p4", "nope"), "unknown place": add("zz-p9", bar),
	} {
		if connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("adding %s: %v", name, err)
		}
	}
	if err := add("zz-p4", bar); err != nil {
		t.Fatal(err)
	}
	approved := list(isshov1.VenueStatus_VENUE_STATUS_APPROVED, "", "", 0)
	if !slices.Equal(ids(approved.Venues), []string{"zz-p3", "zz-p4"}) {
		t.Fatalf("approved = %v", ids(approved.Venues))
	}
	if v := approved.Venues[1]; !v.Added || v.AreaId != areaA || v.ReviewedAt == nil {
		t.Errorf("added p4 = %v", v)
	}
}
