// Package matching forms groups and sets dates among a round's signups. It is
// a pure function: no database, no clock and no outside service, so a decision
// can be replayed and explained ("Matching is a pure function" in
// docs/spec/architecture.md, and docs/spec/matching.md).
//
// This is the thin loop's matching: only the rules that are never broken. A
// group is 3 to 6 people who all ticked its time, share a spoken language and
// hold no kept-apart pair; a person with the "only women and non-binary
// people" setting is only grouped with women and non-binary people; nobody is
// in two groups; no group is on a closed date. The seat order, preferences and
// rounds come with the outing in full (docs/spec/build-order.md).
//
// Dates are the city's calendar dates ("2006-01-02", as cityclock.Date prints
// them) and the caller does the clock work: the part of the day each tick
// means, and the closed dates, are inputs. That keeps this package free of
// time.
package matching

import (
	"cmp"
	"slices"
	"sort"

	"github.com/shrujancheruku/issho/backend/internal/policy"
)

// PersonID names a person. Matching only compares it, and orders by it so that
// the order of the input never changes the result.
type PersonID string

// Part is a part of the day in the city's clock hours. Its values are
// cityclock.Part's, so the caller converts with Part(p).
type Part string

const (
	Morning   Part = "morning"
	Afternoon Part = "afternoon"
	Evening   Part = "evening"
)

// partOrder is the order of a day: a tie between two parts of one date goes to
// the earlier.
var partOrder = []Part{Morning, Afternoon, Evening}

// Gender is what a person declared, which matching reads for one rule only:
// the "only women and non-binary people" setting (docs/spec/people.md).
type Gender string

const (
	Woman     Gender = "woman"
	Man       Gender = "man"
	NonBinary Gender = "non-binary"
	// Undeclared is "prefer not to say".
	Undeclared Gender = "undeclared"
)

// Slot is one tick: a part of a real date, "2026-12-07" and Morning.
type Slot struct {
	Date string
	Part Part
}

// Signup is one person's entry to the round.
type Signup struct {
	Person PersonID
	// Ticks are the times the person can go. A tick whose Part isn't one of
	// the three is ignored.
	Ticks []Slot
	// Languages are the languages the person speaks, as codes ("vi", "en").
	Languages []string
	// WomenAndNonBinaryOnly is the "only women and non-binary people" setting.
	// It holds only while Gender is Woman or NonBinary: a person with it on who
	// isn't one is never grouped, since no group could meet it.
	WomenAndNonBinaryOnly bool
	Gender                Gender
}

// Pair is two people who are never grouped (docs/spec/safety.md).
type Pair struct{ A, B PersonID }

// Input is everything matching reads. It reads nothing else: not home area,
// tastes, age or display language (docs/spec/matching.md).
type Input struct {
	// Signups are the round's. A person who appears twice counts once, by
	// their first entry.
	Signups []Signup
	// KeptApart are the pairs who are never grouped, in either order.
	KeptApart []Pair
	// ClosedDates are the city calendar's closed days, "2006-01-02".
	ClosedDates []string
}

// Group is one future outing: its people on one date and part of the day.
type Group struct {
	Date string
	Part Part
	// People are ordered by ID.
	People []PersonID
	// Languages are every language all of People speak, sorted. Never empty.
	Languages []string
}

// searchSteps bounds the search for the best groups at one time, so a large
// round still returns. It counts search nodes, never time, so the result is the
// same on every machine. Rounds are small (a Pilot city has about 60 people
// over a dozen plans), so the search is exact in practice: the cap only changes
// the answer for a crowd of a few dozen at one time, and then it still returns
// a valid, if not the best, set of groups.
const searchSteps = 200_000

// groupsPerNode bounds how many candidate groups the search weighs for one
// seat. Like searchSteps, it matters only for a crowd at one time.
const groupsPerNode = 2_000

