package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/tagging"
)

// TestEval runs the tagging eval set (testdata/eval.json) against the real
// Claude API. It costs money (about $0.01 for the whole set), so it only runs
// when asked:
//
//	TAGGING_EVAL=1 go test ./internal/tagging/claude/ -run Eval -v
//
// Credentials: ANTHROPIC_API_KEY, or an `ant auth login` profile.
//
// Each case lists the activities its text should produce (optionally with the
// interest level), activities it must not produce, or that it should produce
// none (unrelated text, prompt injection). Extra matches are logged, not
// failed: broad matches are often right.
func TestEval(t *testing.T) {
	if os.Getenv("TAGGING_EVAL") == "" {
		t.Skip("set TAGGING_EVAL=1 to run against the Claude API")
	}
	acts, err := cities.ReadActivities("../../../../data/activities.json")
	if err != nil {
		t.Fatal(err)
	}
	catalog := make([]tagging.Activity, len(acts))
	for i, a := range acts {
		catalog[i] = tagging.Activity{ID: a.ID, Name: a.Name["en"]}
	}
	raw, err := os.ReadFile("testdata/eval.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Lang    string            `json:"lang"`
		Text    string            `json:"text"`
		Expect  map[string]string `json:"expect"`
		MustNot []string          `json:"must_not"`
		None    bool              `json:"none"`
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}

	tagger := New(os.Getenv("ANTHROPIC_API_KEY"))
	level := map[int]string{0: "open_to_try", 1: "like", 2: "love"}
	var expected, found, levelsChecked, levelsRight, mustNotHits, noneCases, noneRight int
	for _, c := range cases {
		tags, err := tagger.Tag(context.Background(), c.Text, catalog)
		if err != nil {
			t.Errorf("[%s] %q: %v", c.Lang, c.Text, err)
			continue
		}
		got := map[string]string{}
		var names []string
		for _, tg := range tags {
			got[tg.ActivityID] = level[tg.Weight]
			names = append(names, tg.ActivityID+"="+level[tg.Weight])
		}
		var problems []string
		for id, want := range c.Expect {
			expected++
			lv, ok := got[id]
			if !ok {
				problems = append(problems, "missing "+id)
				continue
			}
			found++
			if want != "" {
				levelsChecked++
				if lv == want {
					levelsRight++
				} else {
					problems = append(problems, fmt.Sprintf("%s is %s, want %s", id, lv, want))
				}
			}
		}
		for _, id := range c.MustNot {
			if _, ok := got[id]; ok {
				mustNotHits++
				problems = append(problems, "unwanted "+id)
			}
		}
		if c.None {
			noneCases++
			if len(tags) == 0 {
				noneRight++
			} else {
				problems = append(problems, "expected no tags")
			}
		}
		var extra []string
		for id := range got {
			if _, ok := c.Expect[id]; !ok {
				extra = append(extra, id)
			}
		}
		status := "ok  "
		if len(problems) > 0 {
			status = "MISS"
		}
		t.Logf("%s [%s] %s\n       got: %s%s", status, c.Lang, c.Text, strings.Join(names, ", "),
			map[bool]string{true: "\n       " + strings.Join(problems, "; "), false: ""}[len(problems) > 0])
		if len(extra) > 0 && !c.None {
			t.Logf("       extra (review): %s", strings.Join(extra, ", "))
		}
	}

	recall := float64(found) / float64(max(expected, 1))
	levels := float64(levelsRight) / float64(max(levelsChecked, 1))
	t.Logf("recall %.0f%% (%d/%d) · levels %.0f%% (%d/%d) · must-not hits %d · empty cases %d/%d",
		recall*100, found, expected, levels*100, levelsRight, levelsChecked, mustNotHits, noneRight, noneCases)
	// Bars: the person confirms every suggestion, so a miss costs a tap,
	// while an unwanted or invented tag costs trust.
	if recall < 0.9 {
		t.Errorf("recall %.0f%% below 90%%", recall*100)
	}
	if levels < 0.8 {
		t.Errorf("level accuracy %.0f%% below 80%%", levels*100)
	}
	if mustNotHits > 0 || noneRight < noneCases {
		t.Errorf("%d must-not hits, %d of %d empty cases wrong", mustNotHits, noneCases-noneRight, noneCases)
	}
}
