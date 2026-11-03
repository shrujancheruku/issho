package curation

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/geo"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/maps/mapstest"
)

func TestFits(t *testing.T) {
	cafe := dbgen.ListAllVenueCategoriesRow{ID: "cafe", GoogleTypes: []string{"cafe", "coffee_shop"}, SearchRadiusM: 1000}
	centre := maps.LatLng{Lat: 35.7, Lng: 139.65}
	// 0.01° of longitude at 35.7°N is about 900 m.
	east := func(deg float64) maps.LatLng { return maps.LatLng{Lat: centre.Lat, Lng: centre.Lng + deg} }
	// A square boundary about 2 km wide, much larger than the 1 km radius.
	square := maps.Boundary{{
		{Lat: 35.69, Lng: 139.64}, {Lat: 35.69, Lng: 139.66}, {Lat: 35.71, Lng: 139.66}, {Lat: 35.71, Lng: 139.64},
	}}
	plain := area{Area: geo.Area{ID: "a", LatLng: centre}}
	bounded := area{Area: geo.Area{ID: "a", LatLng: centre, Boundary: square}}
	place := func(loc maps.LatLng, types ...string) maps.Place { return maps.Place{Location: loc, Types: types} }

	for _, tc := range []struct {
		name string
		cat  dbgen.ListAllVenueCategoriesRow
		p    maps.Place
		near area
		want bool
	}{
		{"primary type, close", cafe, place(east(0.005), "cafe"), plain, true},
		{"a secondary type counts", cafe, place(east(0.005), "establishment", "coffee_shop"), plain, true},
		{"no matching type", cafe, place(east(0.005), "bar"), plain, false},
		{"no types at all", cafe, place(east(0.005)), plain, false},
		{"too far from the point", cafe, place(east(0.02), "cafe"), plain, false},
		{"far from the point but inside the boundary", cafe, place(east(0.0095), "cafe"), bounded, true},
		{"outside the boundary and the radius", cafe, place(east(0.02), "cafe"), bounded, false},
		{"right type is not enough outside", cafe, place(east(0.02), "bar"), bounded, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := fits(tc.cat, tc.p, tc.near); got != tc.want {
				t.Errorf("fits = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNearest(t *testing.T) {
	a := area{Area: geo.Area{ID: "a", LatLng: maps.LatLng{Lat: 35.70, Lng: 139.60}}, Name: "A"}
	b := area{Area: geo.Area{ID: "b", LatLng: maps.LatLng{Lat: 35.70, Lng: 139.70}}, Name: "B"}
	// c's boundary holds a point that is nearer to a's centre.
	c := area{
		Area: geo.Area{ID: "c", LatLng: maps.LatLng{Lat: 35.80, Lng: 139.60}, Boundary: maps.Boundary{{
			{Lat: 35.68, Lng: 139.59}, {Lat: 35.68, Lng: 139.62}, {Lat: 35.82, Lng: 139.62}, {Lat: 35.82, Lng: 139.59},
		}}},
		Name: "C",
	}
	for _, tc := range []struct {
		name  string
		areas []area
		p     maps.LatLng
		want  string
	}{
		{"nearest by point", []area{a, b}, maps.LatLng{Lat: 35.70, Lng: 139.68}, "b"},
		{"the only area", []area{a}, maps.LatLng{Lat: 36, Lng: 140}, "a"},
		{"the boundary beats the nearer point", []area{a, c}, maps.LatLng{Lat: 35.69, Lng: 139.605}, "c"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := nearest(tc.areas, tc.p); got.ID != tc.want {
				t.Errorf("nearest = %s, want %s", got.ID, tc.want)
			}
		})
	}
}

// fixture is a throwaway city of two areas ("Alpha" and "Beta", about 4.5 km
// apart) with two categories and three discovered places, and a fake Google
// that knows six places.
type fixture struct {
	pool            *pgxpool.Pool
	city, bar, cafe string
	areaA, areaB    string
	locA, locB      maps.LatLng
	fake            *mapstest.Fake
	svc             *Service
	adminID         uuid.UUID
	now             time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
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

	f := &fixture{pool: pool, city: "zz-curation", bar: "zz-curation-bar", cafe: "zz-curation-cafe", adminID: uuid.New(),
		now: time.Date(2030, 1, 1, 9, 0, 0, 0, time.UTC)}
	f.areaA, f.areaB = f.city+"-a", f.city+"-b"
	// Areas about 4.5 km apart (0.05° of longitude at 35.7°N).
	f.locA, f.locB = maps.LatLng{Lat: 35.7, Lng: 139.65}, maps.LatLng{Lat: 35.7, Lng: 139.70}
	cleanup := func() {
		// The city cascades to its areas and venues; categories last.
		for _, s := range [][]any{
			{"delete from app.cities where id = $1", f.city},
			{"delete from app.venue_categories where id = any($1)", []string{f.bar, f.cafe}},
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
		  values ($1, '{"en":"Curation"}', 'Asia/Tokyo', 'JPY', '{ja}', '{walk}')`, f.city},
		{`insert into app.areas (id, city_id, name, centroid) values ($1, $2, '{"en":"Alpha"}', st_point($3, $4)), ($5, $2, '{"en":"Beta"}', st_point($6, $4))`,
			f.areaA, f.city, f.locA.Lng, f.locA.Lat, f.areaB, f.locB.Lng},
		{`insert into app.venue_categories (id, name, google_types, search_radius_m)
		  values ($1, '{"en":"Bar"}', '{bar}', 1500), ($2, '{"en":"Cafe"}', '{cafe,coffee_shop}', 1500)`, f.bar, f.cafe},
		// p1 is a snack bar that's also typed a café; p2 a bar in Beta; p3 a café.
		{`insert into app.venues (google_place_id, city_id, area_id, category_id) values
		  ('zz-p1', $1, $2, $4), ('zz-p1', $1, $2, $5), ('zz-p2', $1, $3, $4), ('zz-p3', $1, $2, $5)`, f.city, f.areaA, f.areaB, f.bar, f.cafe},
	} {
		if _, err := pool.Exec(ctx, s[0].(string), s[1:]...); err != nil {
			t.Fatal(err)
		}
	}
	near := func(p maps.LatLng, dLng float64) maps.LatLng { return maps.LatLng{Lat: p.Lat, Lng: p.Lng + dLng} }
	place := func(id, name string, loc maps.LatLng, closed bool, types ...string) maps.SearchResult {
		return maps.SearchResult{
			Place:   maps.Place{ID: id, PrimaryType: types[0], Types: types, Location: loc},
			Details: maps.PlaceDetails{Name: name, Address: name + " street", MapsURL: "https://maps.example/" + id, Location: loc},
			Closed:  closed,
		}
	}
	f.fake = &mapstest.Fake{Places: map[string]maps.SearchResult{
		"zz-p1": place("zz-p1", "スナック ゆき", f.locA, false, "bar", "cafe"),
		"zz-p2": place("zz-p2", "Bar Beta", f.locB, false, "bar"),
		"zz-p3": place("zz-p3", "Kissa Alpha", near(f.locA, 0.001), false, "cafe"),
		// New: a bar near Alpha, a closed one, and a café 3 km out of Beta.
		"zz-p4": place("zz-p4", "Snack Mimi", near(f.locA, 0.002), false, "bar", "establishment"),
		"zz-p5": place("zz-p5", "Gone Bar", near(f.locA, 0.003), true, "bar"),
		"zz-p6": place("zz-p6", "Far Cafe", near(f.locB, 0.033), false, "cafe"),
	}, Found: map[maps.LatLng][]string{
		f.locA: {"zz-p1", "zz-p4", "zz-p5"},
		f.locB: {"zz-p4", "zz-p6"},
	}}
	f.svc = &Service{DB: pool, Places: f.fake, Now: func() time.Time { return f.now }}
	return f
}

func (f *fixture) cityRow(t *testing.T) dbgen.ListVenueCitiesRow {
	t.Helper()
	cities, _, _, err := f.svc.Cities(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cities {
		if c.ID == f.city {
			return c
		}
	}
	t.Fatal("city missing")
	return dbgen.ListVenueCitiesRow{}
}

func codeOf(err error) connect.Code { return connect.CodeOf(err) }

// TestIntegrationService: listing a city's places with live names and
// flags, filters and pages; approving and rejecting a place in all its
// categories (and counting the places changed, not rows); the city's switch
// as the matcher sees it; a new category inheriting the place's status;
// searching by name; adding a place. Skipped unless TEST_DATABASE_URL is set.
func TestIntegrationService(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	q := dbgen.New(f.pool)
	svc := f.svc

	if c := f.cityRow(t); c.Candidates != 3 || c.Approved != 0 || c.ApprovedVenuesOnly {
		t.Fatalf("city = %+v", c)
	}
	page := func(status, area, category string, size int32, token string) ([]Listed, int32, string) {
		t.Helper()
		ls, total, next, err := svc.List(ctx, ListParams{CityID: f.city, Status: status, AreaID: area, CategoryID: category, PageSize: size, Token: token})
		if err != nil {
			t.Fatal(err)
		}
		return ls, total, next
	}
	list := func(status, area, category string) ([]Listed, int32) {
		t.Helper()
		ls, total, next := page(status, area, category, 0, "")
		if next != "" {
			t.Fatalf("one page expected, got next %q", next)
		}
		return ls, total
	}
	ids := func(ls []Listed) []string {
		var out []string
		for _, l := range ls {
			out = append(out, l.GooglePlaceID)
		}
		return out
	}
	all, total := list("candidate", "", "")
	if total != 3 || !slices.Equal(ids(all), []string{"zz-p1", "zz-p3", "zz-p2"}) {
		t.Fatalf("candidates = %v (total %d)", ids(all), total)
	}
	if v := all[0]; v.Name != "スナック ゆき" || v.Flag != "snack bar?" || v.AreaName != "Alpha" || len(v.Categories) != 2 || v.MapsURL == "" {
		t.Errorf("p1 = %+v", v)
	}
	if got, _ := list("candidate", "", f.bar); !slices.Equal(ids(got), []string{"zz-p1", "zz-p2"}) {
		t.Errorf("bars = %v", ids(got))
	}
	if got, _ := list("candidate", f.areaB, ""); !slices.Equal(ids(got), []string{"zz-p2"}) {
		t.Errorf("in Beta = %v", ids(got))
	}
	// Paging: total stays the whole count on every page; the token walks on.
	first, total, next := page("candidate", "", "", 2, "")
	if total != 3 || next == "" || !slices.Equal(ids(first), []string{"zz-p1", "zz-p3"}) {
		t.Fatalf("first page = %v (total %d, next %q)", ids(first), total, next)
	}
	if second, total, end := page("candidate", "", "", 2, next); total != 3 || end != "" || !slices.Equal(ids(second), []string{"zz-p2"}) {
		t.Errorf("second page = %v (total %d, next %q)", ids(second), total, end)
	}
	for name, p := range map[string]ListParams{
		"negative size":  {CityID: f.city, Status: "candidate", PageSize: -1},
		"garbage token":  {CityID: f.city, Status: "candidate", Token: "garbage"},
		"another filter": {CityID: f.city, Status: "candidate", CategoryID: f.bar, Token: next},
		"another status": {CityID: f.city, Status: "approved", Token: next},
	} {
		if _, _, _, err := svc.List(ctx, p); codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: %v", name, err)
		}
	}

	// Reject the snack bar (both its rows), approve the café.
	set := func(status string, places ...string) int {
		t.Helper()
		n, err := svc.SetStatus(ctx, f.city, places, status, f.adminID)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	// p1 has two rows and "elsewhere" none: one place changed.
	if n := set("rejected", "zz-p1", "elsewhere"); n != 1 {
		t.Errorf("rejected %d places, want 1", n)
	}
	if n := set("approved", "zz-p3"); n != 1 {
		t.Errorf("approved %d places, want 1", n)
	}
	if n := set("approved", "elsewhere", "also-elsewhere"); n != 0 {
		t.Errorf("approved %d unknown places, want 0", n)
	}
	var rows int
	if err := f.pool.QueryRow(ctx, "select count(*) from app.venues where google_place_id = 'zz-p1' and status = 'rejected' and reviewed_by = $1", f.adminID).Scan(&rows); err != nil || rows != 2 {
		t.Errorf("p1 rows rejected by the admin = %d, %v", rows, err)
	}
	if c := f.cityRow(t); c.Candidates != 1 || c.Approved != 1 || c.Rejected != 1 {
		t.Errorf("counts = %+v", c)
	}
	// Back to candidate clears who and when.
	set("candidate", "zz-p1")
	if err := f.pool.QueryRow(ctx, "select count(*) from app.venues where google_place_id = 'zz-p1' and status = 'candidate' and reviewed_by is null and reviewed_at is null").Scan(&rows); err != nil || rows != 2 {
		t.Errorf("p1 rows reset = %d, %v", rows, err)
	}
	set("rejected", "zz-p1")
	if _, err := svc.SetStatus(ctx, f.city, nil, "approved", f.adminID); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("no places: %v", err)
	}
	if _, err := svc.SetStatus(ctx, f.city, make([]string, MaxBatch+1), "approved", f.adminID); codeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("too many places: %v", err)
	}
	if _, err := svc.SetStatus(ctx, "nowhere", []string{"zz-p1"}, "approved", f.adminID); codeOf(err) != connect.CodeNotFound {
		t.Errorf("an unknown city: %v", err)
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
		rows, err := q.ListCityVenueCounts(ctx, f.city)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]int32{}
		for _, r := range rows {
			out[r.AreaID+"/"+r.CategoryID] = r.Venues
		}
		return out
	}
	if pick(f.areaB, f.bar) != "zz-p2" || pick(f.areaA, f.cafe) != "zz-p3" || pick(f.areaA, f.bar) != "" {
		t.Errorf("picks with the switch off: %q %q %q", pick(f.areaB, f.bar), pick(f.areaA, f.cafe), pick(f.areaA, f.bar))
	}
	if c := counts(); c[f.areaB+"/"+f.bar] != 1 || c[f.areaA+"/"+f.cafe] != 1 {
		t.Errorf("counts with the switch off = %v", c)
	}
	if err := svc.SetApprovedOnly(ctx, f.city, true); err != nil {
		t.Fatal(err)
	}
	if pick(f.areaB, f.bar) != "" || pick(f.areaA, f.cafe) != "zz-p3" {
		t.Errorf("picks with the switch on: %q %q", pick(f.areaB, f.bar), pick(f.areaA, f.cafe))
	}
	if c := counts(); len(c) != 1 || c[f.areaA+"/"+f.cafe] != 1 {
		t.Errorf("counts with the switch on = %v", c)
	}
	if err := svc.SetApprovedOnly(ctx, "nowhere", true); codeOf(err) != connect.CodeNotFound {
		t.Errorf("an unknown city: %v", err)
	}

	// Rediscovery: the approved café found as a bar too is approved there.
	if err := q.UpsertVenue(ctx, dbgen.UpsertVenueParams{GooglePlaceID: "zz-p3", CityID: f.city, AreaID: f.areaA, CategoryID: f.bar}); err != nil {
		t.Fatal(err)
	}
	if pick(f.areaA, f.bar) != "zz-p3" {
		t.Error("a place's new category should take its approval")
	}

	// Search: one per area, each place once, known ones marked, new ones
	// with the categories they could go in.
	res, err := svc.Search(ctx, SearchParams{CityID: f.city, Query: " スナック "})
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]Found{}
	for _, v := range res.Venues {
		found[v.ID] = v
	}
	if res.Searches != 2 || len(res.Venues) != 4 {
		t.Fatalf("search = %+v", res)
	}
	if v := found["zz-p1"]; !v.Known || v.Status != "rejected" || len(v.Addable) != 0 {
		t.Errorf("known p1 = %+v", v)
	}
	// Only the test's categories are checked: a seeded database has others.
	if v := found["zz-p4"]; v.Known || !slices.Contains(v.Addable, f.bar) || slices.Contains(v.Addable, f.cafe) || v.AreaID != f.areaA {
		t.Errorf("new p4 = %+v", v)
	}
	if v := found["zz-p5"]; !v.Closed || len(v.Addable) != 0 {
		t.Errorf("closed p5 = %+v", v)
	}
	if v := found["zz-p6"]; slices.Contains(v.Addable, f.cafe) {
		t.Errorf("far p6 addable in %v", v.Addable)
	}
	before := f.fake.Searches()
	if _, err := svc.Search(ctx, SearchParams{CityID: f.city, AreaID: f.areaB, Query: "bar"}); err != nil || f.fake.Searches()-before != 1 {
		t.Errorf("one area: %d searches, %v", f.fake.Searches()-before, err)
	}
	for name, p := range map[string]SearchParams{
		"an empty query":   {CityID: f.city, Query: "  "},
		"a long query":     {CityID: f.city, Query: strings.Repeat("x", MaxQuery+1)},
		"a foreign area":   {CityID: f.city, AreaID: "zz-nope", Query: "bar"},
		"a different city": {CityID: "nowhere", Query: "bar"},
	} {
		want := connect.CodeInvalidArgument
		if name == "a different city" {
			want = connect.CodeNotFound
		}
		if _, err := svc.Search(ctx, p); codeOf(err) != want {
			t.Errorf("search with %s: %v", name, err)
		}
	}

	// Adding: checked with Google again, approved, in the nearest area.
	add := func(id, category string) error {
		_, err := svc.Add(ctx, f.city, id, category, f.adminID)
		return err
	}
	for name, err := range map[string]error{
		"a café as a bar": add("zz-p6", f.bar), "too far": add("zz-p6", f.cafe), "closed": add("zz-p5", f.bar),
		"unknown category": add("zz-p4", "nope"), "unknown place": add("zz-p9", f.bar), "a bad id": add("a/b", f.bar),
	} {
		if codeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("adding %s: %v", name, err)
		}
	}
	area, err := svc.Add(ctx, f.city, "zz-p4", f.bar, f.adminID)
	if err != nil || area != f.areaA {
		t.Fatalf("add p4 = %q, %v", area, err)
	}
	approved, _ := list("approved", "", "")
	if !slices.Equal(ids(approved), []string{"zz-p3", "zz-p4"}) {
		t.Fatalf("approved = %v", ids(approved))
	}
	if v := approved[1]; !v.Added || v.AreaID != f.areaA || !v.ReviewedAt.Valid || !v.ReviewedAt.Time.Equal(f.now) {
		t.Errorf("added p4 = %+v", v)
	}
	// Another city's place can't be taken over.
	if _, err := f.pool.Exec(ctx, `insert into app.cities (id, name, timezone, currency, languages, travel_modes)
		values ('zz-curation-2', '{"en":"Other"}', 'Asia/Tokyo', 'JPY', '{ja}', '{walk}')`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = f.pool.Exec(ctx, "delete from app.cities where id = 'zz-curation-2'") })
	if _, err := f.pool.Exec(ctx, `insert into app.areas (id, city_id, name, centroid) values ('zz-curation-2-a', 'zz-curation-2', '{"en":"Other"}', st_point($1, $2))`, f.locA.Lng, f.locA.Lat); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Add(ctx, "zz-curation-2", "zz-p4", f.bar, f.adminID); codeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("adding another city's place: %v", err)
	}
}

// TestIntegrationUpstreamErrors: Google's error text stays in the log, and a
// service without a Places provider refuses what needs one.
func TestIntegrationUpstreamErrors(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.fake.SearchErr = errors.New("quota exceeded for key AIza-secret")
	_, err := f.svc.Search(ctx, SearchParams{CityID: f.city, Query: "bar"})
	if codeOf(err) != connect.CodeUnavailable || strings.Contains(err.Error(), "AIza") || strings.Contains(err.Error(), "quota") {
		t.Errorf("search failure = %v", err)
	}
	f.svc.Places = nil
	if _, err := f.svc.Search(ctx, SearchParams{CityID: f.city, Query: "bar"}); codeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("search without a key: %v", err)
	}
	if _, err := f.svc.Add(ctx, f.city, "zz-p4", f.bar, f.adminID); codeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("add without a key: %v", err)
	}
	// Listing still works, with blank names.
	ls, _, _, err := f.svc.List(ctx, ListParams{CityID: f.city, Status: "candidate"})
	if err != nil || len(ls) == 0 || ls[0].Name != "" {
		t.Errorf("list without a key: %v, %v", ls, err)
	}
}