// Match forms the groups and dates for a round.
//
// Among the times people ticked, the one that seats the most people wins and
// a tie goes to the earliest; its groups are formed, their people leave the
// pool, and the next time is chosen the same way from those who remain. At
// one time the groups seat the most people first, then prefer sizes nearest
// 4 or 5, then fewer groups ("Sizes" in docs/spec/matching.md). The same input
// always gives the same output, whatever the order of the signups.
func Match(in Input) []Group {
	pool := newPool(in)
	var groups []Group
	for {
		var (
			bestSlot Slot
			best     outcome
		)
		for _, slot := range pool.slots() {
			if o := pool.solve(slot); o.seated > best.seated {
				bestSlot, best = slot, o
			}
		}
		if best.seated == 0 {
			break
		}
		for _, members := range best.groups {
			groups = append(groups, pool.group(bestSlot, members))
			for _, p := range members {
				pool.remove(p)
			}
		}
	}
	slices.SortFunc(groups, func(a, b Group) int {
		return cmp.Or(
			cmp.Compare(a.Date, b.Date),
			cmp.Compare(slices.Index(partOrder, a.Part), slices.Index(partOrder, b.Part)),
			cmp.Compare(a.People[0], b.People[0]),
		)
	})
	return groups
}

type person struct {
	id        PersonID
	langs     []string // sorted, no repeats
	gender    Gender
	womenOnly bool
	slots     map[Slot]bool
	apart     map[PersonID]bool
}

// eligible is whether the person can be in a group made up only of women and
// non-binary people.
func (p *person) eligible() bool { return p.gender == Woman || p.gender == NonBinary }

type pool struct {
	people []*person // ordered by ID, those not yet in a group
}

func newPool(in Input) *pool {
	closed := map[string]bool{}
	for _, d := range in.ClosedDates {
		closed[d] = true
	}
	apart := map[PersonID]map[PersonID]bool{}
	for _, pair := range in.KeptApart {
		for _, ends := range [][2]PersonID{{pair.A, pair.B}, {pair.B, pair.A}} {
			if apart[ends[0]] == nil {
				apart[ends[0]] = map[PersonID]bool{}
			}
			apart[ends[0]][ends[1]] = true
		}
	}
	seen := map[PersonID]bool{}
	p := &pool{}
	for _, s := range in.Signups {
		if seen[s.Person] {
			continue
		}
		seen[s.Person] = true
		langs := slices.Clone(s.Languages)
		slices.Sort(langs)
		pp := &person{
			id:        s.Person,
			langs:     slices.Compact(langs),
			gender:    s.Gender,
			womenOnly: s.WomenAndNonBinaryOnly,
			slots:     map[Slot]bool{},
			apart:     apart[s.Person],
		}
		for _, t := range s.Ticks {
			if !closed[t.Date] && slices.Contains(partOrder, t.Part) {
				pp.slots[t] = true
			}
		}
		p.people = append(p.people, pp)
	}
	slices.SortFunc(p.people, func(a, b *person) int { return cmp.Compare(a.id, b.id) })
	return p
}

// slots are the times someone in the pool ticked, earliest first.
func (p *pool) slots() []Slot {
	set := map[Slot]bool{}
	for _, pp := range p.people {
		for s := range pp.slots {
			set[s] = true
		}
	}
	out := make([]Slot, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b Slot) int {
		return cmp.Or(
			cmp.Compare(a.Date, b.Date),
			cmp.Compare(slices.Index(partOrder, a.Part), slices.Index(partOrder, b.Part)),
		)
	})
	return out
}

func (p *pool) remove(id PersonID) {
	p.people = slices.DeleteFunc(p.people, func(pp *person) bool { return pp.id == id })
}

func (p *pool) group(slot Slot, members []PersonID) Group {
	g := Group{Date: slot.Date, Part: slot.Part, People: members}
	for _, pp := range p.people {
		if slices.Contains(members, pp.id) {
			if g.Languages == nil {
				g.Languages = slices.Clone(pp.langs)
			} else {
				g.Languages = intersect(g.Languages, pp.langs)
			}
		}
	}
	return g
}

// intersect is the languages in both sorted lists.
func intersect(a, b []string) []string {
	var out []string
	for _, l := range a {
		if _, found := slices.BinarySearch(b, l); found {
			out = append(out, l)
		}
	}
	return out
}

// outcome is a way to group some of the people at one time, and how it ranks.
type outcome struct {
	groups [][]PersonID
	seated int
	// distance is each group's size away from the preferred 4 or 5, summed.
	distance int
}

func distanceFromPreferred(size int) int {
	switch {
	case size < policy.PreferredGroupSizeLow:
		return policy.PreferredGroupSizeLow - size
	case size > policy.PreferredGroupSizeHigh:
		return size - policy.PreferredGroupSizeHigh
	}
	return 0
}

