package matching

import (
	"fmt"
	"math"
	"slices"
)

// Group is one group formed for a scheduled plan.
type Group struct {
	// Invited is everyone asked to confirm (sorted): the target size plus up
	// to cfg.ExtraInvites more, because some will drop out. Together they
	// satisfy every hard rule.
	Invited []UserID
	// Quorum is how many confirmations confirm the event straight away: the
	// target size, or fewer when fewer than target + ExtraInvites could be
	// invited, but never below MinAttendance.
	Quorum int
	// MinAttendance is the fewest confirmations the event can still go
	// ahead with at the response deadline (cfg.MinGroupSize), so a decline
	// in a small group doesn't cancel it.
	MinAttendance int
	// Score is the group score (see Weights), for ranking and diagnostics.
	Score float64
}

// group is a Group being assembled.
type group struct {
	act     *Activity
	target  int // the attendance aimed for
	cap     int // the most to invite: target + extras, within the activity's maximum
	members []*User
	// eligible is everyone who could be in the group, placed or not: swaps
	// can move placed people.
	eligible []*User
	// interestOf is how much a member wants the activity.
	interestOf func(*User) float64
}

// assemble splits people into groups for one activity: greedily (start from
// the keenest, keep adding whoever raises the score most, up to the target
// plus cfg.ExtraInvites), then a local search that fills, replaces, swaps
// and spreads small groups while the total score (each group's score × its
// size) rises. People left over stay unplaced.
func assemble(eligible []*User, act *Activity, interestOf func(*User) float64, rel Relations, cfg Config) []*group {
	placed := map[UserID]bool{}
	var groups []*group
	for {
		var free []*User
		for _, u := range eligible {
			if !placed[u.ID] {
				free = append(free, u)
			}
		}
		if len(free) < cfg.MinGroupSize {
			break
		}
		g := &group{act: act, eligible: eligible, interestOf: interestOf, target: targetSize(free, act, cfg)}
		g.cap = min(g.target+cfg.ExtraInvites, act.MaxSize)
		g.fill(placed, rel, cfg)
		if len(g.members) < cfg.MinGroupSize {
			break
		}
		for _, u := range g.members {
			placed[u.ID] = true
		}
		groups = append(groups, g)
	}
	return localSearch(groups, placed, rel, cfg)
}

// result converts an assembled group.
func (g *group) result(rel Relations, cfg Config) Group {
	return Group{
		Invited: ids(g.members), Quorum: g.quorum(len(g.members), cfg),
		MinAttendance: cfg.MinGroupSize, Score: g.score(g.members, rel, cfg),
	}
}

// targetSize is the median of the people's preferred sizes, within the
// activity's limits.
func targetSize(people []*User, act *Activity, cfg Config) int {
	sizes := make([]int, len(people))
	for i, u := range people {
		sizes[i] = prefSize(u, cfg)
	}
	slices.Sort(sizes)
	return min(max(sizes[len(sizes)/2], act.preferredMin(cfg)), act.MaxSize)
}

// quorum is how many of n invitees confirm the event: all but
// cfg.ExtraInvites, at most the target and at least cfg.MinGroupSize (never
// more than n).
func (g *group) quorum(n int, cfg Config) int {
	return min(n, max(cfg.MinGroupSize, min(g.target, n-cfg.ExtraInvites)))
}

func prefSize(u *User, cfg Config) int {
	if t, ok := cfg.SizeTargets[u.Size]; ok {
		return t
	}
	return cfg.SizeTargets[Medium]
}

// fill builds the group greedily: start from the keenest person, then keep
// adding whoever raises the group score most, up to the invite cap.
func (g *group) fill(placed map[UserID]bool, rel Relations, cfg Config) {
	var seed *User
	best := -1.0
	for _, u := range g.eligible {
		if v := g.interestOf(u); !placed[u.ID] && v > best {
			seed, best = u, v
		}
	}
	g.members = []*User{seed}
	for len(g.members) < g.cap {
		u := g.bestAddition(g.members, placed, rel, cfg, nil)
		if u == nil {
			break
		}
		g.members = append(g.members, u)
	}
	g.balanceGender(placed, rel, cfg)
}

// bestAddition returns the unplaced eligible person who, added to members,
// gives the best score without breaking a hard rule. only, if set,
// restricts the choice (e.g. to one gender).
func (g *group) bestAddition(members []*User, placed map[UserID]bool, rel Relations, cfg Config, only func(*User) bool) *User {
	var best *User
	bestScore := -1.0
	for _, u := range g.eligible {
		if placed[u.ID] || slices.Contains(members, u) || (only != nil && !only(u)) {
			continue
		}
		with := append(slices.Clone(members), u)
		if Violation(with, rel, cfg) != nil {
			continue
		}
		if s := g.score(with, rel, cfg); s > bestScore {
			best, bestScore = u, s
		}
	}
	return best
}

