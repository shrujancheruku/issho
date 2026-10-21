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
	// Quorum is how many must accept for the event to confirm: the target
	// size, or fewer when fewer than target + ExtraInvites could be invited,
	// but never below the minimum group size.
	Quorum int
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
			if len(free) < act.minSize(cfg) {
				break
			}
			g := &group{cand: c, act: act, eligible: eligible, target: targetSize(free, act, cfg)}
			g.cap = min(g.target+cfg.ExtraInvites, act.MaxSize)
			g.fill(placed, rel, cfg)
			if len(g.members) < act.minSize(cfg) {
				break
			}
			for _, u := range g.members {
				placed[u.ID] = true
			}
			groups = append(groups, g)
		}
	}

	localSearch(groups, placed, rel, cfg)

	out := make([]Proposal, 0, len(groups))
	for _, g := range groups {
		b := g.cand.bucket
		out = append(out, Proposal{
			Area: b.Area, Slot: b.Slot, Primary: b.Primary, Secondary: b.Secondary,
			Plan: g.cand.plan, Invited: ids(g.members),
			Quorum: g.quorum(len(g.members), cfg), Score: g.score(g.members, rel, cfg),
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
	return min(max(sizes[len(sizes)/2], act.minSize(cfg)), act.MaxSize)
}

// quorum is how many of n invitees must accept: all but cfg.ExtraInvites,
// at most the target and at least the minimum group size (never more than
// n). It's also the attendance the size score expects.
func (g *group) quorum(n int, cfg Config) int {
	return min(n, max(g.act.minSize(cfg), min(g.target, n-cfg.ExtraInvites)))
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
	g.fixLoneGender(placed, rel, cfg)
}

// bestAddition returns the unplaced eligible person who, added to members,
// gives the best score without breaking a rule that adding more people
// can't fix. only, if set, restricts the choice (e.g. to one gender).
func (g *group) bestAddition(members []*User, placed map[UserID]bool, rel Relations, cfg Config, only func(*User) bool) *User {
	var best *User
	bestScore := -1.0
	for _, u := range g.eligible {
		if placed[u.ID] || slices.Contains(members, u) || (only != nil && !only(u)) {
			continue
		}
		with := append(slices.Clone(members), u)
		if monotoneViolation(with, rel, cfg) != nil {
			continue
		}
		if s := g.score(with, rel, cfg); s > bestScore {
			best, bestScore = u, s
		}
	}
	return best
}

// fixLoneGender resolves the one rule greedy growth can break temporarily:
// nobody may be the only woman or the only man. It adds a second person of
// that gender if there's room and someone suitable, else removes the lone
// person (who stays unplaced and may fit another group).
func (g *group) fixLoneGender(placed map[UserID]bool, rel Relations, cfg Config) {
	for {
		lone := loneGender(g.members)
		if lone == nil {
			return
		}
		if len(g.members) < g.act.MaxSize {
			if u := g.bestAddition(g.members, placed, rel, cfg, func(u *User) bool { return u.Gender == lone.Gender }); u != nil {
				g.members = append(g.members, u)
				continue
			}
		}
		g.members = slices.DeleteFunc(g.members, func(u *User) bool { return u == lone })
		if len(g.members) == 0 {
			return
		}
	}
}

// localSearch improves the greedy result. Moves, tried in a fixed order:
// fill a group that's below its invite cap with an unplaced person, replace a
// member with an unplaced person, or swap two members of different groups.
// A move is kept only if the total score rises and every rule still holds.
func localSearch(groups []*group, placed map[UserID]bool, rel Relations, cfg Config) {
	const eps = 1e-9
	value := func(g *group, ms []*User) float64 { return float64(len(ms)) * g.score(ms, rel, cfg) }
	valid := func(ms []*User) bool { return Violation(ms, rel, cfg) == nil }
	eligibleFor := func(g *group, u *User) bool { return slices.Contains(g.eligible, u) }

	for range cfg.SearchPasses {
		improved := false
		for _, g := range groups {
			for len(g.members) < g.cap {
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
			return
		}
	}
}

// score is the weighted sum of the group's parts, each between 0 and 1.
func (g *group) score(ms []*User, rel Relations, cfg Config) float64 {
	n := float64(len(ms))
	if n == 0 {
		return 0
	}
	var interestSum, fitSum, reliability float64
	newcomers := 0
	// Size fit compares preferences with the expected attendance, not the
	// number invited.
	expected := float64(g.quorum(len(ms), cfg))
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
	w := cfg.Weights
	return w.Interest*interestSum/n/cfg.PrefValues[Love] +
		w.SizeFit*fitSum/n +
		w.Mutual*min(1, mutual) +
		w.Newcomers*mix +
		w.Reliability*reliability/n
}

// Violation reports the first hard rule a group breaks, or nil. The worker
// (M6) re-checks it as people accept and decline.
//
//   - nobody is grouped with someone either of them blocked
//   - someone who asked for same-gender groups only gets exactly that
//   - nobody is the only woman or the only man (nonbinary and unspecified
//     are exempt: requiring two of them would leave most unmatched)
//   - at most cfg.MaxKnown people already know each other (a mutual pick)
func Violation(members []*User, rel Relations, cfg Config) error {
	if err := monotoneViolation(members, rel, cfg); err != nil {
		return err
	}
	if u := loneGender(members); u != nil {
		return fmt.Errorf("%s is the only %s", u.ID, u.Gender)
	}
	return nil
}

// monotoneViolation checks the rules that adding people can only break,
// never fix, so greedy growth can check them as it goes.
func monotoneViolation(members []*User, rel Relations, cfg Config) error {
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
