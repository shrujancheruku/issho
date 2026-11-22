package matching_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"maps"
	"math/rand/v2"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// The property tests generate rounds from a fixed seed, so a failure repeats.
// Each rule below is stated again here, independently of the package: the
// numbers are the spec's own words (docs/spec/matching.md), not policy's.

const propertyRounds = 3000

var (
	allSlots = []matching.Slot{
		{Date: "2026-12-07", Part: matching.Morning}, {Date: "2026-12-07", Part: matching.Afternoon}, {Date: "2026-12-07", Part: matching.Evening},
		{Date: "2026-12-08", Part: matching.Morning}, {Date: "2026-12-08", Part: matching.Afternoon}, {Date: "2026-12-08", Part: matching.Evening},
	}
	allLangs   = []string{"vi", "en", "ja"}
	allGenders = []matching.Gender{matching.Woman, matching.Man, matching.NonBinary, matching.Undeclared}
)

func someOf[T any](r *rand.Rand, from []T, chance float64) []T {
	var out []T
	for _, v := range from {
		if r.Float64() < chance {
			out = append(out, v)
		}
	}
	return out
}

// generate makes a round of up to maxPeople people. Languages, genders, ticks,
// settings, kept-apart pairs and closed dates are all random, and are dense
// enough that groups form most of the time.
func generate(r *rand.Rand, maxPeople int) matching.Input {
	var in matching.Input
	n := r.IntN(maxPeople + 1)
	for i := range n {
		in.Signups = append(in.Signups, matching.Signup{
			Person:                matching.PersonID(fmt.Sprintf("p%02d", i)),
			Ticks:                 someOf(r, allSlots, 0.5),
			Languages:             someOf(r, allLangs, 0.7),
			Gender:                allGenders[r.IntN(len(allGenders))],
			WomenAndNonBinaryOnly: r.Float64() < 0.25,
		})
	}
	for range r.IntN(n + 1) {
		in.KeptApart = append(in.KeptApart, matching.Pair{
			A: matching.PersonID(fmt.Sprintf("p%02d", r.IntN(n+1))),
			B: matching.PersonID(fmt.Sprintf("p%02d", r.IntN(n+1))),
		})
	}
	in.ClosedDates = someOf(r, []string{"2026-12-07", "2026-12-08"}, 0.2)
	return in
}

// problems says which of the never-broken rules a set of people at one time
// breaks, or "" when it breaks none. It is the oracle for every test below.
func problems(in matching.Input, people []matching.PersonID, slot matching.Slot) string {
	if len(people) < 3 || len(people) > 6 {
		return fmt.Sprintf("size %d", len(people))
	}
	if slices.Contains(in.ClosedDates, slot.Date) {
		return "closed date " + slot.Date
	}
	byID := map[matching.PersonID]matching.Signup{}
	for _, s := range in.Signups {
		if _, seen := byID[s.Person]; !seen {
			byID[s.Person] = s
		}
	}
	shared := slices.Clone(byID[people[0]].Languages)
	anyOnly, allEligible := false, true
	for _, id := range people {
		s, known := byID[id]
		switch {
		case !known:
			return "unknown person " + string(id)
		case !slices.Contains(s.Ticks, slot):
			return string(id) + " didn't tick the time"
		}
		shared = slices.DeleteFunc(shared, func(l string) bool { return !slices.Contains(s.Languages, l) })
		anyOnly = anyOnly || s.WomenAndNonBinaryOnly
		allEligible = allEligible && (s.Gender == matching.Woman || s.Gender == matching.NonBinary)
		for _, other := range people {
			for _, pair := range in.KeptApart {
				if pair.A == id && pair.B == other && id != other {
					return fmt.Sprintf("%s and %s are kept apart", id, other)
				}
			}
		}
	}
	if len(shared) == 0 {
		return "no shared language"
	}
	if anyOnly && !allEligible {
		return "women-and-non-binary setting broken"
	}
	return ""
}

func TestPropertyEveryGroupBreaksNoRule(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 617))
	groupsSeen := 0
	for round := range propertyRounds {
		in := generate(r, 16)
		groups := matching.Match(in)
		inGroup := map[matching.PersonID]bool{}
		for _, g := range groups {
			groupsSeen++
			slot := matching.Slot{Date: g.Date, Part: g.Part}
			if p := problems(in, g.People, slot); p != "" {
				t.Fatalf("round %d: group %+v: %s\ninput: %+v", round, g, p, in)
			}
			if !slices.IsSorted(sortedStrings(g.People)) || len(g.Languages) == 0 {
				t.Fatalf("round %d: group %+v: people or languages malformed", round, g)
			}
			for _, id := range g.People {
				if inGroup[id] {
					t.Fatalf("round %d: %s is in two groups: %+v", round, id, groups)
				}
				inGroup[id] = true
			}
			for _, l := range g.Languages {
				for _, id := range g.People {
					if !hasLanguage(in, id, l) {
						t.Fatalf("round %d: group lists %s, which %s doesn't speak", round, l, id)
					}
				}
			}
		}
	}
	if groupsSeen < propertyRounds {
		t.Errorf("only %d groups in %d rounds: the generator is too sparse to test the rules", groupsSeen, propertyRounds)
	}
}