// balanceGender works towards a preference that one-at-a-time growth can't
// see: nobody should be the only woman or the only man. If someone is, it
// adds a second person of that gender when there's room and someone
// suitable. Otherwise the group stays as it is: better a group with a lone
// woman than no group for her (the score already counts against it, so
// local search moves people when it can).
func (g *group) balanceGender(placed map[UserID]bool, rel Relations, cfg Config) {
	for len(g.members) < g.act.MaxSize {
		lone := loneGender(g.members)
		if lone == nil {
			return
		}
		u := g.bestAddition(g.members, placed, rel, cfg, func(u *User) bool { return u.Gender == lone.Gender })
		if u == nil {
			return
		}
		g.members = append(g.members, u)
	}
}

// localSearch improves the greedy result. Moves, tried in a fixed order:
// fill a group that's below its invite cap with an unplaced person, replace a
// member with an unplaced person, move everyone in a group below the
// preferred size into other groups, or swap two members of different groups.
// A move is kept only if the total score rises and every rule still holds.
func localSearch(groups []*group, placed map[UserID]bool, rel Relations, cfg Config) []*group {
	const eps = 1e-9
	value := func(g *group, ms []*User) float64 { return float64(len(ms)) * g.score(ms, rel, cfg) }
	valid := func(ms []*User) bool { return Violation(ms, rel, cfg) == nil }
	eligibleFor := func(g *group, u *User) bool { return slices.Contains(g.eligible, u) }

	for range cfg.SearchPasses {
		improved := false
		for _, g := range groups {
			for len(g.members) > 0 && len(g.members) < g.cap {
				u := g.bestAddition(g.members, placed, rel, cfg, nil)
				if u == nil || !valid(append(slices.Clone(g.members), u)) {
					break
				}
				g.members = append(g.members, u)
				placed[u.ID] = true
				improved = true
			}
			for i := range g.members {
				cur := value(g, g.members)
				for _, u := range g.eligible {
					if placed[u.ID] {
						continue
					}
					next := slices.Clone(g.members)
					next[i] = u
					if valid(next) && value(g, next) > cur+eps {
						placed[g.members[i].ID], placed[u.ID] = false, true
						g.members, cur, improved = next, value(g, next), true
					}
				}
			}
		}
		if dissolveSmall(groups, rel, cfg, value, valid) {
			improved = true
		}
		for gi, a := range groups {
			for _, b := range groups[gi+1:] {
				for i := range a.members {
					for j := range b.members {
						ua, ub := a.members[i], b.members[j]
						if !eligibleFor(b, ua) || !eligibleFor(a, ub) {
							continue
						}
						na, nb := slices.Clone(a.members), slices.Clone(b.members)
						na[i], nb[j] = ub, ua
						if !valid(na) || !valid(nb) {
							continue
						}
						if value(a, na)+value(b, nb) > value(a, a.members)+value(b, b.members)+eps {
							a.members, b.members, improved = na, nb, true
						}
					}
				}
			}
		}
		if !improved {
			break
		}
	}
	// Dissolved groups are left empty; drop them.
	kept := groups[:0]
	for _, g := range groups {
		if len(g.members) > 0 {
			kept = append(kept, g)
		}
	}
	return kept
}

// dissolveSmall tries to spread each group below its preferred minimum
// across other groups: greedy assembly leaves remnants (often pairs) that
// would be better off joining bigger groups. Each member goes to the group
// with room where they raise its value most. The move is kept only if
// everyone finds a place and the total value rises.
func dissolveSmall(groups []*group, rel Relations, cfg Config, value func(*group, []*User) float64, valid func([]*User) bool) bool {
	changed := false
	for _, small := range groups {
		if len(small.members) == 0 || len(small.members) >= small.act.preferredMin(cfg) {
			continue
		}
		next := map[*group][]*User{}
		var touched []*group // in order, so float sums are deterministic
		members := func(g *group) []*User {
			if ms, ok := next[g]; ok {
				return ms
			}
			return g.members
		}
		ok := true
		for _, u := range small.members {
			var best *group
			bestGain := math.Inf(-1)
			for _, g := range groups {
				ms := members(g)
				if g == small || len(ms) == 0 || len(ms) >= g.cap || !slices.Contains(g.eligible, u) {
					continue
				}
				with := append(slices.Clone(ms), u)
				if !valid(with) {
					continue
				}
				if gain := value(g, with) - value(g, ms); gain > bestGain {
					best, bestGain = g, gain
				}
			}
			if best == nil {
				ok = false
				break
			}
			if _, seen := next[best]; !seen {
				touched = append(touched, best)
			}
			next[best] = append(slices.Clone(members(best)), u)
		}
		if !ok {
			continue
		}
		before, after := value(small, small.members), 0.0
		for _, g := range touched {
			before += value(g, g.members)
			after += value(g, next[g])
		}
		if after > before {
			for _, g := range touched {
				g.members = next[g]
			}
			small.members = nil
			changed = true
		}
	}
	return changed
}

