package bench

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/maps"
	"github.com/shrujancheruku/issho/backend/internal/walks"
)

// TestCases: every case loads against the real catalogue and city packs (a
// walking activity, enabled and in season, in a real area), ids are unique,
// and every case has its three tags.
func TestCases(t *testing.T) {
	cs, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	reqs, err := Requests(cs, "../../../../data", "")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	themes := map[string]int{}
	for _, c := range cs {
		if seen[c.ID] || len(c.Tags) != 3 || c.Tags[0] != c.Activity || c.Tags[1] != c.City {
			t.Errorf("case %+v: duplicate id or bad tags", c)
		}
		seen[c.ID] = true
		themes[c.Activity]++
		if r := reqs[c.ID]; r.Theme == "" || r.Area == "" || r.Center.Lat == 0 {
			t.Errorf("case %s: request %+v", c.ID, r)
		}
	}
	if len(cs) < 30 || len(themes) != 6 {
		t.Errorf("%d cases over %d themes; want at least 30 over all 6", len(cs), len(themes))
	}
	// The city's own name for an activity is used.
	if r := reqs["seasonal-viewing@austin-zilker"]; r.Activity != "Wildflowers and autumn leaves" {
		t.Errorf("Austin seasonal activity = %q", r.Activity)
	}
}

// fakeProposer proposes a fixed walk ("good") or nothing usable ("null"),
// reporting the model as asked, or another one ("rerouted").
type fakeProposer struct{ kind, model string }

func (f fakeProposer) Propose(_ context.Context, req walks.Request, _ []string) (walks.Proposal, error) {
	pl := func(name, kind string, min int) walks.Place {
		return walks.Place{Name: name, Search: name, Kind: kind, Minutes: min, Note: name + " in " + req.Area + "."}
	}
	meta := walks.Meta{Model: f.model, InputTokens: 1000, OutputTokens: 4000, Latency: 2 * time.Second}
	switch f.kind {
	case "null":
		return walks.Proposal{Meta: meta}, nil
	case "rerouted":
		meta.Model = "claude-other"
	case "broken":
		return walks.Proposal{}, errors.New("HTTP 500")
	}
	return walks.Proposal{
		Title: f.model + " walk", Summary: "A walk.", Meta: meta,
		Meet:   pl("Station", "station", 10),
		Stops:  []walks.Place{pl("Temple A", "temple", 30), pl("Temple B", "temple", 30), pl("Temple C", "temple", 30), pl("Nowhere", "temple", 30)},
		Finish: pl("Cafe", "cafe", 45),
	}, nil
}

// findAll finds every place but "Nowhere", near the centre, with a type
// fitting its kind.
type findAll struct{}