func sortedStrings(ids []matching.PersonID) []string {
	var out []string
	for _, id := range ids {
		out = append(out, string(id))
	}
	return out
}

func hasLanguage(in matching.Input, id matching.PersonID, lang string) bool {
	for _, s := range in.Signups {
		if s.Person == id {
			return slices.Contains(s.Languages, lang)
		}
	}
	return false
}

// Nobody is left out who could have been grouped: the people left over can't
// form a valid group among themselves at any time.
func TestPropertyNoLeftoverGroupWasPossible(t *testing.T) {
	r := rand.New(rand.NewPCG(2, 617))
	for round := range propertyRounds / 3 {
		in := generate(r, 10)
		inGroup := map[matching.PersonID]bool{}
		for _, g := range matching.Match(in) {
			for _, id := range g.People {
				inGroup[id] = true
			}
		}
		var left []matching.PersonID
		for _, s := range in.Signups {
			if !inGroup[s.Person] {
				left = append(left, s.Person)
			}
		}
		for _, slot := range allSlots {
			for mask := 1; mask < 1<<len(left); mask++ {
				var subset []matching.PersonID
				for i, id := range left {
					if mask>>i&1 == 1 {
						subset = append(subset, id)
					}
				}
				if problems(in, subset, slot) == "" {
					t.Fatalf("round %d: %v could still form a group at %v\ninput: %+v", round, subset, slot, in)
				}
			}
		}
	}
}

func TestPropertySameInputSameOutput(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 617))
	for round := range propertyRounds / 3 {
		in := generate(r, 14)
		want := matching.Match(in)
		if got := matching.Match(in); !reflect.DeepEqual(got, want) {
			t.Fatalf("round %d: two runs differ:\n%+v\n%+v", round, want, got)
		}
		// The order of the signups, kept-apart pairs and ticks changes nothing.
		shuffled := in
		shuffled.Signups = slices.Clone(in.Signups)
		r.Shuffle(len(shuffled.Signups), func(i, j int) { shuffled.Signups[i], shuffled.Signups[j] = shuffled.Signups[j], shuffled.Signups[i] })
		for i := range shuffled.Signups {
			ticks := slices.Clone(shuffled.Signups[i].Ticks)
			r.Shuffle(len(ticks), func(a, b int) { ticks[a], ticks[b] = ticks[b], ticks[a] })
			shuffled.Signups[i].Ticks = ticks
		}
		if got := matching.Match(shuffled); !reflect.DeepEqual(got, want) {
			t.Fatalf("round %d: a shuffled input differs:\n%+v\n%+v", round, want, got)
		}
	}
}

// Matching is checked against a brute-force search of every way to seat up to
// eight people at one time (at most two groups): the same seated count, the
// same distance from sizes 4 and 5, and the same number of groups.
func TestPropertyBestGroupingAtOneTime(t *testing.T) {
	r := rand.New(rand.NewPCG(4, 617))
	slot := allSlots[0]
	for round := range 400 {
		in := generate(r, 8)
		for i := range in.Signups {
			in.Signups[i].Ticks = []matching.Slot{slot}
		}
		in.ClosedDates = nil

		var got score
		for _, g := range matching.Match(in) {
			got.seated += len(g.People)
			got.distance += distance(len(g.People))
			got.groups++
		}
		want := bruteForce(in, slot)
		if got != want {
			t.Fatalf("round %d: matching seats %+v, brute force finds %+v\ninput: %+v", round, got, want, in)
		}
	}
}

type score struct{ seated, distance, groups int }

func distance(size int) int {
	switch {
	case size < 4:
		return 4 - size
	case size > 5:
		return size - 5
	}
	return 0
}

// bruteForce tries every assignment of each person to unseated, group 1 or
// group 2, and keeps the best by seated most, then distance, then fewest groups.
func bruteForce(in matching.Input, slot matching.Slot) score {
	n := len(in.Signups)
	best := score{}
	label := make([]int, n)
	var try func(i int)
	try = func(i int) {
		if i < n {
			for l := range 3 {
				label[i] = l
				try(i + 1)
			}
			return
		}
		var s score
		for g := 1; g <= 2; g++ {
			var members []matching.PersonID
			for j, l := range label {
				if l == g {
					members = append(members, in.Signups[j].Person)
				}
			}
			if len(members) == 0 {
				continue
			}
			if problems(in, members, slot) != "" {
				return
			}
			s.seated += len(members)
			s.distance += distance(len(members))
			s.groups++
		}
		if s.seated > best.seated ||
			s.seated == best.seated && s.seated > 0 && (s.distance < best.distance || s.distance == best.distance && s.groups < best.groups) {
			best = s
		}
	}
	try(0)
	return best
}

// The package may import only what a pure function needs: no database, clock
// or outside-service package (docs/spec/architecture.md, "The seams").
func TestImportsNoDatabaseClockOrOutsideService(t *testing.T) {
	allowed := map[string]bool{
		"cmp": true, "slices": true, "sort": true,
		"github.com/shrujancheruku/issho/backend/internal/policy": true,
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		checked++
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if !allowed[path] {
				t.Errorf("%s imports %q: matching may import only %v", file, path, slices.Sorted(maps.Keys(allowed)))
			}
		}
	}
	if checked == 0 {
		t.Fatal("found no source files to check")
	}
}