// score is the weighted sum of the group's parts, each between 0 and 1.
func (g *group) score(ms []*User, rel Relations, cfg Config) float64 {
	n := float64(len(ms))
	if n == 0 {
		return 0
	}
	var interestSum, fitSum, reliability float64
	newcomers := 0
	// Size scores use the expected attendance, not the number invited:
	// invitees beyond the target are assumed to cover declines.
	expected := float64(min(len(ms), g.target))
	for _, u := range ms {
		interestSum += g.interestOf(u)
		t := float64(prefSize(u, cfg))
		fitSum += max(0, 1-math.Abs(expected-t)/t)
		reliability += u.Reliability
		if u.EventsAttended == 0 {
			newcomers++
		}
	}
	mutual := friendship(ms, rel)
	// Newcomers: welcome, but a majority isn't good (nobody knows how it
	// works yet). A group of regulars with no newcomers is just as good.
	mix := 1.0
	if k := float64(newcomers); 2*k > n {
		mix = 2 * (n - k) / n
	}
	// Groups below the preferred minimum are allowed, but count less.
	minSize := min(1, expected/float64(g.act.preferredMin(cfg)))
	balance := 1.0
	if loneGender(ms) != nil {
		balance = 0
	}
	w := cfg.Weights
	return w.Interest*interestSum/n/cfg.PrefValues[Love] +
		w.SizeFit*fitSum/n +
		w.MinSize*minSize +
		w.Balance*balance +
		w.Mutual*mutual +
		w.Newcomers*mix +
		w.Reliability*reliability/n
}

// friendship scores how much a group already gets on, from 0 (strangers)
// to 1 (everyone a close mutual friend of everyone else), so clusters of
// friends are encouraged rather than capped. It's the average of two
// parts: how many members have a friend there (each member's link
// strengths, up to 1), and how close-knit the whole group is (link
// strengths over all pairs). One pair in a group of five scores 0.25; a
// group who all picked each other scores 1.
func friendship(ms []*User, rel Relations) float64 {
	n := len(ms)
	if n < 2 {
		return 0
	}
	perMember := make([]float64, n)
	total := 0.0
	for i, a := range ms {
		for j := i + 1; j < n; j++ {
			s := rel.Mutual[MakePair(a.ID, ms[j].ID)]
			perMember[i] += s
			perMember[j] += s
			total += s
		}
	}
	coverage := 0.0
	for _, s := range perMember {
		coverage += min(1, s)
	}
	pairs := float64(n * (n - 1) / 2)
	return (coverage/float64(n) + total/pairs) / 2
}

// Violation reports the first hard rule a group breaks, or nil.
//
//   - nobody is grouped with someone either of them blocked
//   - someone who asked for same-gender groups only gets exactly that
//
// There's no cap on people who already know each other: groups of friends
// are encouraged (friendship, in the score).
//
// Removing people never breaks these rules, so a group that passes still
// passes after declines. Everything else (size, gender balance) is a
// preference, in the score.
func Violation(members []*User, rel Relations, _ Config) error {
	for i, a := range members {
		for _, b := range members[i+1:] {
			p := MakePair(a.ID, b.ID)
			if rel.Blocked[p] {
				return fmt.Errorf("%s and %s blocked each other", p.A, p.B)
			}
			if (a.SameGenderOnly || b.SameGenderOnly) && a.Gender != b.Gender {
				return fmt.Errorf("%s and %s: same-gender only", p.A, p.B)
			}
		}
	}
	return nil
}

// loneGender returns someone who is the only woman or the only man, or nil.
// Nonbinary and unspecified are exempt: wanting two of them in every group
// would leave most of them unmatched.
func loneGender(members []*User) *User {
	counts := map[Gender]int{}
	for _, u := range members {
		counts[u.Gender]++
	}
	for _, u := range members {
		if (u.Gender == Woman || u.Gender == Man) && counts[u.Gender] == 1 {
			return u
		}
	}
	return nil
}

func ids(us []*User) []UserID {
	out := make([]UserID, len(us))
	for i, u := range us {
		out[i] = u.ID
	}
	slices.Sort(out)
	return out
}
