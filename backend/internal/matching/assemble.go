package matching

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
)

// Proposal is one outing offered to a group.
type Proposal struct {
	Area      AreaID
	Slot      Slot
	Primary   string // the event's language
	Secondary string // its second language, for two-language events
	Plan      Plan
	// Invited is everyone invited (sorted): the target size plus up to
	// cfg.ExtraInvites more, because some will decline. Together they
	// satisfy every hard rule.
	Invited []UserID
	// Quorum is how many acceptances confirm the event straight away: the
	// target size, or fewer when fewer than target + ExtraInvites could be
	// invited, but never below MinAttendance.
	Quorum int
	// MinAttendance is the fewest acceptances the event can still go ahead
	// with at the response deadline (cfg.MinGroupSize), so a decline in a
	// small group doesn't cancel it.
	MinAttendance int
	// Score is the group score (see Weights), for ranking and diagnostics.
	Score float64
}

type candidate struct {
	bucket *Bucket
	plan   Plan
}

// group is a proposal being assembled.
type group struct {
	cand    candidate
	act     *Activity
	target  int // the attendance aimed for
	cap     int // the most to invite: target + extras, within the activity's maximum
	members []*User
	// eligible are the bucket's members interested in the activity, in
	// bucket order, placed or not: swaps can move placed people.
	eligible []*User
}

// Assemble runs steps 3 and 4. Candidates are taken best first and each is
// filled greedily from people not yet placed, up to its target size plus
// cfg.ExtraInvites (step 4: over-invite, confirm on quorum). A local search
// then swaps people between groups, or with people left over, while that
// raises the total score (each group's score × its size). Each person is
// invited at most once.
func Assemble(cands []candidate, users map[UserID]*User, acts map[ActivityID]*Activity, rel Relations, cfg Config) []Proposal {
	// Two-language buckets contain everyone in both single-language buckets,
	// so they'd always win on demand. They go after single-language
	// candidates of the same score, and at a discount, so people are first
	// grouped in a language they all share.
	rank := func(c candidate) float64 {
		if c.bucket.Bilingual() {
			return c.plan.Score * cfg.BilingualWeight
		}
		return c.plan.Score
	}
	order := slices.Clone(cands)
	slices.SortStableFunc(order, func(a, b candidate) int { return cmp.Compare(rank(b), rank(a)) })

	placed := map[UserID]bool{}
	var groups []*group
	for _, c := range order {
		act := acts[c.plan.Activity]
		var eligible []*User
		for _, id := range c.bucket.Members {
			if _, ok := interest(users[id], act, cfg); ok {
				eligible = append(eligible, users[id])
			}
		}
		// A popular plan in a big bucket can fill several groups.
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
			g := &group{cand: c, act: act, eligible: eligible, target: targetSize(free, act, cfg)}
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
	}

	groups = localSearch(groups, placed, rel, cfg)

	out := make([]Proposal, 0, len(groups))
	for _, g := range groups {
		b := g.cand.bucket
		out = append(out, Proposal{
			Area: b.Area, Slot: b.Slot, Primary: b.Primary, Secondary: b.Secondary,
			Plan: g.cand.plan, Invited: ids(g.members),
			Quorum: g.quorum(len(g.members), cfg), MinAttendance: cfg.MinGroupSize,
			Score: g.score(g.members, rel, cfg),
		})
	}
	slices.SortFunc(out, func(a, b Proposal) int {
		return cmp.Or(compareSlots(a.Slot, b.Slot), cmp.Compare(a.Area, b.Area),
			cmp.Compare(a.Primary, b.Primary), cmp.Compare(a.Secondary, b.Secondary),
			cmp.Compare(a.Plan.Activity, b.Plan.Activity), slices.Compare(a.Invited, b.Invited))
	})
	return out
}

// targetSize is the median of the eligible people's preferred sizes, within
// the activity's limits.
func targetSize(eligible []*User, act *Activity, cfg Config) int {
	sizes := make([]int, len(eligible))
	for i, u := range eligible {
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
		if v, _ := interest(u, g.act, cfg); !placed[u.ID] && v > best {
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
		v, _ := interest(u, g.act, cfg)
		interestSum += v
		t := float64(prefSize(u, cfg))
		fitSum += max(0, 1-math.Abs(expected-t)/t)
		reliability += u.Reliability
		if u.EventsAttended == 0 {
			newcomers++
		}
	}
	mutual := 0.0
	for i, a := range ms {
		for _, b := range ms[i+1:] {
			mutual += rel.Mutual[MakePair(a.ID, b.ID)]
		}
	}
	// Newcomers: some are good (that's how people join), a majority isn't
	// (nobody knows how it works yet).
	mix := 0.0
	if k := float64(newcomers); k > 0 && 2*k <= n {
		mix = 1
	} else if k > 0 {
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
		w.Mutual*min(1, mutual) +
		w.Newcomers*mix +
		w.Reliability*reliability/n
}

// Violation reports the first hard rule a group breaks, or nil.
//
//   - nobody is grouped with someone either of them blocked
//   - someone who asked for same-gender groups only gets exactly that
//   - at most cfg.MaxKnown people already know each other (a mutual pick)
//
// Removing people never breaks these rules, so a group that passes still
// passes after declines. Everything else (size, gender balance) is a
// preference, in the score.
func Violation(members []*User, rel Relations, cfg Config) error {
	known := map[UserID]bool{}
	for i, a := range members {
		for _, b := range members[i+1:] {
			p := MakePair(a.ID, b.ID)
			if rel.Blocked[p] {
				return fmt.Errorf("%s and %s blocked each other", p.A, p.B)
			}
			if (a.SameGenderOnly || b.SameGenderOnly) && a.Gender != b.Gender {
				return fmt.Errorf("%s and %s: same-gender only", p.A, p.B)
			}
			if rel.Mutual[p] > 0 {
				known[a.ID], known[b.ID] = true, true
			}
		}
	}
	if len(known) > cfg.MaxKnown {
		return errors.New("too many people already know each other")
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
