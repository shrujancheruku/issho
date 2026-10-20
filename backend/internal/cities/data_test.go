package cities

import (
	"path/filepath"
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
		"partial override": func(p *Pack) {
			p.ActivityOverrides = []ActivityOverride{{ActivityID: "hike", Name: Text{"en": "Hike"}}}
		},
		"no modes":       func(p *Pack) { p.TravelModes = nil },
		"area prefix":    func(p *Pack) { p.Areas[0].ID = "a" },
		"null island":    func(p *Pack) { p.Areas[0].Lat, p.Areas[0].Lng = 0, 0 },
		"bad month":      func(p *Pack) { p.ActivityOverrides = []ActivityOverride{{ActivityID: "hike", Months: []int16{13}}} },
		"unknown act":    func(p *Pack) { p.ActivityOverrides = []ActivityOverride{{ActivityID: "nope"}} },
		"duplicate area": func(p *Pack) { p.Areas = append(p.Areas, p.Areas[0]) },
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
