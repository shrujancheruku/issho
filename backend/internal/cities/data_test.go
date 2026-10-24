package cities

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
