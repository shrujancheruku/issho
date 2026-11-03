package cities

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

func TestPlanAreas(t *testing.T) {
	was := map[string]maps.LatLng{"x-a": {}, "x-b": {}, "x-c": {}, "x-d": {}}
	p := Pack{ID: "x", Areas: []Area{
		{ID: "x-a"},
		{ID: "x-n", Replaces: []string{"x-b", "x-gone"}}, // x-gone was applied before
	}}
	plan := planAreas(was, p)
	// x-c and x-d are dropped without a replacement; x-b is replaced.
	if !slices.Equal(plan.removed, []string{"x-c", "x-d"}) {
		t.Errorf("removed = %v", plan.removed)
	}
	if len(plan.replaced) != 1 || plan.replaced[0].Old != "x-b" || plan.replaced[0].New != "x-n" {
		t.Errorf("replaced = %+v", plan.replaced)
	}
	if !slices.Equal(plan.replaces, []string{"x-b", "x-gone"}) {
		t.Errorf("replaces = %v", plan.replaces)
	}
}

// TestIntegrationAuthoritative: applying a pack makes the database match it,
// refuses what would lose people's data, and moves it when told where. Uses a
// synthetic city, not Tokyo. Skipped unless TEST_DATABASE_URL is set.
func TestIntegrationAuthoritative(t *testing.T) {
	const city = "zz-auth"
	f := testfixture.New(t, city, "Asia/Tokyo", 1)
	ctx := context.Background()
	pool := f.Pool
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}
	count := func(sql string, args ...any) (n int) {
		t.Helper()
		if err := pool.QueryRow(ctx, sql, args...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return n
	}
	const other, otherArea, cat = "zz-auth-other", "zz-auth-other-x", "zz-auth-cafe"
	cleanup := func() {
		exec("delete from app.cities where id = $1", other)
		exec("delete from app.venue_categories where id = $1", cat)
	}
	cleanup()
	t.Cleanup(cleanup)
	exec(`insert into app.venue_categories (id, name, google_types) values ($1, '{"en":"Cafe"}', '{cafe}')`, cat)
	exec(`insert into app.cities (id, name, timezone, currency, languages, travel_modes) values ($1, '{"en":"Other"}', 'Asia/Tokyo', 'JPY', '{ja}', '{walk}')`, other)
	exec(`insert into app.areas (id, city_id, name, centroid) values ($1, $2, '{"en":"X"}', 'POINT(0 0)')`, otherArea, other)

	area := func(suffix string, lat float64, replaces ...string) Area {
		return Area{ID: city + "-" + suffix, Name: Text{"en": suffix}, Lat: lat, Lng: 139.6, Replaces: replaces}
	}
	pack := Pack{
		ID: city, Name: Text{"en": "Auth"}, Timezone: "Asia/Tokyo", Currency: "JPY", Languages: []string{"ja"}, TravelModes: []string{"walk"},
		Areas:             []Area{area("a", 35.70), area("b", 35.71), area("c", 35.72), area("d", 35.73)},
		ActivityOverrides: []ActivityOverride{{ActivityID: f.Act, Disabled: true}},
	}
	apply := func(p Pack, opts Options) (Diff, error) {
		t.Helper()
		return Apply(ctx, pool, nil, p, opts)
	}
	if _, err := apply(pack, Options{}); err != nil {
		t.Fatal(err)
	}
	if n := count("select count(*) from app.city_activity_overrides where city_id = $1 and enabled = false", city); n != 1 {
		t.Fatalf("override not applied: %d", n)
	}

	// Removing an override re-enables the activity (the row goes, so the
	// catalogue default applies), and the Diff says so.
	pack.ActivityOverrides = nil
	d, err := apply(pack, Options{})
	if err != nil || !slices.Equal(d.RemovedOverrides, []string{f.Act}) {
		t.Fatalf("removing an override = %+v, %v", d, err)
	}
	if n := count("select count(*) from app.city_activity_overrides where city_id = $1", city); n != 0 {
		t.Errorf("override rows left: %d", n)
	}

	// An unreferenced removed area goes with its venues and travel times.
	exec(`insert into app.venues (google_place_id, city_id, area_id, category_id) values ('zz-auth-p1', $1, $2, $3)`, city, city+"-d", cat)
	exec(`insert into app.area_travel_times (from_area_id, to_area_id, mode, minutes, computed_at) values ($1, $2, 'walk', 9, now())`, city+"-d", city+"-a")
	full := pack
	pack.Areas = pack.Areas[:3]
	d, err = apply(pack, Options{})
	if err != nil || !slices.Equal(d.RemovedAreas, []string{city + "-d"}) {
		t.Fatalf("removing an unreferenced area = %+v, %v", d, err)
	}
	if n := count("select count(*) from app.areas where id = $1", city+"-d") + count("select count(*) from app.venues where area_id = $1", city+"-d") +
		count("select count(*) from app.area_travel_times where from_area_id = $1", city+"-d"); n != 0 {
		t.Errorf("rows of the removed area left: %d", n)
	}

	// A referenced removed area is refused with counts, and nothing is
	// written (the override change that comes with it is not applied).
	f.AddProfile(t, f.Users[0], "ja")
	exec("update app.profiles set home_area_id = $1 where user_id = $2", city+"-b", f.Users[0])
	exec(`insert into app.crews (city_id, area_id, offer_until) values ($1, $2, now())`, city, city+"-b")
	exec(`insert into app.plans (city_id, area_id, activity_id, primary_language, audience, score, window_start, window_days, signup_deadline)
	      values ($1, $2, $3, 'ja', 1, 1, current_date, 7, now())`, city, city+"-b", f.Act)
	exec(`insert into app.city_activity_overrides (city_id, activity_id, enabled) values ($1, $2, false)`, city, f.Act)
	pack.Areas = []Area{area("a", 35.70), area("c", 35.72)}
	if _, err := apply(pack, Options{}); !errors.Is(err, ErrRefused) ||
		!strings.Contains(err.Error(), city+"-b: 1 people, 1 crews, 1 plans; list it in an area's replaces") {
		t.Fatalf("a referenced area: %v", err)
	}
	if n := count("select count(*) from app.areas where id = $1", city+"-b") + count("select count(*) from app.city_activity_overrides where city_id = $1", city); n != 2 {
		t.Errorf("a refused apply wrote something (%d)", n)
	}

	// A replaced id from another city is refused too, and nothing of it is
	// deleted.
	bad := pack
	bad.Areas = []Area{area("a", 35.70), area("b", 35.71), area("c", 35.72, otherArea)}
	if _, err := apply(bad, Options{}); !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "belongs to city "+other) {
		t.Errorf("a replaced id of another city: %v", err)
	}
	if n := count("select count(*) from app.areas where id = $1", otherArea); n != 1 {
		t.Errorf("another city's area was deleted")
	}

	// replaces moves people, crews and plans, then deletes the old area.
	pack.Areas = []Area{area("a", 35.70), area("c", 35.72, city+"-b", city+"-gone")}
	d, err = apply(pack, Options{})
	want := ReplacedArea{Old: city + "-b", New: city + "-c", People: 1, Crews: 1, Plans: 1}
	if err != nil || len(d.ReplacedAreas) != 1 || d.ReplacedAreas[0] != want || len(d.RemovedAreas) != 0 {
		t.Fatalf("replacing = %+v, %v", d, err)
	}
	if n := count("select count(*) from app.profiles where home_area_id = $1", city+"-c") +
		count("select count(*) from app.crews where area_id = $1", city+"-c") +
		count("select count(*) from app.plans where area_id = $1", city+"-c"); n != 3 {
		t.Errorf("moved rows = %d, want 3", n)
	}
	if n := count("select count(*) from app.areas where id = $1", city+"-b"); n != 0 {
		t.Errorf("the replaced area is still there")
	}
	// Re-applying with the replaces entry kept is a no-op.
	if d, err := apply(pack, Options{}); err != nil || len(d.ReplacedAreas)+len(d.RemovedAreas)+len(d.RemovedOverrides) != 0 {
		t.Errorf("re-apply = %+v, %v", d, err)
	}

	// Curated venues are not deleted silently: refused unless allowed.
	pack.Areas = append(pack.Areas, full.Areas[3])
	if _, err := apply(pack, Options{}); err != nil {
		t.Fatal(err)
	}
	exec(`insert into app.venues (google_place_id, city_id, area_id, category_id, status) values ('zz-auth-p2', $1, $2, $3, 'approved')`, city, city+"-d", cat)
	pack.Areas = pack.Areas[:2]
	if _, err := apply(pack, Options{}); !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), "1 curated venues") {
		t.Fatalf("curated venues: %v", err)
	}
	if n := count("select count(*) from app.venues where area_id = $1", city+"-d"); n != 1 {
		t.Errorf("a refused apply deleted a venue")
	}
	if d, err := apply(pack, Options{DropCuratedVenues: true}); err != nil || !slices.Equal(d.RemovedAreas, []string{city + "-d"}) {
		t.Errorf("allowed: %+v, %v", d, err)
	}

	// The same holds for a *replaced* area: its approved venue would cascade.
	pack.Areas = append(pack.Areas, area("e", 35.74))
	if _, err := apply(pack, Options{}); err != nil {
		t.Fatal(err)
	}
	exec(`insert into app.venues (google_place_id, city_id, area_id, category_id, status) values ('zz-auth-p3', $1, $2, $3, 'approved')`, city, city+"-e", cat)
	pack.Areas = append(pack.Areas[:2], area("f", 35.75, city+"-e"))
	if _, err := apply(pack, Options{}); !errors.Is(err, ErrRefused) || !strings.Contains(err.Error(), city+"-e: 1 curated venues") {
		t.Fatalf("curated venues of a replaced area: %v", err)
	}
	if n := count("select count(*) from app.areas where id = $1", city+"-e") + count("select count(*) from app.venues where area_id = $1", city+"-e"); n != 2 {
		t.Errorf("a refused apply deleted the replaced area or its venue (%d)", n)
	}
	if d, err := apply(pack, Options{DropCuratedVenues: true}); err != nil || len(d.ReplacedAreas) != 1 || d.ReplacedAreas[0].Old != city+"-e" {
		t.Errorf("allowed: %+v, %v", d, err)
	}
	if n := count("select count(*) from app.areas where id = $1", city+"-e"); n != 0 {
		t.Errorf("the replaced area is still there")
	}
}
