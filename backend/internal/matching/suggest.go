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
	// Skip are activities not to suggest in an area this time, in any
	// language (a walk that couldn't be composed there): only Area and
	// Activity are compared. Unlike Open they don't count towards the
	// city's cap or the spacing between plans, so the next best plan takes
	// their place.
	Skip []Plan
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
// only a meeting point). A walk's venue categories, if any, only say the
// area has places on its theme; whether a good walk exists there is decided
// when it's composed, and a walk gets no follow-up. Demand adds up the pool's interest (cfg.PrefValues,
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
			if slices.ContainsFunc(in.Skip, func(p Plan) bool { return p.Area == s.Area && p.Activity == s.Activity }) {
				continue
			}
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

// tooClose reports whether two plans share an activity (in either place:
// "bouldering, then lunch" and "museum, then lunch" draw on the same
// people), share a language and are within cfg.PlanSpacingMin of each other
// (either way).
func tooClose(a, b Plan, travel TravelTimes, cfg Config) bool {
	shares := a.Activity == b.Activity ||
		(b.FollowUp != "" && (a.Activity == b.FollowUp || a.FollowUp == b.FollowUp)) ||
		(a.FollowUp != "" && a.FollowUp == b.Activity)
	if !shares {
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

// rankActivities scores the plans that could happen in the pool's area and
// have enough audience, best first. Every activity is weighed alone and
// followed by each of the pool's most wanted others (cfg.PairCandidates),
// and keeps whichever scores higher: two activities aren't required, so a
// full outing of one (a long walk) can beat two shorter halves.
//
// A plan's score is its audience's interest (appeal: for two activities,
// each person's interest in both, weighted by how long each takes) × how
// much of an outing it fills (cfg.OutingHours) × how many venues it can
// choose from, with cfg.PairBonus more for two activities.
func rankActivities(pool Pool, users map[UserID]*User, activities []Activity, venues Venues, cfg Config) []Suggestion {
	type option struct {
		act      *Activity
		feasible float64
		values   []float64 // per pool member: their interest when usually free for it, else 0
		demand   float64
	}
	var opts []*option
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
		o := &option{act: a, feasible: feasible, values: make([]float64, len(pool.Members))}
		for j, id := range pool.Members {
			u := users[id]
			if !usuallyFree(u, a) {
				continue
			}
			if v, ok := interest(u, a, cfg); ok {
				o.values[j] = v
				o.demand += v
			}
		}
		opts = append(opts, o)
	}

	// Second activities: the most wanted in the pool that need a venue (the
	// second stop is a place) and aren't walks (a walk is a whole outing).
	var seconds []*option
	for _, o := range opts {
		if !o.act.Walk && len(o.act.VenueCategories) > 0 && o.demand > 0 {
			seconds = append(seconds, o)
		}
	}
	slices.SortFunc(seconds, func(x, y *option) int {
		return cmp.Or(cmp.Compare(y.demand*y.feasible, x.demand*x.feasible), cmp.Compare(x.act.ID, y.act.ID))
	})
	seconds = seconds[:min(len(seconds), cfg.PairCandidates)]

	evaluate := func(a, b *option) Suggestion {
		s := Suggestion{Plan: Plan{Area: pool.Area, Activity: a.act.ID, Primary: pool.Primary, Secondary: pool.Secondary}}
		var demand float64
		for j := range pool.Members {
			v := a.values[j]
			if b != nil {
				v = appeal(a.act, b.act, v, b.values[j], cfg)
			}
			if v > 0 {
				demand += v
				s.Audience++
			}
		}
		hours, feasible := a.act.hours(), a.feasible
		if b != nil {
			s.FollowUp = b.act.ID
			hours += b.act.hours()
			feasible *= b.feasible
			demand *= 1 + cfg.PairBonus
		}
		s.Score = demand * feasible * min(1, hours/cfg.OutingHours)
		return s
	}

	var out []Suggestion
	for _, a := range opts {
		best := evaluate(a, nil)
		if !a.act.Walk && (!cfg.PairShortOnly || a.act.hours() < cfg.OutingHours) {
			for _, b := range seconds {
				if pairs(a.act, b.act, cfg) {
					if s := evaluate(a, b); s.Score > best.Score {
						best = s
					}
				}
			}
		}
		if best.Audience < cfg.MinAudience {
			continue
		}
		out = append(out, best)
	}
	slices.SortFunc(out, func(x, y Suggestion) int {
		return cmp.Or(cmp.Compare(y.Score, x.Score), cmp.Compare(x.Activity, y.Activity))
	})
	return out
}

// appeal is a person's interest in a plan of two activities given their
// interest in each (0 = no preference): the average, weighted by how long
// each takes, with cfg.NeutralInterest for one they have no preference for.
// So two activities appeal most to people who like both, and someone who
// likes only one finds the pair less appealing than it alone. 0 when they
// like neither.
func appeal(a, b *Activity, va, vb float64, cfg Config) float64 {
	if va == 0 && vb == 0 {
		return 0
	}
	if va == 0 {
		va = cfg.NeutralInterest
	}
	if vb == 0 {
		vb = cfg.NeutralInterest
	}
	ha, hb := a.hours(), b.hours()
	return (ha*va + hb*vb) / (ha + hb)
}

// pairs reports whether b can follow a in one outing: different kinds of
// activity, together no longer than cfg.MaxOutingHours, and in a natural
// order (a meal last; otherwise the more active first, then the less
// social, so the group winds down and talks over the second; ties by id, so
// each pair is one plan).
func pairs(a, b *Activity, cfg Config) bool {
	if a.ID == b.ID || a.Walk || b.Walk || a.Category == b.Category || a.hours()+b.hours() > cfg.MaxOutingHours {
		return false
	}
	aFood, bFood := a.Category == "food", b.Category == "food"
	if aFood != bFood {
		return bFood
	}
	if a.Energy != b.Energy {
		return a.Energy > b.Energy
	}
	if a.Conversational != b.Conversational {
		return b.Conversational
	}
	return a.ID < b.ID
}

// PlanInterest is a person's interest in a plan (appeal, for two
// activities), their interest in the activity of it they like most, and
// their stronger preference; ok is false when they have none for either.
func PlanInterest(u *User, p Plan, acts map[ActivityID]*Activity, cfg Config) (v, best float64, pref int, ok bool) {
	a := acts[p.Activity]
	v, ok = interest(u, a, cfg)
	best, pref = v, u.Prefs[a.ID]
	if b := acts[p.FollowUp]; b != nil {
		vb, okb := interest(u, b, cfg)
		v = appeal(a, b, v, vb, cfg)
		best = max(best, vb)
		if okb && (!ok || u.Prefs[b.ID] > pref) {
			pref = u.Prefs[b.ID]
		}
		ok = ok || okb
	}
	return v, best, pref, ok
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
