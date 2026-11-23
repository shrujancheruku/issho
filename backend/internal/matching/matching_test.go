package matching_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

var (
	monMorning = matching.Slot{Date: "2026-12-07", Part: matching.Morning}
	monEvening = matching.Slot{Date: "2026-12-07", Part: matching.Evening}
	tueMorning = matching.Slot{Date: "2026-12-08", Part: matching.Morning}
)

// crowd is n people who all tick one slot and all speak English.
func crowd(n int, slot matching.Slot) []matching.Signup {
	var out []matching.Signup
	for i := range n {
		out = append(out, matching.Signup{
			Person:    matching.PersonID(fmt.Sprintf("p%02d", i)),
			Ticks:     []matching.Slot{slot},
			Languages: []string{"en"},
			Gender:    matching.Man,
		})
	}
	return out
}

func sizes(groups []matching.Group) []int {
	var out []int
	for _, g := range groups {
		out = append(out, len(g.People))
	}
	slices.Sort(out)
	slices.Reverse(out)
	return out
}

// The spec's own sizes (docs/spec/matching.md, "Sizes"): the most people
// seated, then sizes nearest 4 or 5, then fewer groups.
func TestNamedSizes(t *testing.T) {
	for _, tc := range []struct {
		people int
		want   []int
	}{
		{0, nil},
		{2, nil},
		{3, []int{3}},
		{4, []int{4}},
		{5, []int{5}},
		{6, []int{6}},
		{7, []int{4, 3}},
		{8, []int{4, 4}},
		{9, []int{5, 4}},
		{10, []int{5, 5}},
		{11, []int{6, 5}},
		{12, []int{4, 4, 4}},
	} {
		t.Run(fmt.Sprint(tc.people), func(t *testing.T) {
			got := sizes(matching.Match(matching.Input{Signups: crowd(tc.people, monMorning)}))
			if !slices.Equal(got, tc.want) {
				t.Errorf("%d people made groups of %v, want %v", tc.people, got, tc.want)
			}
		})
	}
}

func ids(g matching.Group) []string {
	var out []string
	for _, p := range g.People {
		out = append(out, string(p))
	}
	return out
}

