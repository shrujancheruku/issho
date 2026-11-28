package matching

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// The search is exact unless it runs out of work (see the README, "How exact
// it is"). Rounds up to 14 people, with each person ticking about half the
// times, never reach the limits, so every answer there is the best one; and
// crowds who can all meet, even 60 of them, are settled before they do.
func TestTheSearchFinishesSmallAndOpenRounds(t *testing.T) {
	slots := []Slot{
		{"2026-12-07", Morning}, {"2026-12-07", Afternoon}, {"2026-12-07", Evening},
		{"2026-12-08", Morning}, {"2026-12-08", Afternoon}, {"2026-12-08", Evening},
	}
	genders := []Gender{Woman, Man, NonBinary, Undeclared}
	r := rand.New(rand.NewPCG(8, 617))
	finishes := func(in Input) bool {
		p := newPool(in)
		if len(p.people) < 3 {
			return true
		}
		s := newSearch(p)
		s.run(false)
		exactSeating := s.work <= s.budget
		s.run(true)
		return exactSeating && s.work <= s.budget
	}
	for round := range 300 {
		var in Input
		for i := range r.IntN(15) {
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
				if r.Float64() < 0.5 {
					sg.Ticks = append(sg.Ticks, sl)
				}
			}
			in.Signups = append(in.Signups, sg)
		}
		if !finishes(in) {
			t.Fatalf("round %d: the search ran out of work on %d people: %+v", round, len(in.Signups), in)
		}
	}
	var open Input
	for i := range 60 {
		open.Signups = append(open.Signups, Signup{Person: PersonID(fmt.Sprintf("p%02d", i)), Ticks: slots, Languages: []string{"en"}})
	}
	if !finishes(open) {
		t.Error("the search ran out of work on 60 people who can all meet at any time")
	}
}
