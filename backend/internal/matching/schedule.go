package matching

import (
	"cmp"
	"slices"
)

// ScheduleInput is one open plan with its signups.
type ScheduleInput struct {
	Plan     Plan
	Activity *Activity
	Signups  []Signup
	Users    map[UserID]*User // at least everyone who signed up
	// Relations between the people who signed up (mutual picks, blocks).
	Relations Relations
	// Window is when the plan can happen, e.g. Occasions(start, 14). Parts
	// of the day the activity doesn't suit are skipped.
	Window []Occasion
}

// Scheduled is a plan given a time and groups.
type Scheduled struct {
	Occasion Occasion
	Groups   []Group
	// Waiting signed up but weren't placed (not free then, or no group had
	// room): they stay signed up for the plan's next round.
	Waiting []UserID
	// Ineligible signed up without speaking the plan's language(s); the app
	// shouldn't allow it, so they're ignored.
	Ineligible []UserID
}

// Schedule decides whether a plan is ready and, if so, when it happens and
// who goes together.
//
// For each occasion in the window, it forms groups from the signups free
// then, and keeps the occasion with the best total (each group's score × its
// size; ties go to the earlier one). The plan is ready when, on that
// occasion, at least one group reaches its target size: until then it keeps
// collecting signups, and groups below the preferred size wait for the next
// round. force (e.g. at the plan's signup deadline) schedules the best
// occasion even when no group reaches its target, keeping every group that
// can form at all.
func Schedule(in ScheduleInput, cfg Config, force bool) (Scheduled, bool) {
	var eligible []*Signup
	var out Scheduled
	for i := range in.Signups {
		s := &in.Signups[i]
		if in.Users[s.User].speaks(in.Plan.Primary, in.Plan.Secondary) {
			eligible = append(eligible, s)
		} else {
			out.Ineligible = append(out.Ineligible, s.User)
		}
	}
	interestOf := func(u *User) float64 {
		// Signing up says they want it, so no preference counts as a like.
		if v, ok := interest(u, in.Activity, cfg); ok {
			return v
		}
		v := cfg.PrefValues[Like]
		if in.Activity.CostLevel > u.Budget {
			v *= cfg.OverBudget
		}
		return v
	}

	window := slices.Clone(in.Window)
	slices.SortFunc(window, compareOccasions)
	var best []*group
	bestValue := -1.0
	found := false
	for _, o := range window {
		if !in.Activity.suits(o.Block) {
			continue
		}
		var free []*User
		for _, s := range eligible {
			if u := in.Users[s.User]; s.Available(u, o) {
				free = append(free, u)
			}
		}
		slices.SortFunc(free, func(a, b *User) int { return cmp.Compare(a.ID, b.ID) })
		if len(free) < cfg.MinGroupSize {
			continue
		}
		groups := assemble(free, in.Activity, interestOf, in.Relations, cfg)
		if !force {
			// Remnants below the preferred size wait for the next round,
			// when more signups may make them a proper group.
			groups = slices.DeleteFunc(groups, func(g *group) bool { return len(g.members) < in.Activity.preferredMin(cfg) })
		}
		ready := force && len(groups) > 0
		value := 0.0
		for _, g := range groups {
			value += float64(len(g.members)) * g.score(g.members, in.Relations, cfg)
			ready = ready || len(g.members) >= g.target
		}
		if ready && value > bestValue {
			out.Occasion, best, bestValue, found = o, groups, value, true
		}
	}
	if !found {
		return Scheduled{Ineligible: out.Ineligible}, false
	}

	placed := map[UserID]bool{}
	for _, g := range best {
		out.Groups = append(out.Groups, g.result(in.Relations, cfg))
		for _, u := range g.members {
			placed[u.ID] = true
		}
	}
	for _, s := range eligible {
		if !placed[s.User] {
			out.Waiting = append(out.Waiting, s.User)
		}
	}
	slices.Sort(out.Waiting)
	slices.Sort(out.Ineligible)
	return out, true
}
