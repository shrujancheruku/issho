package matching

import (
	"cmp"
	"slices"
)

// SuggestInput is what Suggest needs, for one city and one window.
type SuggestInput struct {
	Users      []User
	Areas      []AreaID
	Travel     TravelTimes
	Activities []Activity // enabled and in season for the window
	Venues     Venues
	Open       []Plan // plans already open: not suggested again
}

// Suggestion is a plan worth opening for signups.
type Suggestion struct {
	Plan
	// Audience is how many people in the pool have a preference for the
	// activity and are usually free at a time it suits.
	Audience int
	// Score ranks suggestions: demand × feasibility, plus a share of the
	// follow-up's demand.
	Score float64
}

// Suggest picks plans to open for signups. Every pool's activities are
// ranked by demand × feasibility, then taken best first across the whole
// city, skipping:
//
//   - a plan already open, or more than cfg.PlansPerPool from one pool
//   - the same activity in a shared language within cfg.PlanSpacingMin
//     minutes of a plan already open or taken, so neighbouring areas don't
//     split the same people's signups (and a two-language plan doesn't
//     duplicate a one-language one)
//   - anything beyond one open plan per cfg.UsersPerPlan people, so signups
//     concentrate on few enough plans to fill
//
// An activity can happen in an area when it has a venue there (or needs
// only a meeting point). Demand adds up the pool's interest (cfg.PrefValues,
// scaled down for people whose budget is below its cost), counting only
// people whose profile availability includes a part of the day it suits.
// Fewer than cfg.VenueChoice venues scale the score down. A plan needs an
// audience of cfg.MinAudience: signups are a fraction of it. Two-language
// plans are discounted (cfg.BilingualWeight) so most plans are in a single
// language everyone shares.
func Suggest(in SuggestInput, cfg Config) []Suggestion {
	users := make(map[UserID]*User, len(in.Users))
	for i := range in.Users {
		users[in.Users[i].ID] = &in.Users[i]
	}
	var cands []Suggestion
	for _, pool := range Pools(in.Users, in.Areas, in.Travel, cfg) {
		for _, s := range rankActivities(pool, users, in.Activities, in.Venues, cfg) {
			if pool.Bilingual() {
				s.Score *= cfg.BilingualWeight
			}
			cands = append(cands, s)
		}
	}
	slices.SortStableFunc(cands, func(a, b Suggestion) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(boolInt(a.Secondary != ""), boolInt(b.Secondary != "")),
			cmp.Compare(a.Area, b.Area), cmp.Compare(a.Activity, b.Activity),
			cmp.Compare(a.Primary, b.Primary), cmp.Compare(a.Secondary, b.Secondary))
	})

	limit := (len(in.Users) + cfg.UsersPerPlan - 1) / cfg.UsersPerPlan
	taken := slices.Clone(in.Open)
	perPool := map[[3]string]int{}
	var out []Suggestion
	for _, s := range cands {
		if len(taken) >= limit {
			break
		}
		pool := [3]string{string(s.Area), s.Primary, s.Secondary}
		if perPool[pool] == cfg.PlansPerPool || slices.ContainsFunc(taken, func(p Plan) bool { return tooClose(p, s.Plan, in.Travel, cfg) }) {
			continue
		}
		perPool[pool]++
		taken = append(taken, s.Plan)
		out = append(out, s)
	}
	return out
}

// tooClose reports whether two plans are the same activity, share a
// language and are within cfg.PlanSpacingMin of each other (either way).
func tooClose(a, b Plan, travel TravelTimes, cfg Config) bool {
	if a.Activity != b.Activity {
		return false
	}
	shared := a.Primary == b.Primary || (b.Secondary != "" && a.Primary == b.Secondary) ||
		(a.Secondary != "" && (a.Secondary == b.Primary || a.Secondary == b.Secondary))
	if !shared {
		return false
	}
	ab, ok1 := travel.Minutes(a.Area, b.Area)
	ba, ok2 := travel.Minutes(b.Area, a.Area)
	return (ok1 && ab <= cfg.PlanSpacingMin) || (ok2 && ba <= cfg.PlanSpacingMin)
}

// rankActivities scores every activity that could happen in the pool's
// area and has enough audience, best first.
func rankActivities(pool Pool, users map[UserID]*User, activities []Activity, venues Venues, cfg Config) []Suggestion {
	type scored struct {
		act      *Activity
		demand   float64
		feasible float64
		audience int
	}
	var mains, followUps []scored
	for i := range activities {
		a := &activities[i]
		feasible := 1.0
		if len(a.VenueCategories) > 0 {
			n := venues.count(pool.Area, a)
			if n == 0 {
				continue
			}
			feasible = min(1, float64(n)/float64(cfg.VenueChoice))
		}
		s := scored{act: a, feasible: feasible}
		for _, id := range pool.Members {
			u := users[id]
			if !usuallyFree(u, a) {
				continue
			}
			if v, ok := interest(u, a, cfg); ok {
				s.demand += v
				s.audience++
			}
		}
		if s.audience < cfg.MinAudience {
			continue
		}
		mains = append(mains, s)
		if a.Conversational && len(a.VenueCategories) > 0 && slices.Contains(cfg.FollowUpCategories, a.Category) {
			followUps = append(followUps, s)
		}
	}
	slices.SortFunc(followUps, func(x, y scored) int {
		return cmp.Or(cmp.Compare(y.demand*y.feasible, x.demand*x.feasible), cmp.Compare(x.act.ID, y.act.ID))
	})

	out := make([]Suggestion, 0, len(mains))
	for _, m := range mains {
		s := Suggestion{
			Plan:     Plan{Area: pool.Area, Activity: m.act.ID, Primary: pool.Primary, Secondary: pool.Secondary},
			Audience: m.audience,
			Score:    m.demand * m.feasible,
		}
		if !m.act.Conversational {
			for _, f := range followUps {
				if f.act.ID != m.act.ID {
					s.FollowUp = f.act.ID
					s.Score += cfg.FollowUpWeight * f.demand * f.feasible
					break
				}
			}
		}
		out = append(out, s)
	}
	slices.SortFunc(out, func(x, y Suggestion) int {
		return cmp.Or(cmp.Compare(y.Score, x.Score), cmp.Compare(x.Activity, y.Activity))
	})
	return out
}

// usuallyFree reports whether the person's profile availability includes a
// part of the day the activity suits.
func usuallyFree(u *User, a *Activity) bool {
	for _, s := range u.Availability {
		if a.suits(s.Block) {
			return true
		}
	}
	return false
}

// interest is how much a person wants an activity, and whether they have a
// preference for it at all.
func interest(u *User, a *Activity, cfg Config) (float64, bool) {
	w, ok := u.Prefs[a.ID]
	if !ok || w < OpenToTry || w > Love {
		return 0, false
	}
	v := cfg.PrefValues[w]
	if a.CostLevel > u.Budget {
		v *= cfg.OverBudget
	}
	return v, true
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
