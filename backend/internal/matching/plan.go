package matching

import (
	"cmp"
	"slices"
)

// Plan is a candidate outing for one bucket.
type Plan struct {
	Activity ActivityID
	// FollowUp is an optional second activity, such as a meal after
	// bouldering, so a group whose main activity leaves little room to talk
	// still gets to know each other. "" = none.
	FollowUp ActivityID
	// Interested is how many of the bucket's members have a preference for
	// Activity: the most people a group for this plan could have.
	Interested int
	// Score ranks plans: demand × feasibility, plus a share of the
	// follow-up's demand.
	Score float64
}

// Plans runs step 2 for one bucket: activities that could happen there and
// then, ranked by demand × feasibility. It keeps the best
// cfg.PlansPerBucket.
//
// An activity is feasible when it suits the slot's part of the day, the area
// has a venue for it (or it needs only a meeting point), and enough members
// are interested to form a group (cfg.MinGroupSize; the activity's own
// minimum is a preference, applied when groups are scored). Demand adds up
// members' interest (cfg.PrefValues),
// scaled down for those whose budget is below its cost. Feasibility scales
// the score down when the area has fewer than cfg.VenueChoice venues for it.
//
// Opening hours aren't known here (Google's terms forbid storing them); the
// event-creation step checks the chosen venue live.
func Plans(b Bucket, users map[UserID]*User, activities []Activity, venues Venues, cfg Config) []Plan {
	members := make([]*User, 0, len(b.Members))
	for _, id := range b.Members {
		members = append(members, users[id])
	}
	type scored struct {
		act        *Activity
		demand     float64
		feasible   float64
		interested int
	}
	var mains, followUps []scored
	for i := range activities {
		a := &activities[i]
		if !a.suits(b.Slot.Block) {
			continue
		}
		feasible := 1.0
		if len(a.VenueCategories) > 0 {
			n := venues.count(b.Area, a)
			if n == 0 {
				continue
			}
			feasible = min(1, float64(n)/float64(cfg.VenueChoice))
		}
		s := scored{act: a, feasible: feasible}
		for _, u := range members {
			if v, ok := interest(u, a, cfg); ok {
				s.demand += v
				s.interested++
			}
		}
		if s.interested < cfg.MinGroupSize {
			continue
		}
		mains = append(mains, s)
		if a.Conversational && len(a.VenueCategories) > 0 && slices.Contains(cfg.FollowUpCategories, a.Category) {
			followUps = append(followUps, s)
		}
	}
	byScore := func(x, y scored) int {
		return cmp.Or(cmp.Compare(y.demand*y.feasible, x.demand*x.feasible), cmp.Compare(x.act.ID, y.act.ID))
	}
	slices.SortFunc(followUps, byScore)

	plans := make([]Plan, 0, len(mains))
	for _, m := range mains {
		p := Plan{Activity: m.act.ID, Interested: m.interested, Score: m.demand * m.feasible}
		if !m.act.Conversational {
			for _, f := range followUps {
				if f.act.ID != m.act.ID {
					p.FollowUp = f.act.ID
					p.Score += cfg.FollowUpWeight * f.demand * f.feasible
					break
				}
			}
		}
		plans = append(plans, p)
	}
	slices.SortFunc(plans, func(x, y Plan) int {
		return cmp.Or(cmp.Compare(y.Score, x.Score), cmp.Compare(x.Activity, y.Activity))
	})
	return plans[:min(len(plans), cfg.PlansPerBucket)]
}

// interest is how much a person wants an activity, and whether they have a
// preference for it at all. People without one aren't grouped for it.
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
