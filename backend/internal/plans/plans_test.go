package plans_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/db"
	"github.com/shrujancheruku/issho/backend/fixtures"
	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/plans"
	"github.com/shrujancheruku/issho/backend/internal/testdb"
)

const (
	city     = "da-nang-hoi-an"
	lanterns = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0001"
	sunrise  = "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0002"
)

// seeded is a database with every migration and the fixture loaded the way
// development does: the city, then its plans.
func seeded(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool := testdb.NewDatabase(t)
	if err := db.Up(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err := cities.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	if err := plans.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	return pool
}

// The fixture seeds two plans, and the city lists both.
func TestTheCityListsItsTwoFixturePlans(t *testing.T) {
	pool := seeded(t)
	got, err := plans.New(pool).List(context.Background(), city, "en")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range got {
		ids = append(ids, p.ID)
	}
	// Ordered by start time: the sunrise plan starts at 06:30, the lanterns at 17:00.
	if want := []string{sunrise, lanterns}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

// A plan carries every field a plan page shows.
func TestAPlanCarriesTheFieldsAPlanPageShows(t *testing.T) {
	pool := seeded(t)
	got, err := plans.New(pool).Get(context.Background(), lanterns, "en")
	if err != nil {
		t.Fatal(err)
	}
	want := plans.Plan{
		ID: lanterns, City: city, Language: "en",
		Title:          "Hoi An old town at lantern time",
		Description:    "Walk the old town as the lanterns come on, eat cao lầu at a family stall, then finish with a drink by the river.",
		Price:          180000,
		MeetingAddress: "Nguyễn Thị Minh Khai, Cẩm Châu, Hội An",
		MeetingPoint:   "At the east end of the Japanese Covered Bridge, by the stone dog.",
		WhatToBring:    []string{"Comfortable walking shoes", "Cash for the stall", "A light rain jacket in the wet months"},
		PartsOfDay:     []string{"evening"},
		StartsAt:       "17:00", EndsAt: "20:30",
		Stops: []plans.Stop{
			{Position: 1, StayMinutes: 30, Name: "Chùa Cầu (Japanese Covered Bridge)", Note: "Meet here and say hello; the bridge is free to look at.", Leg: &plans.Leg{Mode: "walk", Minutes: 8}},
			{Position: 2, StayMinutes: 75, Name: "Quán cao lầu Bà Bé", Note: "Cao lầu is a Hoi An noodle dish; the stall has a vegetarian bowl.", Leg: &plans.Leg{Mode: "walk", Minutes: 6}},
			{Position: 3, StayMinutes: 60, Name: "Bờ sông Hoài", Note: "A drink on the riverside; the lantern boats go past."},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %+v\nwant   %+v", got, want)
	}
}

// The text comes back in the caller's language; the fields don't change.
func TestAPlansTextIsInTheCallersLanguage(t *testing.T) {
	pool := seeded(t)
	svc := plans.New(pool)
	en, err := svc.Get(context.Background(), sunrise, "en")
	if err != nil {
		t.Fatal(err)
	}
	vi, err := svc.Get(context.Background(), sunrise, "vi")
	if err != nil {
		t.Fatal(err)
	}
	if vi.Language != "vi" || vi.Title != "Bình minh ở Mỹ Khê, rồi bánh mì và cà phê" {
		t.Fatalf("vi title = %q (%s)", vi.Title, vi.Language)
	}
	if got, want := vi.Stops[1].Name, "Bánh mì Bà Lan"; got != want {
		t.Fatalf("vi stop name = %q, want %q", got, want)
	}
	if vi.Stops[0].Note == en.Stops[0].Note || vi.WhatToBring[0] == en.WhatToBring[0] {
		t.Fatalf("vi and en text are the same: %+v", vi)
	}
	// What must be true on the day is a field, not prose: it is the same in both.
	vi.Language, vi.Title, vi.Description, vi.MeetingPoint, vi.WhatToBring = en.Language, en.Title, en.Description, en.MeetingPoint, en.WhatToBring
	for i := range vi.Stops {
		vi.Stops[i].Name, vi.Stops[i].Note = en.Stops[i].Name, en.Stops[i].Note
	}
	if !reflect.DeepEqual(vi, en) {
		t.Fatalf("fields differ by language:\nvi %+v\nen %+v", vi, en)
	}
}

// A plan with no text in the caller's language reads in English, the fallback
// for every language (docs/spec/languages.md).
func TestAPlanWithNoTextInTheLanguageReadsInEnglish(t *testing.T) {
	ctx := context.Background()
	pool := seeded(t)
	if _, err := pool.Exec(ctx, `delete from app.plan_stop_texts where plan_id = $1 and language = 'vi'`, lanterns); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `delete from app.plan_texts where plan_id = $1 and language = 'vi'`, lanterns); err != nil {
		t.Fatal(err)
	}
	got, err := plans.New(pool).Get(ctx, lanterns, "vi")
	if err != nil {
		t.Fatal(err)
	}
	if got.Language != "en" || got.Title != "Hoi An old town at lantern time" || got.Stops[0].Name != "Chùa Cầu (Japanese Covered Bridge)" {
		t.Fatalf("plan = %+v", got)
	}
}

// A plan that doesn't exist is ErrNotFound, never an empty plan; so is an id
// that couldn't be one.
func TestAPlanThatDoesNotExistIsNotFound(t *testing.T) {
	pool := seeded(t)
	for _, id := range []string{"6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b00ff", "", "not-an-id", lanterns + "'; drop table app.plans; --"} {
		got, err := plans.New(pool).Get(context.Background(), id, "en")
		if !errors.Is(err, plans.ErrNotFound) {
			t.Errorf("Get(%q) = %+v, %v; want ErrNotFound", id, got, err)
		}
	}
}

// A city with no plans lists none, and loading the fixture twice leaves two.
func TestAnEmptyCityListsNoPlans(t *testing.T) {
	ctx := context.Background()
	pool := seeded(t)
	if _, err := pool.Exec(ctx, `insert into app.cities (slug, state, time_zone, currency) values ('hue', 'planned', 'Asia/Ho_Chi_Minh', 'VND')`); err != nil {
		t.Fatal(err)
	}
	got, err := plans.New(pool).List(ctx, "hue", "en")
	if err != nil || len(got) != 0 {
		t.Fatalf("List = %v, %v; want none", got, err)
	}
}

func TestLoadingThePlansTwiceChangesNothing(t *testing.T) {
	ctx := context.Background()
	pool := seeded(t)
	if err := plans.LoadFixture(ctx, pool, fixtures.DaNangHoiAn); err != nil {
		t.Fatal(err)
	}
	got, err := plans.New(pool).List(ctx, city, "en")
	if err != nil || len(got) != 2 {
		t.Fatalf("List = %d plans, %v; want 2", len(got), err)
	}
	if len(got[1].Stops) != 3 {
		t.Fatalf("stops = %d, want 3 (not doubled)", len(got[1].Stops))
	}
}

// A fixture that says something this package doesn't know is refused.
func TestAFixtureWithAnUnknownFieldIsRefused(t *testing.T) {
	pool := seeded(t)
	bad := []byte(`{"plans": [{"id": "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0009", "colour": "red"}]}`)
	if err := plans.LoadFixture(context.Background(), pool, bad); err == nil {
		t.Fatal("LoadFixture accepted an unknown field")
	}
}

// A plan's rounds are counted from its opening day, so a fixture plan without
// one, or with one that isn't a date, is refused rather than given a day.
func TestAFixturePlanWithoutAValidOpeningDayIsRefused(t *testing.T) {
	pool := seeded(t)
	for _, opens := range []string{``, `"opens_on": "",`, `"opens_on": "5 January",`} {
		bad := []byte(`{"plans": [{"id": "6a1f0c52-3d1b-4f0e-9a52-0d7c9a1b0009", "city": "da-nang-hoi-an", "price": 1,
			"meeting_address": "x", "parts_of_day": ["evening"], "starts_at": "17:00", "ends_at": "18:00", ` + opens + `
			"texts": {}, "stops": []}]}`)
		if err := plans.LoadFixture(context.Background(), pool, bad); err == nil {
			t.Errorf("LoadFixture accepted a plan with %q", opens)
		}
	}
}
