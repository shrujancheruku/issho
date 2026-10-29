package sim

import (
	"path/filepath"
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

// TestSimulate runs the whole flow on 500 synthetic users in each city pack
// and checks the invariants: every hard rule holds, nobody is placed twice
// or invited when not free, and most people who sign up get invited.
func TestSimulate(t *testing.T) {
	cfg := matching.DefaultConfig()
	cfg.LanguagePairs = [][2]string{{"ja", "en"}}
	for _, city := range []string{"tokyo", "austin", "da-nang-hoi-an"} {
		t.Run(city, func(t *testing.T) {
			pack, acts := load(t, city)
			st := Run(pack, acts, Options{Users: 500, Seed: 1, Month: time.May}, cfg)
			t.Logf("%s\n%s", city, st)
			if len(st.Violations) > 0 {
				t.Errorf("%d rule violations, e.g. %s", len(st.Violations), st.Violations[0])
			}
			if rate := float64(st.Invited) / float64(st.SignedUp); rate < 0.5 {
				t.Errorf("only %.0f%% of people who signed up were invited", 100*rate)
			}
		})
	}
}

func TestDeterministic(t *testing.T) {
	pack, acts := load(t, "tokyo")
	cfg := matching.DefaultConfig()
	opt := Options{Users: 150, Seed: 7}
	a, b := Run(pack, acts, opt, cfg), Run(pack, acts, opt, cfg)
	a.Duration, b.Duration = 0, 0
	if a.String() != b.String() {
		t.Fatalf("runs differ:\n%s\n%s", a, b)
	}
}