func (findAll) Find(_ context.Context, r maps.FindRequest) (maps.Found, bool, error) {
	name, _, _ := strings.Cut(r.Query, ",")
	if name == "Nowhere" {
		return maps.Found{}, false, nil
	}
	typ := map[string]string{"Station": "train_station", "Cafe": "cafe"}[name]
	return maps.Found{Place: maps.Place{ID: "id-" + name, PrimaryType: cmpOr(typ, "buddhist_temple"), Location: r.Near}, Name: name}, true, nil
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

type walkAll struct{}

func (walkAll) WalkThrough(_ context.Context, places []string, _ bool) (maps.WalkRoute, error) {
	w := maps.WalkRoute{Route: maps.Route{Polyline: "x", Meters: 3000}}
	for i := range places {
		w.Order = append(w.Order, i)
		if i > 0 {
			w.Legs = append(w.Legs, maps.Route{Duration: 8 * time.Minute, Meters: 600})
			w.Duration += 8 * time.Minute
		}
	}
	return w, nil
}

// judgeFake prefers the walk whose title starts with "claude-opus".
type judgeFake struct{ name string }

func (j judgeFake) Name() string { return j.name }

func (j judgeFake) JSON(_ context.Context, system, user string, schema map[string]any) (string, Call, error) {
	if !strings.Contains(system, "strangers") || strings.Contains(user, "id-Temple") {
		return "", Call{}, errors.New("judge must get the rubric and no place ids")
	}
	a, _, _ := strings.Cut(strings.SplitN(user, "<plan_a>", 2)[1], "</plan_a>")
	w := "B"
	if strings.Contains(a, "Title: claude-opus") {
		w = "A"
	}
	v := map[string]string{}
	for _, c := range Criteria {
		v[c] = w
	}
	b, _ := json.Marshal(map[string]any{"verdicts": v, "dubious_a": []string{}, "dubious_b": []string{"x"}, "reason": "r"})
	return string(b), Call{Model: j.name, InputTokens: 10, OutputTokens: 10}, nil
}

// TestRunJudgeReport runs the whole pipeline on fakes: an oracle variant
// composes every case, a null one none, a rerouted one is an error (the
// model that answered isn't the one asked for), a broken one is an error;
// a rerun resumes; judges compare pairs blind in both orders; the review
// sheet and the report come out.
func TestRunJudgeReport(t *testing.T) {
	cs, err := Cases()
	if err != nil {
		t.Fatal(err)
	}
	cs = cs[:3]
	reqs, err := Requests(cs, "../../../../data", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var variants []Variant
	for _, s := range []string{"claude:claude-opus-5-5:high", "claude:claude-sonnet-5-5:high", "gemini:null", "claude:rerouted", "gemini:broken"} {
		v, err := ParseVariant(s)
		if err != nil {
			t.Fatal(err)
		}
		variants = append(variants, v)
	}
	cfg := Config{
		Cases: cs, Requests: reqs, Variants: variants, Reps: 1, Out: dir, Prompt: walks.PromptVersion,
		NewProposer: func(v Variant) (walks.Proposer, error) {
			switch v.Model {
			case "null", "rerouted", "broken":
				return fakeProposer{kind: v.Model, model: v.Model}, nil
			}
			return fakeProposer{kind: "good", model: v.Model}, nil
		},
		Places: findAll{}, Routes: walkAll{}, Rules: walks.DefaultRules(), Concurrency: 4,
	}
	ctx := context.Background()
	n, err := Run(ctx, cfg)
	if err == nil || n != 9 {
		t.Fatalf("ran %d (%v); want 9 verdicts and an error for the failed walks", n, err)
	}
	results, _ := ReadResults(filepath.Join(dir, "results.jsonl"))
	errRows, _ := readLines[ErrorRow](filepath.Join(dir, "errors.jsonl"))
	classes := map[string]int{}
	for _, e := range errRows {
		classes[e.Class]++
	}
	if len(results) != 9 || classes["model_mismatch"] != 3 || classes["api"] != 3 {
		t.Fatalf("%d results, errors %v", len(results), classes)
	}
	for _, r := range results {
		switch r.Variant {
		case "gemini:null":
			if r.Status != "rejected" || r.Attempts != 2 {
				t.Errorf("null variant: %+v", r)
			}
		default:
			// "Nowhere" is dropped; 10 + 90 + 45 at the stops + 4 walks of 8.
			if r.Status != "composed" || r.Drops[walks.DropNotFound] != 1 || r.Proposed != 6 || r.Walk.Minutes != 177 ||
				len(r.Walk.Stops) != 5 || r.Walk.Stops[1].Name != "Temple A" || r.Walk.Stops[1].PlaceID != "id-Temple A" ||
				r.PlaceLookups != 6 || r.RouteCalls != 1 || r.ModelUSD <= 0 {
				t.Errorf("good variant: %+v", r)
			}
		}
	}
	// A rerun only retries what failed.
	if n, _ := Run(ctx, cfg); n != 0 {
		t.Errorf("rerun ran %d new verdicts", n)
	}

	settings := map[string]string{}
	for id, r := range reqs {
		settings[id] = walks.UserPrompt(r, nil)
	}
	judged, err := Judge(ctx, JudgeConfig{Dir: dir, Cases: cs, Settings: settings, Judges: []LLM{judgeFake{"j1"}, judgeFake{"j2"}}, Concurrency: 4})
	// 3 cases × 1 pair (opus, sonnet) × 2 judges × 2 orders.
	if err != nil || judged != 12 {
		t.Fatalf("judged %d, %v", judged, err)
	}
	if err := Review(dir, settings, 0, 1, ""); err != nil {
		t.Fatal(err)
	}
	page, _ := os.ReadFile(filepath.Join(dir, "review.html"))
	if strings.Contains(string(page), "claude-opus-5-5:high") || !strings.Contains(string(page), "Temple A") {
		t.Error("the review sheet must be blind and show the walks")
	}
	// Each plan links to its walking route through every stop, in order.
	if !strings.Contains(string(page), "https://www.google.com/maps/dir/?api=1&amp;destination=Cafe&amp;destination_place_id=id-Cafe&amp;origin=Station&amp;origin_place_id=id-Station&amp;travelmode=walking&amp;waypoint_place_ids=id-Temple&#43;A%7Cid-Temple&#43;B%7Cid-Temple&#43;C&amp;waypoints=Temple&#43;A%7CTemple&#43;B%7CTemple&#43;C") ||
		strings.Contains(string(page), "maps/embed") {
		t.Errorf("route links missing or a map embedded without a key")
	}
	// A second sheet keeps the pairs, sides and ids, and embeds with a key.
	before, _ := os.ReadFile(filepath.Join(dir, "review_key.json"))
	if err := Review(dir, settings, 0, 99, "embed-key"); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(dir, "review_key.json"))
	page, _ = os.ReadFile(filepath.Join(dir, "review.html"))
	if string(before) != string(after) || !strings.Contains(string(page), "maps/embed/v1/directions?destination=place_id%3Aid-Cafe&amp;key=embed-key&amp;mode=walking") {
		t.Error("regenerating changed the pairs, or the map isn't embedded")
	}
	// Answer every pair for opus.
	var key []ReviewPair
	raw, _ := os.ReadFile(filepath.Join(dir, "review_key.json"))
	_ = json.Unmarshal(raw, &key)
	answers := map[string]map[string]any{}
	for _, p := range key {
		pick := "B"
		if p.A == "claude:claude-opus-5-5:high" {
			pick = "A"
		}
		answers[p.ID] = map[string]any{"overall": pick}
	}
	ab, _ := json.Marshal(answers)
	ansPath := filepath.Join(dir, "answers.json")
	_ = os.WriteFile(ansPath, ab, 0o644)

	report, err := Summarize(dir, ansPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| claude:claude-opus-5-5:high | 3/3 100%",
		"| gemini:null | 0/3 0%",
		"| claude:claude-opus-5-5:high | 100% (100–100) | 100% (100–100)",
		"| claude:claude-sonnet-5-5:high | 0% (0–0)",
		"| j1 | 3 | 100% |",
		"| claude:claude-opus-5-5:high | 100% (3) | 100% (3) |", // by judge
		"Judges agree on the overall verdict for 100% of 3 pairs",
		"| claude:claude-opus-5-5:high | 100% (3) |",
		"| j1 | 100% of 3 |",
		"| claude:claude-sonnet-5-5:high | 0% (0–0) | 0% (0–0) | 0% (0–0) | 0% (0–0) | 0% (0–0) | 0% (0–0) | 0.50 |", // the fake doubts whichever walk is plan B
		"| claude:rerouted | 0/0", // every walk failed: still listed, with its errors
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
}