// beats is whether o ranks above other: the most people seated, then sizes
// nearest 4 or 5, then fewer groups. A tie keeps the one found first.
func (o outcome) beats(other outcome) bool {
	return cmp.Or(
		-cmp.Compare(o.seated, other.seated),
		cmp.Compare(o.distance, other.distance),
		cmp.Compare(len(o.groups), len(other.groups)),
	) < 0
}

// solve finds the best groups of the people who ticked slot.
func (p *pool) solve(slot Slot) outcome {
	s := &search{}
	for _, pp := range p.people {
		if pp.slots[slot] {
			s.cands = append(s.cands, pp)
		}
	}
	s.decided = make([]bool, len(s.cands))
	s.descend(0, nil)
	return s.best
}

// search is a depth-first search over one time's people, in ID order: the first
// undecided person is either put in a group with later people, or left out.
type search struct {
	cands   []*person
	decided []bool
	steps   int
	best    outcome
	found   bool
}

func (s *search) descend(undecidedFrom int, chosen [][]int) {
	s.steps++
	for undecidedFrom < len(s.cands) && s.decided[undecidedFrom] {
		undecidedFrom++
	}
	cur := outcome{}
	left := 0
	for _, g := range chosen {
		cur.seated += len(g)
		cur.distance += distanceFromPreferred(len(g))
	}
	for i := range s.cands {
		if !s.decided[i] {
			left++
		}
	}
	// Even if everyone left were seated, could this beat the best?
	if s.found && cur.seated+left < s.best.seated {
		return
	}
	if left == 0 {
		cur.groups = make([][]PersonID, len(chosen))
		for i, g := range chosen {
			for _, c := range g {
				cur.groups[i] = append(cur.groups[i], s.cands[c].id)
			}
		}
		if !s.found || cur.beats(s.best) {
			s.best, s.found = cur, true
		}
		return
	}
	// Past the step budget the search stops, but only once it has a result.
	if s.found && s.steps > searchSteps {
		return
	}
	first := undecidedFrom
	s.decided[first] = true
	for _, g := range s.groupsWith(first) {
		for _, c := range g {
			s.decided[c] = true
		}
		s.descend(first+1, append(chosen, g))
		for _, c := range g[1:] { // g[0] is first, which stays decided here
			s.decided[c] = false
		}
	}
	// Leave the first person out.
	s.descend(first+1, chosen)
	s.decided[first] = false
}

// groupsWith lists the groups that include candidate first and any later
// undecided candidates, best first: nearest 4 or 5, then larger.
func (s *search) groupsWith(first int) [][]int {
	var out [][]int
	var grow func(members []int, langs []string, anyWomenOnly bool, from int)
	grow = func(members []int, langs []string, anyWomenOnly bool, from int) {
		if len(members) >= policy.MinGroupSize {
			out = append(out, slices.Clone(members))
		}
		if len(members) == policy.MaxGroupSize {
			return
		}
		for c := from; c < len(s.cands) && len(out) < groupsPerNode; c++ {
			if s.decided[c] {
				continue
			}
			cand := s.cands[c]
			shared := intersect(langs, cand.langs)
			if len(shared) == 0 {
				continue
			}
			womenOnly := anyWomenOnly || cand.womenOnly
			if womenOnly && !cand.eligible() {
				continue
			}
			if cand.womenOnly && !s.allEligible(members) {
				continue
			}
			if s.apartFromAny(cand, members) {
				continue
			}
			grow(append(members, c), shared, womenOnly, c+1)
		}
	}
	f := s.cands[first]
	if len(f.langs) > 0 && (!f.womenOnly || f.eligible()) {
		grow([]int{first}, f.langs, f.womenOnly, first+1)
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := distanceFromPreferred(len(out[i])), distanceFromPreferred(len(out[j]))
		if di != dj {
			return di < dj
		}
		return len(out[i]) > len(out[j])
	})
	return out
}

func (s *search) allEligible(members []int) bool {
	for _, m := range members {
		if !s.cands[m].eligible() {
			return false
		}
	}
	return true
}

func (s *search) apartFromAny(cand *person, members []int) bool {
	for _, m := range members {
		if cand.apart[s.cands[m].id] {
			return true
		}
	}
	return false
}
