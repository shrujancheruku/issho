package cities

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/maps/google"
)

// TestDataFiles validates the real catalogue and every city pack in data/, so
// a bad edit fails CI instead of failing at seed time or on a paid API call.
func TestDataFiles(t *testing.T) {
	const dataDir = "../../../data"
	var cats []VenueCategory
	if err := readJSON(filepath.Join(dataDir, "venue-categories.json"), &cats); err != nil {
		t.Fatal(err)
	}
	var acts []Activity
	if err := readJSON(filepath.Join(dataDir, "activities.json"), &acts); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCatalog(cats, acts); err != nil {
		t.Errorf("catalogue:\n%v", err)
	}
	for _, c := range cats {
		for _, gt := range c.GoogleTypes {
			if !google.IsPlaceType(gt) {
				t.Errorf("venue category %s: %q is not a Places API (New) Table A type", c.ID, gt)
			}
		}
	}

	ids := map[string]bool{}
	for _, a := range acts {
		ids[a.ID] = true
	}
	packs, _ := filepath.Glob(filepath.Join(dataDir, "citypacks", "*.json"))
	if len(packs) < 2 {
		t.Fatalf("expected at least 2 city packs, found %d", len(packs))
	}
	for _, path := range packs {
		var p Pack
		if err := readJSON(path, &p); err != nil {
			t.Fatal(err)
		}
		if want := filepath.Base(path); want != p.ID+".json" {
			t.Errorf("%s: file name should be %s.json", path, p.ID)
		}
		if err := ValidatePack(p, ids); err != nil {
			t.Errorf("%s:\n%v", path, err)
		}
		// The admin SPA rewrites packs with EncodePack: it must reproduce
		// the file exactly, so its pull requests change only what was edited.
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if enc, err := EncodePack(p); err != nil || string(enc) != string(raw) {
			t.Errorf("%s: EncodePack doesn't reproduce the file (%v); keep packs in its format", path, err)
		}
	}
}