func TestGroupCarriesItsDatePartAndSharedLanguages(t *testing.T) {
	signups := []matching.Signup{
		{Person: "a", Ticks: []matching.Slot{tueMorning}, Languages: []string{"vi", "en", "ja"}},
		{Person: "b", Ticks: []matching.Slot{tueMorning}, Languages: []string{"en", "vi"}},
		{Person: "c", Ticks: []matching.Slot{tueMorning}, Languages: []string{"vi", "en", "fr"}},
	}
	got := matching.Match(matching.Input{Signups: signups})
	want := []matching.Group{{
		Date: "2026-12-08", Part: matching.Morning,
		People: []matching.PersonID{"a", "b", "c"}, Languages: []string{"en", "vi"},
	}}
	if !slices.EqualFunc(got, want, func(a, b matching.Group) bool {
		return a.Date == b.Date && a.Part == b.Part && slices.Equal(a.People, b.People) && slices.Equal(a.Languages, b.Languages)
	}) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestOnlyPeopleWhoTickedTheTimeAreGrouped(t *testing.T) {
	signups := crowd(3, monMorning)
	signups = append(signups, matching.Signup{Person: "evening-only", Ticks: []matching.Slot{monEvening}, Languages: []string{"en"}})
	got := matching.Match(matching.Input{Signups: signups})
	if len(got) != 1 || slices.Contains(got[0].People, "evening-only") {
		t.Errorf("got %+v, want one group without evening-only", got)
	}
}

func TestAGroupNeedsOneSharedLanguage(t *testing.T) {
	// Two English speakers and two Vietnamese speakers share nothing; the
	// bilingual person joins either side, but never makes a two-language group.
	mk := func(id string, langs ...string) matching.Signup {
		return matching.Signup{Person: matching.PersonID(id), Ticks: []matching.Slot{monMorning}, Languages: langs}
	}
	t.Run("no shared language, no group", func(t *testing.T) {
		got := matching.Match(matching.Input{Signups: []matching.Signup{mk("a", "en"), mk("b", "vi"), mk("c", "ja")}})
		if len(got) != 0 {
			t.Errorf("got %+v, want none", got)
		}
	})
	t.Run("a bilingual person bridges nothing", func(t *testing.T) {
		got := matching.Match(matching.Input{Signups: []matching.Signup{
			mk("a", "en"), mk("b", "en"), mk("c", "vi"), mk("d", "vi"), mk("e", "en", "vi"),
		}})
		for _, g := range got {
			if len(g.Languages) == 0 {
				t.Errorf("group %v has no shared language", ids(g))
			}
		}
		if seated := len(got[0].People); len(got) != 1 || seated != 3 {
			t.Errorf("got %+v, want one group of 3", got)
		}
	})
}

func TestKeptApartPairsAreNeverGrouped(t *testing.T) {
	// Four people, one pair kept apart: three of them make the group.
	in := matching.Input{
		Signups:   crowd(4, monMorning),
		KeptApart: []matching.Pair{{A: "p03", B: "p01"}}, // either order
	}
	got := matching.Match(in)
	if len(got) != 1 || len(got[0].People) != 3 {
		t.Fatalf("got %+v, want one group of 3", got)
	}
	if slices.Contains(got[0].People, "p01") && slices.Contains(got[0].People, "p03") {
		t.Errorf("group %v holds a kept-apart pair", ids(got[0]))
	}
}

func TestNoGroupOnAClosedDate(t *testing.T) {
	signups := crowd(3, monMorning)
	for i := range signups {
		signups[i].Ticks = []matching.Slot{monMorning, tueMorning}
	}
	got := matching.Match(matching.Input{Signups: signups, ClosedDates: []string{monMorning.Date}})
	if len(got) != 1 || got[0].Date != tueMorning.Date {
		t.Errorf("got %+v, want one group on %s", got, tueMorning.Date)
	}
	got = matching.Match(matching.Input{Signups: signups, ClosedDates: []string{monMorning.Date, tueMorning.Date}})
	if len(got) != 0 {
		t.Errorf("got %+v, want none when every ticked date is closed", got)
	}
}

func TestOnlyWomenAndNonBinarySetting(t *testing.T) {
	mk := func(id string, g matching.Gender, only bool) matching.Signup {
		return matching.Signup{Person: matching.PersonID(id), Ticks: []matching.Slot{monMorning}, Languages: []string{"en"}, Gender: g, WomenAndNonBinaryOnly: only}
	}
	t.Run("a man can't join, so the others group without him", func(t *testing.T) {
		got := matching.Match(matching.Input{Signups: []matching.Signup{
			mk("a", matching.Woman, true), mk("b", matching.NonBinary, false), mk("c", matching.Woman, false), mk("d", matching.Man, false),
		}})
		if len(got) != 1 || slices.Contains(got[0].People, "d") || len(got[0].People) != 3 {
			t.Errorf("got %+v, want a, b, c only", got)
		}
	})
	t.Run("others don't need the setting on", func(t *testing.T) {
		got := matching.Match(matching.Input{Signups: []matching.Signup{
			mk("a", matching.Woman, false), mk("b", matching.NonBinary, false), mk("c", matching.Woman, true),
		}})
		if len(got) != 1 {
			t.Errorf("got %+v, want one group", got)
		}
	})
	t.Run("someone who declined to say, or a man, with the setting on is never grouped", func(t *testing.T) {
		got := matching.Match(matching.Input{Signups: []matching.Signup{
			mk("a", matching.Undeclared, true), mk("b", matching.Man, true), mk("c", matching.Woman, false),
			mk("d", matching.Woman, false), mk("e", matching.Woman, false),
		}})
		if len(got) != 1 || slices.Contains(got[0].People, "a") || slices.Contains(got[0].People, "b") {
			t.Errorf("got %+v, want c, d, e only", got)
		}
	})
}

func TestTheDateThatSeatsTheMostWins(t *testing.T) {
	// Seven tick Monday morning, but only five of them can do Tuesday morning:
	// Monday seats more.
	signups := crowd(7, monMorning)
	for i := range signups[:5] {
		signups[i].Ticks = []matching.Slot{monMorning, tueMorning}
	}
	got := matching.Match(matching.Input{Signups: signups})
	if len(got) == 0 || got[0].Date != monMorning.Date || sizes(got)[0] != 4 {
		t.Errorf("got %+v, want Monday's groups of 4 and 3", got)
	}
}

func TestATieGoesToTheEarliest(t *testing.T) {
	for _, tc := range []struct {
		name  string
		ticks []matching.Slot
		want  matching.Slot
	}{
		{"earlier date", []matching.Slot{tueMorning, monEvening}, monEvening},
		{"earlier part of the same day", []matching.Slot{monEvening, monMorning}, monMorning},
	} {
		t.Run(tc.name, func(t *testing.T) {
			signups := crowd(4, monMorning)
			for i := range signups {
				signups[i].Ticks = tc.ticks
			}
			got := matching.Match(matching.Input{Signups: signups})
			if len(got) != 1 || got[0].Date != tc.want.Date || got[0].Part != tc.want.Part {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLeftoversAreGroupedAtAnotherTime(t *testing.T) {
	// Three tick only Monday morning and three only Tuesday morning: two groups.
	signups := append(crowd(3, monMorning), crowd(6, tueMorning)[3:]...)
	got := matching.Match(matching.Input{Signups: signups})
	if len(got) != 2 || got[0].Date != monMorning.Date || got[1].Date != tueMorning.Date {
		t.Errorf("got %+v, want a group on each day", got)
	}
}

func TestAPersonAppearingTwiceCountsOnce(t *testing.T) {
	signups := crowd(3, monMorning)
	signups = append(signups, signups[0])
	got := matching.Match(matching.Input{Signups: signups})
	if len(got) != 1 || len(got[0].People) != 3 {
		t.Errorf("got %+v, want one group of 3", got)
	}
}

func TestTicksOfAnUnknownPartAreIgnored(t *testing.T) {
	signups := crowd(3, matching.Slot{Date: monMorning.Date, Part: "midnight"})
	if got := matching.Match(matching.Input{Signups: signups}); len(got) != 0 {
		t.Errorf("got %+v, want none", got)
	}
}

// "Matching first seats the most people" (docs/spec/matching.md, "Sizes"): the
// most people over the whole assignment, not date by date. Monday seats 4 and
// Tuesday 5, but taking Tuesday's five first strands p3 and p4; Monday's four
// with Tuesday's other three seat all seven.
func TestSeatsTheMostOverTheWholeAssignment(t *testing.T) {
	mk := func(id string, ticks ...matching.Slot) matching.Signup {
		return matching.Signup{Person: matching.PersonID(id), Ticks: ticks, Languages: []string{"en"}}
	}
	got := matching.Match(matching.Input{Signups: []matching.Signup{
		mk("p1", monMorning, tueMorning), mk("p2", monMorning, tueMorning),
		mk("p3", monMorning), mk("p4", monMorning),
		mk("p5", tueMorning), mk("p6", tueMorning), mk("p7", tueMorning),
	}})
	seated := 0
	for _, g := range got {
		seated += len(g.People)
	}
	if seated != 7 {
		t.Fatalf("seated %d, want all 7: %+v", seated, got)
	}
	// The two ways to seat 7 are equal on sizes and groups (4 and 3); the date
	// that seats the most wins, and 4 on Monday is earlier than 4 on Tuesday.
	if len(got) != 2 || got[0].Date != monMorning.Date || len(got[0].People) != 4 || got[1].Date != tueMorning.Date {
		t.Errorf("got %+v, want 4 on Monday and 3 on Tuesday", got)
	}
}

// Sizes are weighed over every date together. Monday's six seat only 6 and
// leave Tuesday's two stranded; splitting them seats all 8, and 4 and 4 is
// nearer 4 or 5 than 3 and 5.
func TestSizesAreWeighedAcrossDates(t *testing.T) {
	mk := func(id string, ticks ...matching.Slot) matching.Signup {
		return matching.Signup{Person: matching.PersonID(id), Ticks: ticks, Languages: []string{"en"}}
	}
	// a, b, c can do only Monday; d, e, f can do Monday or Tuesday; g, h only
	// Tuesday.
	got := matching.Match(matching.Input{Signups: []matching.Signup{
		mk("a", monMorning), mk("b", monMorning), mk("c", monMorning),
		mk("d", monMorning, tueMorning), mk("e", monMorning, tueMorning), mk("f", monMorning, tueMorning),
		mk("g", tueMorning), mk("h", tueMorning),
	}})
	if want := []int{4, 4}; !slices.Equal(sizes(got), want) {
		t.Errorf("sizes %v, want %v: %+v", sizes(got), want, got)
	}
}

// Sixty is the Pilot's active people (docs/spec/matching.md, "How many plans
// are open"), and a round can be all of them at one time.
func TestAPilotSizedRoundIsExact(t *testing.T) {
	for _, tc := range []struct {
		people int
		want   []int
	}{
		{18, []int{5, 5, 4, 4}},
		{60, slices.Repeat([]int{5}, 12)},
		{61, slices.Concat(slices.Repeat([]int{5}, 9), slices.Repeat([]int{4}, 4))},
	} {
		t.Run(fmt.Sprint(tc.people), func(t *testing.T) {
			got := sizes(matching.Match(matching.Input{Signups: crowd(tc.people, monMorning)}))
			if !slices.Equal(got, tc.want) {
				t.Errorf("%d people made groups of %v, want %v", tc.people, got, tc.want)
			}
		})
	}
}

// A person who appears twice with the same entry counts once. One who appears
// with entries that differ can't be told apart from a mistake, so they are
// left out whatever the order of the entries: matching never guesses which
// entry is right when a hard rule (a language, a setting) hangs on it.
func TestADuplicateSignupWithDifferentContentIsLeftOut(t *testing.T) {
	base := crowd(3, monMorning)
	other := base[0]
	other.Ticks = []matching.Slot{monMorning, tueMorning}
	for _, signups := range [][]matching.Signup{
		append(slices.Clone(base), other),
		append([]matching.Signup{other}, base...),
	} {
		got := matching.Match(matching.Input{Signups: signups})
		if len(got) != 0 {
			t.Errorf("got %+v, want none: p00 appears twice with different ticks, leaving two people", got)
		}
	}
	// The same entry twice, with ticks and languages in another order.
	same := base[0]
	same.Ticks = []matching.Slot{monMorning, monMorning}
	same.Languages = []string{"en", "en"}
	if got := matching.Match(matching.Input{Signups: append(slices.Clone(base), same)}); len(got) != 1 || len(got[0].People) != 3 {
		t.Errorf("got %+v, want one group of 3", got)
	}
}
