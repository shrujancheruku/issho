package sim

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

const dataDir = "../../../../data"

func load(t *testing.T, city string) (cities.Pack, []cities.Activity) {
	t.Helper()
	pack, err := cities.ReadPack(filepath.Join(dataDir, "citypacks", city+".json"))
	if err != nil {
		t.Fatal(err)
	}
	acts, err := cities.ReadActivities(filepath.Join(dataDir, "activities.json"))
	if err != nil {
		t.Fatal(err)
	}
	return pack, acts
}

// TestSimulate runs the engine on 500 synthetic users in each city pack and
// checks the invariants: every hard rule holds, nobody is placed twice, and
// most people get a proposal.
func TestSimulate(t *testing.T) {
	if testing.Short() {
		t.Skip("simulation is slow-ish")
	}
	cfg := matching.DefaultConfig()
	cfg.LanguagePairs = [][2]string{{"ja", "en"}}
	for _, city := range []string{"tokyo", "austin"} {
		t.Run(city, func(t *testing.T) {
			pack, acts := load(t, city)
			st := Run(pack, acts, Options{Users: 500, Seed: 1, Month: time.May}, cfg)
			t.Logf("%s\n%s", city, st)
			if len(st.Violations) > 0 {
				t.Errorf("%d rule violations, e.g. %s", len(st.Violations), st.Violations[0])
			}
			if rate := float64(st.Invited) / float64(st.Users); rate < 0.6 {
				t.Errorf("only %.0f%% of users got a proposal", 100*rate)
			}
		})
	}
}

func TestDeterministic(t *testing.T) {
	pack, acts := load(t, "tokyo")
	cfg := matching.DefaultConfig()
	opt := Options{Users: 150, Seed: 7}
	first := matching.Run(Generate(pack, acts, opt), cfg)
	again := matching.Run(Generate(pack, acts, opt), cfg)
	if len(first) != len(again) {
		t.Fatalf("%d vs %d proposals", len(first), len(again))
	}
	for i := range first {
		a, b := first[i], again[i]
		if a.Plan != b.Plan || a.Area != b.Area || a.Slot != b.Slot || a.Quorum != b.Quorum || !slices.Equal(a.Invited, b.Invited) {
			t.Fatalf("proposal %d differs: %+v vs %+v", i, a, b)
		}
	}
}
