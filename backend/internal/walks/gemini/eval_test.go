package gemini

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/maps/google"
	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// TestEval composes real walks with Gemini, Places and Routes and logs each
// one with its places' names for review. It costs money (about $0.25 a
// walk: Text Search per place, one or two Routes calls, Place Details for
// the log), so it only runs when asked:
//
//	WALKS_EVAL=1 GOOGLE_GEMINI_API_KEY=… GOOGLE_MAPS_API_KEY=… go test ./internal/walks/gemini/ -run Eval -v -timeout 30m
//
// WALKS_EVAL_ONLY=photography@tokyo-suginami limits it to one case. The bar
// is that most walks compose; their quality is for a person to judge.
func TestEval(t *testing.T) {
	if os.Getenv("WALKS_EVAL") == "" {
		t.Skip("set WALKS_EVAL=1 to run against Gemini and Google Maps")
	}
	gemKey, mapsKey := os.Getenv("GOOGLE_GEMINI_API_KEY"), os.Getenv("GOOGLE_MAPS_API_KEY")
	if gemKey == "" || mapsKey == "" {
		t.Fatal("GOOGLE_GEMINI_API_KEY and GOOGLE_MAPS_API_KEY are required")
	}
	acts, err := cities.ReadActivities("../../../../data/activities.json")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]cities.Activity{}
	for _, a := range acts {
		byID[a.ID] = a
	}
	areas := map[string]struct {
		city  string
		area  cities.Area
		names map[string]string // the pack's activity renames, as the app shows them
	}{}
	for _, f := range []string{"tokyo", "austin", "da-nang-hoi-an"} {
		p, err := cities.ReadPack("../../../../data/citypacks/" + f + ".json")
		if err != nil {
			t.Fatal(err)
		}
		names := map[string]string{}
		for _, o := range p.ActivityOverrides {
			if o.Name["en"] != "" {
				names[o.ActivityID] = o.Name["en"]
			}
		}
		for _, a := range p.Areas {
			areas[a.ID] = struct {
				city  string
				area  cities.Area
				names map[string]string
			}{p.Name["en"], a, names}
		}
	}
	// Every walking theme, in areas known for it and in ones that aren't.
	cases := []string{
		"temple-shrine@tokyo-suginami", "temple-shrine@tokyo-shibuya",
		"photography@tokyo-meguro", "photography@tokyo-setagaya",
		"city-walk@tokyo-musashino", "food-tour@tokyo-musashino",
		"cafe-hop@tokyo-setagaya", "seasonal-viewing@tokyo-meguro",
		"city-walk@austin-south-congress", "food-tour@austin-east",
	}
	if only := os.Getenv("WALKS_EVAL_ONLY"); only != "" {
		cases = strings.Split(only, ",")
	}

	g := google.New(mapsKey)
	c := &walks.Composer{Proposer: New(gemKey), Places: g, Routes: g, Rules: walks.DefaultRules()}
	type result struct {
		walk walks.Walk
		err  error
		took time.Duration
	}
	results := make([]result, len(cases))
	var wg sync.WaitGroup
	for i, cs := range cases {
		actID, areaID, _ := strings.Cut(cs, "@")
		a, ok := areas[areaID]
		act, ok2 := byID[actID]
		if !ok || !ok2 || act.WalkTheme == "" {
			t.Fatalf("%s: unknown area or not a walking activity", cs)
		}
		name := act.Name["en"]
		if n, ok := a.names[actID]; ok {
			name = n
		}
		req := walks.Request{
			City: a.city, Area: a.area.Name["en"], Center: maps.LatLng{Lat: a.area.Lat, Lng: a.area.Lng},
			Activity: name, Theme: act.WalkTheme, Month: time.November, Blocks: act.Blocks,
		}
		wg.Go(func() {
			start := time.Now()
			w, err := c.Compose(context.Background(), req)
			results[i] = result{w, err, time.Since(start)}
		})
	}
	wg.Wait()

	composed := 0
	for i, cs := range cases {
		r := results[i]
		var rejected *walks.Rejected
		switch {
		case errors.As(r.err, &rejected):
			t.Logf("REJECTED %s (%s): %s", cs, r.took.Round(time.Second), strings.Join(rejected.Reasons, "; "))
			continue
		case r.err != nil:
			t.Errorf("%s: %v", cs, r.err)
			continue
		}
		composed++
		var b strings.Builder
		fmt.Fprintf(&b, "%s (%s): %q, %d min, %.1f km walking\n  %s\n", cs, r.took.Round(time.Second), r.walk.Title,
			r.walk.Minutes(), float64(r.walk.Route.Meters)/1000, r.walk.Summary)
		for _, s := range r.walk.Stops {
			d, err := g.Details(context.Background(), s.PlaceID, "en")
			name := d.Name
			if err != nil {
				name = "(" + err.Error() + ")"
			}
			fmt.Fprintf(&b, "  %-6s +%2.0f min walk, %2d min at %s, %s\n         %s\n", s.Role, s.Walk.Minutes(), s.Minutes, name, d.Address, s.Note)
		}
		t.Log(b.String())
	}
	t.Logf("composed %d of %d", composed, len(cases))
	if composed*10 < len(cases)*7 {
		t.Errorf("only %d of %d walks composed (want 70%%)", composed, len(cases))
	}
}
