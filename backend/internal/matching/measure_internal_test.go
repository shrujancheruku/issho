//go:build measure

package matching

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// The two tests below print the figures the README's "How exact it is" section
// quotes. They are slow (about two and a half minutes together), so they run
// only on request:
//
//	cd backend && go test -tags measure -run TestMeasure -v ./internal/matching
//
// Every round is random, from a fixed seed, in the shape the property tests
// generate: three languages (each person speaks each with chance 0.7), a
// quarter with the women-and-non-binary setting, a kept-apart pair for every
// three people, and six times to tick. Each is searched as Match searches it,
// then again with ten times the work, to see what a longer search finds.
var measureSlots = []Slot{
	{"2026-12-07", Morning}, {"2026-12-07", Afternoon}, {"2026-12-07", Evening},
	{"2026-12-08", Morning}, {"2026-12-08", Afternoon}, {"2026-12-08", Evening},
}

func measureRound(r *rand.Rand, people int, tickChance float64) Input {
	slots := measureSlots
	genders := []Gender{Woman, Man, NonBinary, Undeclared}
	var in Input
	for i := range people {
		sg := Signup{
			Person:                PersonID(fmt.Sprintf("p%02d", i)),
			Gender:                genders[r.IntN(len(genders))],
			WomenAndNonBinaryOnly: r.Float64() < 0.25,
		}
		for _, l := range []string{"vi", "en", "ja"} {
			if r.Float64() < 0.7 {
				sg.Languages = append(sg.Languages, l)
			}
		}
		for _, sl := range slots {
			if r.Float64() < tickChance {
				sg.Ticks = append(sg.Ticks, sl)
			}
		}
		in.Signups = append(in.Signups, sg)
	}
	for range people / 3 {
		in.KeptApart = append(in.KeptApart, Pair{
			A: PersonID(fmt.Sprintf("p%02d", r.IntN(people))),
			B: PersonID(fmt.Sprintf("p%02d", r.IntN(people))),
		})
	}
	return in
}

// measureSearch runs both passes of Match at a multiple of the work limits. It
// says whether the first (the most seated, then sizes and groups) ran out of
// work, whether either did, and the first's steps.
func measureSearch(in Input, scale int) (s *search, firstRanOut, anyRanOut bool, firstWork int) {
	s = newSearch(newPool(in))
	s.scale = scale
	s.run(false)
	firstRanOut, firstWork = s.work > s.budget, s.work
	s.run(true)
	return s, firstRanOut, firstRanOut || s.work > s.budget, firstWork
}

func TestMeasureSmallAndOpenRounds(t *testing.T) {
	// Rounds of up to 14 people, each person ticking about half the times.
	r := rand.New(rand.NewPCG(8, 617))
	small, smallRanOut := 300, 0
	for range small {
		in := measureRound(r, r.IntN(15), 0.5)
		if len(newPool(in).people) < 3 {
			continue
		}
		if _, _, ran, _ := measureSearch(in, 1); ran {
			smallRanOut++
		}
	}
	t.Logf("up to 14 people, half the times ticked: %d random rounds, %d ran out of work", small, smallRanOut)

	// 60 people who can all meet at any of the six times.
	var open Input
	for i := range 60 {
		open.Signups = append(open.Signups, Signup{Person: PersonID(fmt.Sprintf("p%02d", i)), Ticks: measureSlots, Languages: []string{"en"}})
	}
	_, _, openRanOut, openWork := measureSearch(open, 1)
	t.Logf("60 people who can all meet at any of six times: %d steps for the most seated, ran out of work: %v", openWork, openRanOut)
}

func TestMeasureLargerRounds(t *testing.T) {
	// Rounds of 14 to 60 people, three tick densities.
	r := rand.New(rand.NewPCG(9, 617))
	const each = 20
	var rounds, firstRanOut, differ, seatedFewer int
	var mostSteps int
	for _, people := range []int{14, 18, 24, 40, 50, 60} {
		for _, density := range []float64{0.3, 0.6, 0.9} {
			var ran, diff, fewer int
			for range each {
				in := measureRound(r, people, density)
				short, first, _, firstWork := measureSearch(in, 1)
				mostSteps = max(mostSteps, firstWork)
				long, _, _, _ := measureSearch(in, 10)
				rounds++
				if first {
					ran++
				}
				a, b := short.best, long.best
				switch {
				case a.seated != b.seated:
					diff++
					if a.seated < b.seated {
						fewer++
					}
				case a.distance != b.distance || a.groups != b.groups:
					diff++
				}
			}
			t.Logf("%2d people, ticks at %.1f: %d rounds, %d ran out of work, %d differ from ten times the work, %d seated fewer", people, density, each, ran, diff, fewer)
			firstRanOut += ran
			differ += diff
			seatedFewer += fewer
		}
	}
	t.Logf("TOTAL: %d rounds of 14 to 60 people; the first search ran out of work in %d; %d differed in seated, sizes or groups from ten times the work; %d of those seated fewer; the most steps the first search took in one round: %d (its limit is seatWork, %d)",
		rounds, firstRanOut, differ, seatedFewer, mostSteps, seatWork)
}
