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
	// Signups are the round's, one for each person. A person who appears more
	// than once with the same entry counts once; one who appears with entries
	// that differ is never grouped, whatever the order of the entries, because
	// matching can't know which one a language or a setting should come from.
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

// The search is a branch and bound over the whole assignment. It is exact
// unless it runs out of work, and then it returns the best valid groups it has
// found, which are always valid but may not be the best. The limits count
// search nodes and candidate groups weighed, never time, so a result is the
// same on every machine. The README says how often they bite.
//
// The search first looks for what the rules allow at best: with a crowd of
// people who can all be grouped it stops as soon as it has seated everyone in
// the nearest sizes. Where the rules leave a puzzle it keeps looking until it
// has proved nothing seats more people in nearer sizes and fewer groups.
const (
	// seatWork is the budget for seating the most people, then the sizes and
	// the fewest groups.
	seatWork = 400_000
	// dateWork is the budget for the tie-break among assignments that are
	// equal on those three: the date that seats the most.
	dateWork = 100_000
	// groupsPerNode bounds how many candidate groups the search weighs for one
	// person.
	groupsPerNode = 4_000
)

// Match forms the groups and dates for a round.
//
// It ranks every way to group the signups as the spec words it ("Sizes" in
// docs/spec/matching.md), over the whole assignment and not date by date: the
// most people seated first, then sizes nearest 4 or 5 (the sum of each group's
// distance from that range), then fewer groups. Among assignments equal on
// those, the date that seats the most people wins and a tie goes to the
// earliest; then the next date is weighed the same way. Anything still equal
// is settled by the order the search tries things in, which depends only on
// the people and the rules. The same input always gives the same output,
// whatever the order of the signups.
func Match(in Input) []Group {
	p := newPool(in)
	if len(p.people) < policy.MinGroupSize {
		return nil
	}
	s := newSearch(p)
	s.run(false)
	s.run(true)
	groups := make([]Group, len(s.best.chosen))
	for i, g := range s.best.chosen {
		ids := make([]PersonID, len(g.members))
		for j, m := range g.members {
			ids[j] = s.cands[m].id
		}
		slices.Sort(ids)
		groups[i] = p.group(s.slots[g.slot], ids)
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
	ticks     []bool // by index into pool.slots
	apart     map[PersonID]bool
	// class numbers people who are interchangeable: the same ticks, languages
	// and setting, and no kept-apart pair. Two of one class never need
	// telling apart, which keeps the search from trying both orders.
	class int
}

// eligible is whether the person can be in a group made up only of women and
// non-binary people.
func (p *person) eligible() bool { return p.gender == Woman || p.gender == NonBinary }

// fits is whether two people could be in one group, setting aside the rest of
// the group and the time: a language, no kept-apart pair, the setting, and a
// time they both ticked.
func fits(a, b *person) bool {
	if a.apart[b.id] || b.apart[a.id] {
		return false
	}
	if a.womenOnly && !b.eligible() || b.womenOnly && !a.eligible() {
		return false
	}
	if len(intersect(a.langs, b.langs)) == 0 {
		return false
	}
	for i := range a.ticks {
		if a.ticks[i] && b.ticks[i] {
			return true
		}
	}
	return false
}

type pool struct {
	people []*person // ordered by ID: those who could be in some group
	slots  []Slot    // the times someone ticked, earliest first
}

// entry is what a signup says, with the order of its lists taken out.
type entry struct {
	ticks     []Slot
	langs     []string
	gender    Gender
	womenOnly bool
}

func normalise(s Signup) entry {
	e := entry{gender: s.Gender, womenOnly: s.WomenAndNonBinaryOnly}
	for _, t := range s.Ticks {
		if slices.Contains(partOrder, t.Part) {
			e.ticks = append(e.ticks, t)
		}
	}
	slices.SortFunc(e.ticks, compareSlots)
	e.ticks = slices.Compact(e.ticks)
	e.langs = slices.Clone(s.Languages)
	slices.Sort(e.langs)
	e.langs = slices.Compact(e.langs)
	return e
}

func (e entry) equal(o entry) bool {
	return e.gender == o.gender && e.womenOnly == o.womenOnly &&
		slices.Equal(e.langs, o.langs) && slices.Equal(e.ticks, o.ticks)
}

func compareSlots(a, b Slot) int {
	return cmp.Or(
		cmp.Compare(a.Date, b.Date),
		cmp.Compare(slices.Index(partOrder, a.Part), slices.Index(partOrder, b.Part)),
	)
}

func newPool(in Input) *pool {
	closed := map[string]bool{}
	for _, d := range in.ClosedDates {
		closed[d] = true
	}
	// One entry for each person, or none when a person's entries differ.
	entries := map[PersonID]entry{}
	conflicted := map[PersonID]bool{}
	for _, s := range in.Signups {
		e := normalise(s)
		if prev, seen := entries[s.Person]; seen && !prev.equal(e) {
			conflicted[s.Person] = true
		}
		entries[s.Person] = e
	}
	ids := make([]PersonID, 0, len(entries))
	for id := range entries {
		if !conflicted[id] {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)

	apart := map[PersonID]map[PersonID]bool{}
	for _, pair := range in.KeptApart {
		if pair.A == pair.B {
			continue
		}
		for _, ends := range [][2]PersonID{{pair.A, pair.B}, {pair.B, pair.A}} {
			if apart[ends[0]] == nil {
				apart[ends[0]] = map[PersonID]bool{}
			}
			apart[ends[0]][ends[1]] = true
		}
	}

	p := &pool{}
	slotSet := map[Slot]bool{}
	for _, id := range ids {
		e := entries[id]
		// A person with the setting on who isn't a woman or non-binary person
		// can be in no group, and nobody without a language can.
		pp := &person{id: id, langs: e.langs, gender: e.gender, womenOnly: e.womenOnly, apart: apart[id]}
		if len(pp.langs) == 0 || pp.womenOnly && !pp.eligible() {
			continue
		}
		for _, t := range e.ticks {
			if !closed[t.Date] {
				slotSet[t] = true
			}
		}
		p.people = append(p.people, pp)
	}
	for s := range slotSet {
		p.slots = append(p.slots, s)
	}
	slices.SortFunc(p.slots, compareSlots)
	for _, pp := range p.people {
		pp.ticks = make([]bool, len(p.slots))
		for _, t := range entries[pp.id].ticks {
			if i, ok := slices.BinarySearchFunc(p.slots, t, compareSlots); ok {
				pp.ticks[i] = true
			}
		}
	}

	// Drop whoever couldn't be grouped with two others, until nobody goes.
	// That is a cheap bound, not the rules: it never drops anyone who could
	// be grouped, so the groups are the same, and it makes the search small.
	for changed := true; changed; {
		changed = false
		for i := 0; i < len(p.people); i++ {
			partners := 0
			for _, o := range p.people {
				if o != p.people[i] && fits(p.people[i], o) {
					partners++
				}
			}
			if partners < policy.MinGroupSize-1 {
				p.people = slices.Delete(p.people, i, i+1)
				i--
				changed = true
			}
		}
	}
	// Only the kept-apart pairs among those left make anyone distinct.
	left := map[PersonID]bool{}
	for _, pp := range p.people {
		left[pp.id] = true
	}
	for _, pp := range p.people {
		kept := map[PersonID]bool{}
		for other := range pp.apart {
			if left[other] {
				kept[other] = true
			}
		}
		pp.apart = kept
	}
	for i, a := range p.people {
		a.class = i
		for _, b := range p.people[:i] {
			if interchangeable(a, b) {
				a.class = b.class
				break
			}
		}
	}
	return p
}

func interchangeable(a, b *person) bool {
	return len(a.apart) == 0 && len(b.apart) == 0 &&
		a.womenOnly == b.womenOnly && a.eligible() == b.eligible() &&
		slices.Equal(a.langs, b.langs) && slices.Equal(a.ticks, b.ticks)
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

func distanceFromPreferred(size int) int {
	switch {
	case size < policy.PreferredGroupSizeLow:
		return policy.PreferredGroupSizeLow - size
	case size > policy.PreferredGroupSizeHigh:
		return size - policy.PreferredGroupSizeHigh
	}
	return 0
}

// group is one group the search has chosen: a time (an index into the slots)
// and its people (indexes into the candidates, the first one lowest).
type chosenGroup struct {
	slot    int
	members []int
}

// dateSeats is how many people one time seats.
type dateSeats struct{ slot, seats int }

// ranking is how good a whole assignment is, in the spec's order.
type ranking struct {
	seated, distance, groups int
	// dates are the times with a group, the one seating the most first, a tie
	// going to the earliest.
	dates  []dateSeats
	chosen []chosenGroup
}

// beats is whether r ranks above other. The date order only breaks a tie among
// assignments equal on the other three, and only when withDates is on.
func (r *ranking) beats(other *ranking, withDates bool) bool {
	if c := cmp.Or(
		-cmp.Compare(r.seated, other.seated),
		cmp.Compare(r.distance, other.distance),
		cmp.Compare(r.groups, other.groups),
	); c != 0 || !withDates {
		return c < 0
	}
	for i := range min(len(r.dates), len(other.dates)) {
		if c := cmp.Or(-cmp.Compare(r.dates[i].seats, other.dates[i].seats), cmp.Compare(r.dates[i].slot, other.dates[i].slot)); c != 0 {
			return c < 0
		}
	}
	return false
}

func rank(chosen []chosenGroup, slots int) *ranking {
	r := &ranking{chosen: slices.Clone(chosen)}
	seats := make([]int, slots)
	for _, g := range chosen {
		r.seated += len(g.members)
		r.distance += distanceFromPreferred(len(g.members))
		r.groups++
		seats[g.slot] += len(g.members)
	}
	for slot, n := range seats {
		if n > 0 {
			r.dates = append(r.dates, dateSeats{slot, n})
		}
	}
	slices.SortFunc(r.dates, func(a, b dateSeats) int {
		return cmp.Or(-cmp.Compare(a.seats, b.seats), cmp.Compare(a.slot, b.slot))
	})
	return r
}

// search is a depth-first branch and bound over the whole assignment. It takes
// the lowest undecided person and either puts them in a group, at any time
// they ticked and with any later undecided people who fit, or leaves them out.
type search struct {
	cands   []*person
	slots   []Slot
	decided []bool
	blocked []bool // by class: see groupsWith
	// nbrs are the candidates each one fits with (see fits). deg counts, for
	// each undecided candidate, the undecided ones among them; and seatable
	// counts the undecided candidates with at least two, the only ones who
	// could still be in a group.
	nbrs     [][]int
	deg      []int
	seatable int

	work, budget int
	withDates    bool
	done         bool
	best         *ranking

	// ideal[m] is the best m people could do on sizes, whatever the rules
	// leave: the least distance and the fewest groups, or not ok when no
	// groups of 3 to 6 seat exactly m.
	ideal []idealCost
	// idealSlot is the earliest time that, for the most seated, could seat
	// them all.
	idealSlot int
}

func newSearch(p *pool) *search {
	s := &search{cands: p.people, slots: p.slots}
	n := len(s.cands)
	s.ideal = idealSizes(n)
	s.blocked = make([]bool, n)
	s.nbrs = make([][]int, n)
	for i, a := range s.cands {
		for j, b := range s.cands {
			if i != j && fits(a, b) {
				s.nbrs[i] = append(s.nbrs[i], j)
			}
		}
	}
	return s
}

// decide takes candidate x out of the undecided, and undecide puts them back;
// calls nest like a stack.
func (s *search) decide(x int) {
	s.decided[x] = true
	if s.deg[x] >= policy.MinGroupSize-1 {
		s.seatable--
	}
	for _, j := range s.nbrs[x] {
		if !s.decided[j] {
			s.deg[j]--
			if s.deg[j] == policy.MinGroupSize-2 {
				s.seatable--
			}
		}
	}
}

func (s *search) undecide(x int) {
	s.decided[x] = false
	for _, j := range s.nbrs[x] {
		if !s.decided[j] {
			if s.deg[j] == policy.MinGroupSize-2 {
				s.seatable++
			}
			s.deg[j]++
		}
	}
	if s.deg[x] >= policy.MinGroupSize-1 {
		s.seatable++
	}
}

type idealCost struct {
	distance, groups int
	ok               bool
}

// idealSizes is, for each m up to n, the least total distance from 4 or 5, then
// the fewest groups, that m people can be split into groups of 3 to 6 with.
func idealSizes(n int) []idealCost {
	best := make([]idealCost, n+1)
	best[0].ok = true
	for i := 1; i <= n; i++ {
		for size := policy.MinGroupSize; size <= min(i, policy.MaxGroupSize); size++ {
			prev := best[i-size]
			if !prev.ok {
				continue
			}
			c := idealCost{prev.distance + distanceFromPreferred(size), prev.groups + 1, true}
			if !best[i].ok || c.distance < best[i].distance || c.distance == best[i].distance && c.groups < best[i].groups {
				best[i] = c
			}
		}
	}
	return best
}

// run searches once: for the most seated, sizes and groups, or (withDates)
// for the date tie-break, starting from what the first run found.
func (s *search) run(withDates bool) {
	s.withDates = withDates
	s.done = false
	s.work = 0
	s.budget = seatWork
	s.idealSlot = -1
	if withDates {
		s.budget = dateWork
		// The earliest time that could seat everyone who can be seated.
		for i := range s.slots {
			n := 0
			for _, c := range s.cands {
				if c.ticks[i] {
					n++
				}
			}
			if n >= s.best.seated {
				s.idealSlot = i
				break
			}
		}
	}
	if s.best != nil && s.settled() {
		return
	}
	s.decided = make([]bool, len(s.cands))
	s.deg = make([]int, len(s.cands))
	s.seatable = 0
	for i, n := range s.nbrs {
		s.deg[i] = len(n)
		if len(n) >= policy.MinGroupSize-1 {
			s.seatable++
		}
	}
	s.descend(nil, 0, 0, 0)
}

func (s *search) descend(chosen []chosenGroup, seated, distance, groups int) {
	if s.done {
		return
	}
	s.work++
	if s.seatable == 0 {
		// Whoever is left can't be in a group.
		s.leaf(chosen)
		return
	}
	if s.best != nil {
		// Even if everyone who could still be seated were, could this beat the
		// best? A tie on the most seated can only be won on sizes: the sizes
		// and groups only grow, and seating m more people in groups of 3 to 6
		// costs at least what the ideal split of m costs.
		if upper := seated + s.seatable; upper < s.best.seated {
			return
		} else if upper == s.best.seated {
			more := s.ideal[s.seatable]
			switch {
			case !more.ok, distance+more.distance > s.best.distance:
				return
			case distance+more.distance == s.best.distance && groups+more.groups > s.best.groups:
				return
			}
		}
		if s.work > s.budget {
			s.done = true
			return
		}
	}
	first := s.mostConstrained()
	s.decide(first)
	candidates := s.groupsWith(first)
	s.work += len(candidates)
	for _, g := range candidates {
		for _, c := range g.members[1:] {
			s.decide(c)
		}
		s.descend(append(chosen, g), seated+len(g.members), distance+distanceFromPreferred(len(g.members)), groups+1)
		for i := len(g.members) - 1; i >= 1; i-- {
			s.undecide(g.members[i])
		}
		if s.done {
			break
		}
	}
	if !s.done {
		// Leave the first person out.
		s.descend(chosen, seated, distance, groups)
	}
	s.undecide(first)
}

// mostConstrained is the undecided candidate who fits with the fewest other
// undecided ones, the lowest ID on a tie. Deciding the person with the fewest
// ways out first keeps the search small.
func (s *search) mostConstrained() int {
	pick := -1
	for i := range s.cands {
		if !s.decided[i] && (pick < 0 || s.deg[i] < s.deg[pick]) {
			pick = i
		}
	}
	return pick
}

func (s *search) leaf(chosen []chosenGroup) {
	if r := rank(chosen, len(s.slots)); s.best == nil || r.beats(s.best, s.withDates) {
		s.best = r
	}
	s.done = s.settled()
}

// settled is whether the best found can't be beaten, so the search can stop.
func (s *search) settled() bool {
	r := s.best
	if !s.withDates {
		// Nothing seats more people, in nearer sizes, in fewer groups.
		ideal := s.ideal[len(s.cands)]
		return ideal.ok && r.seated == len(s.cands) && r.distance == ideal.distance && r.groups == ideal.groups
	}
	// Everyone seated at the earliest time that could seat them.
	return len(r.dates) == 1 && r.dates[0].slot == s.idealSlot
}

// groupsWith lists the groups that include candidate first and any other
// undecided candidates, at any time first ticked, best first: nearest 4 or 5,
// then larger, then the earlier time.
func (s *search) groupsWith(first int) []chosenGroup {
	var out []chosenGroup
	f := s.cands[first]
	for slot := range s.slots {
		if !f.ticks[slot] {
			continue
		}
		var grow func(members []int, langs []string, active, allEligible bool, from int)
		grow = func(members []int, langs []string, active, allEligible bool, from int) {
			if len(members) >= policy.MinGroupSize {
				out = append(out, chosenGroup{slot, slices.Clone(members)})
			}
			if len(members) == policy.MaxGroupSize {
				return
			}
			var skipped []int // classes blocked for the rest of this loop
			for c := from; c < len(s.cands) && len(out) < groupsPerNode; c++ {
				cand := s.cands[c]
				// Of interchangeable people, take the lowest IDs: a later one
				// is tried only once every earlier one of its class is in.
				if s.decided[c] || !cand.ticks[slot] || s.blocked[cand.class] {
					continue
				}
				shared := intersect(langs, cand.langs)
				if len(shared) == 0 {
					continue
				}
				nowActive, nowEligible := active || cand.womenOnly, allEligible && cand.eligible()
				if nowActive && !nowEligible {
					continue
				}
				if s.apartFromAny(cand, members) {
					continue
				}
				grow(append(members, c), shared, nowActive, nowEligible, c+1)
				s.blocked[cand.class] = true
				skipped = append(skipped, cand.class)
			}
			for _, class := range skipped {
				s.blocked[class] = false
			}
		}
		grow([]int{first}, f.langs, f.womenOnly, f.eligible(), 0)
	}
	slices.SortStableFunc(out, func(a, b chosenGroup) int {
		return cmp.Or(
			cmp.Compare(distanceFromPreferred(len(a.members)), distanceFromPreferred(len(b.members))),
			-cmp.Compare(len(a.members), len(b.members)),
		)
	})
	return out
}

func (s *search) apartFromAny(cand *person, members []int) bool {
	for _, m := range members {
		if cand.apart[s.cands[m].id] {
			return true
		}
	}
	return false
}
