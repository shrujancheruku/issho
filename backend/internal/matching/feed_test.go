package matching

import (
	"fmt"
	"slices"
	"testing"
)

func TestFeed(t *testing.T) {
	cfg := DefaultConfig()
	acts := map[ActivityID]*Activity{}
	for _, id := range []string{"walk", "chess", "karaoke", "pottery", "bar"} {
		a := act(id, 3, 8)
		acts[a.ID] = &a
	}
	acts["bar"].Blocks = []Block{Evening}
	u := &User{
		HomeArea: "a", MaxTravelMin: 20, Languages: fluent("ja"), Availability: []Slot{sunPM}, Budget: 3,
		Prefs: map[ActivityID]int{"walk": Love, "chess": Like, "karaoke": Like, "pottery": OpenToTry, "bar": Love},
	}
	plans := []OpenPlan{
		{Plan: Plan{Area: "a", Activity: "pottery", Primary: "ja"}},
		{Plan: Plan{Area: "c", Activity: "walk", Primary: "ja"}}, // 40 min: too far
		{Plan: Plan{Area: "a", Activity: "walk", Primary: "es"}}, // can't speak it
		{Plan: Plan{Area: "a", Activity: "chess", Primary: "ja"}, Signups: 8},
		{Plan: Plan{Area: "b", Activity: "karaoke", Primary: "ja"}},
		{Plan: Plan{Area: "a", Activity: "walk", Primary: "es", Secondary: "ja"}}, // speaks one of them: fine
		{Plan: Plan{Area: "a", Activity: "bar", Primary: "ja"}},                   // only in the evening, when they're not usually free
	}
	var got []string
	for _, it := range Feed(u, plans, acts, testTravel, cfg, nil) {
		got = append(got, fmt.Sprintf("%s %s@%s/%s", it.Level, it.Activity, it.Area, it.Primary))
	}
	want := []string{
		"top walk@a/es",     // love
		"top bar@a/ja",      // love, but halved for the time of day
		"good chess@a/ja",   // like, with full momentum
		"good karaoke@b/ja", // like, 15 minutes away
		"maybe pottery@a/ja",
		"low walk@c/ja",
		"not-for-you walk@a/es",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got\n  %q\nwant\n  %q", got, want)
	}
}

// A plan friends signed up for moves up a level and ahead within it; one
// in a language they don't speak stays out of reach.
func TestFeedFriends(t *testing.T) {
	cfg := DefaultConfig()
	acts := map[ActivityID]*Activity{}
	for _, id := range []string{"walk", "chess", "pottery"} {
		a := act(id, 3, 8)
		acts[a.ID] = &a
	}
	u := &User{
		HomeArea: "a", MaxTravelMin: 20, Languages: fluent("ja"), Availability: []Slot{sunPM}, Budget: 3,
		Prefs: map[ActivityID]int{"walk": Love, "chess": Like, "pottery": OpenToTry},
	}
	plans := []OpenPlan{
		{Plan: Plan{Area: "a", Activity: "walk", Primary: "ja"}},
		{Plan: Plan{Area: "a", Activity: "chess", Primary: "ja"}},
		{Plan: Plan{Area: "a", Activity: "pottery", Primary: "ja"}},
		{Plan: Plan{Area: "a", Activity: "walk", Primary: "es"}},
	}
	friends := []Friends{{}, {}, {Count: 2, Strength: 1.5}, {Count: 1, Strength: 1}}
	var got []string
	for _, it := range Feed(u, plans, acts, testTravel, cfg, friends) {
		got = append(got, fmt.Sprintf("%s %s/%s friends=%d", it.Level, it.Activity, it.Primary, it.Friends))
	}
	want := []string{
		"top walk/ja friends=0",
		"good pottery/ja friends=2", // open to try, lifted to good and ahead of chess
		"good chess/ja friends=0",
		"not-for-you walk/es friends=0",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got\n  %q\nwant\n  %q", got, want)
	}
}