func TestValidatePackRejects(t *testing.T) {
	good := Pack{
		ID: "x", Name: Text{"en": "X"}, Timezone: "UTC", Currency: "USD", Languages: []string{"en", "ja"},
		TravelModes: []string{"drive"},
		Areas:       []Area{{ID: "x-a", Name: Text{"en": "A"}, Lat: 1, Lng: 1}},
	}
	if err := ValidatePack(good, nil); err != nil {
		t.Fatalf("valid pack rejected: %v", err)
	}
	bounded := good
	bounded.Areas = []Area{{ID: "x-a", Name: Text{"en": "A"}, Lat: 1, Lng: 1, OSMID: 42,
		Boundary: []string{maps.EncodePolyline([]maps.LatLng{{Lat: 0.5, Lng: 0.5}, {Lat: 0.5, Lng: 1.5}, {Lat: 1.5, Lng: 1.5}, {Lat: 1.5, Lng: 0.5}})}}}
	if err := ValidatePack(bounded, nil); err != nil {
		t.Fatalf("valid pack with a boundary rejected: %v", err)
	}
	cases := map[string]func(p *Pack){
		"bad mode":      func(p *Pack) { p.TravelModes = []string{"teleport"} },
		"bad lang code": func(p *Pack) { p.Languages = []string{"ja", "Japanese"} },
		"translated override": func(p *Pack) {
			p.ActivityOverrides = []ActivityOverride{{ActivityID: "hike", Name: Text{"en": "Hike", "fr": "Rando"}}}
		},
		"foreign area name": func(p *Pack) { p.Areas[0].Name = Text{"en": "A", "fr": "A"} },
		"no english name":   func(p *Pack) { p.Areas[0].Name = Text{"ja": "エー"} },
		"no modes":          func(p *Pack) { p.TravelModes = nil },
		"area prefix":       func(p *Pack) { p.Areas[0].ID = "a" },
		"null island":       func(p *Pack) { p.Areas[0].Lat, p.Areas[0].Lng = 0, 0 },
		"bad month":         func(p *Pack) { p.ActivityOverrides = []ActivityOverride{{ActivityID: "hike", Months: []int16{13}}} },
		"unknown act":       func(p *Pack) { p.ActivityOverrides = []ActivityOverride{{ActivityID: "nope"}} },
		"duplicate area":    func(p *Pack) { p.Areas = append(p.Areas, p.Areas[0]) },
		"bad boundary":      func(p *Pack) { p.Areas[0].Boundary = []string{"_p~iF"} },
		"negative osm id":   func(p *Pack) { p.Areas[0].OSMID = -1 },
		"point outside its boundary": func(p *Pack) {
			p.Areas[0].Boundary = []string{maps.EncodePolyline([]maps.LatLng{{Lat: 2, Lng: 2}, {Lat: 2, Lng: 3}, {Lat: 3, Lng: 3}, {Lat: 3, Lng: 2}})}
		},
		"point inside another's boundary": func(p *Pack) {
			p.Areas = append(p.Areas, Area{ID: "x-b", Name: Text{"en": "B"}, Lat: 1.5, Lng: 1.5,
				Boundary: []string{maps.EncodePolyline([]maps.LatLng{{Lat: 0.5, Lng: 0.5}, {Lat: 0.5, Lng: 2}, {Lat: 2, Lng: 2}, {Lat: 2, Lng: 0.5}})}})
		},
	}
	for name, mutate := range cases {
		p := good
		p.Areas = append([]Area(nil), good.Areas...)
		mutate(&p)
		if err := ValidatePack(p, map[string]bool{"hike": true}); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

// TestCategoryLabels keeps the catalogue's categories and the app's labels in
// step: the onboarding grid shows "category.<id>" from the app's UI text
// (copied into internal/uistrings by make gen).
func TestCategoryLabels(t *testing.T) {
	var ui map[string]string
	if err := readJSON("../uistrings/en.json", &ui); err != nil {
		t.Fatal(err)
	}
	for _, c := range Categories {
		if ui["category."+c] == "" {
			t.Errorf("no app label category.%s", c)
		}
	}
	for k := range ui {
		if c, ok := strings.CutPrefix(k, "category."); ok && !slices.Contains(Categories, c) {
			t.Errorf("app label %s has no category", k)
		}
	}
}

func TestValidateCatalogReplaces(t *testing.T) {
	acts := []Activity{
		{ID: "hike", Name: Text{"en": "Hike"}, Category: "outdoors", Energy: 2, MinGroupSize: 2, MaxGroupSize: 8, Replaces: []string{"walk"}},
		{ID: "walk", Name: Text{"en": "Walk"}, Category: "outdoors", Energy: 1, MinGroupSize: 2, MaxGroupSize: 8},
		{ID: "run", Name: Text{"en": "Run"}, Category: "sports", Energy: 3, MinGroupSize: 2, MaxGroupSize: 8, Replaces: []string{"jog"}},
		{ID: "sprint", Name: Text{"en": "Sprint"}, Category: "sports", Energy: 3, MinGroupSize: 2, MaxGroupSize: 8, Replaces: []string{"jog"}},
	}
	err := ValidateCatalog(nil, acts)
	if err == nil || !strings.Contains(err.Error(), `"walk", which is still an activity`) || !strings.Contains(err.Error(), `"jog" is already replaced by run`) {
		t.Errorf("got %v", err)
	}
}

func TestWKT(t *testing.T) {
	if got := wkt(nil); got != "" {
		t.Errorf("no boundary: %q", got)
	}
	b := maps.Boundary{{{Lat: 16, Lng: 108}, {Lat: 16, Lng: 108.5}, {Lat: 16.5, Lng: 108}}, {{Lat: 1, Lng: 2}, {Lat: 1, Lng: 3}, {Lat: 2, Lng: 3}, {Lat: 1, Lng: 2}}}
	// Rings are closed (the second already is), longitude first.
	const want = "MULTIPOLYGON(((108 16,108.5 16,108 16.5,108 16)),((2 1,3 1,3 2,2 1)))"
	if got := wkt(b); got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
	if len(b[0]) != 3 {
		t.Error("closing a ring changed the caller's slice")
	}
}
