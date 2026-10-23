// Package sim generates synthetic users for a city pack and runs the whole
// matching flow on them: suggest plans, let people sign up from their feeds,
// then schedule. It needs no database or Google calls: travel times are
// estimated from straight-line distance, venue counts are made up and
// signup behaviour is a guess, so results show the engine's behaviour, not
// real demand. Everything is seeded: a given seed always gives the same run.
package sim

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// Options control generation.
type Options struct {
	Users int
	Seed  uint64
	Month time.Month // for seasonal activities; 0 = ignore seasons
	Start time.Time  // first day of the plans' window (midnight UTC); zero = 2026-11-02
	Days  int        // length of the window; 0 = 14
}

// City is a generated city: the engine's inputs.
type City struct {
	Users      []matching.User
	Areas      []matching.AreaID
	Travel     matching.TravelTimes
	Activities []matching.Activity
	Venues     matching.Venues
	Relations  matching.Relations
}

// otherLanguages are spoken by a minority of people in any city, so buckets
// in less common languages get exercised too.
var otherLanguages = []string{"es", "zh", "ko", "fr", "pt"}

// Generate builds a synthetic city.
func Generate(pack cities.Pack, catalogue []cities.Activity, opt Options) City {
	r := rand.New(rand.NewPCG(opt.Seed, 1))
	in := City{
		Travel:    matching.TravelTimes{},
		Venues:    matching.Venues{},
		Relations: matching.Relations{Mutual: map[matching.Pair]float64{}, Blocked: map[matching.Pair]bool{}},
	}

	for _, a := range pack.Areas {
		in.Areas = append(in.Areas, matching.AreaID(a.ID))
	}
	for _, from := range pack.Areas {
		for _, to := range pack.Areas {
			if from.ID == to.ID {
				continue
			}
			if m, ok := travelMinutes(km(from, to), pack.TravelModes); ok {
				in.Travel[matching.AreaPair{From: matching.AreaID(from.ID), To: matching.AreaID(to.ID)}] = m
			}
		}
	}

	in.Activities = activities(pack, catalogue, opt.Month)
	categories := map[matching.CategoryID]bool{}
	for _, a := range in.Activities {
		for _, c := range a.VenueCategories {
			categories[c] = true
		}
	}
	for _, area := range in.Areas {
		in.Venues[area] = map[matching.CategoryID]int{}
		for _, c := range sortedKeys(categories) {
			if r.Float64() < 0.6 {
				in.Venues[area][c] = 1 + r.IntN(6)
			}
		}
	}

	// Popularity follows a long tail: a few activities are wanted by many.
	popularity := make([]float64, len(in.Activities))
	for i, rank := range r.Perm(len(in.Activities)) {
		popularity[i] = 1 / math.Pow(float64(rank+1), 0.8)
	}

	for i := range opt.Users {
		in.Users = append(in.Users, user(r, fmt.Sprintf("%s-%04d", pack.ID, i), pack, in.Areas, in.Activities, popularity))
	}
	for range opt.Users / 10 {
		a, b := in.Users[r.IntN(opt.Users)].ID, in.Users[r.IntN(opt.Users)].ID
		if a != b {
			in.Relations.Mutual[matching.MakePair(a, b)] = 0.3 + 0.7*r.Float64()
		}
	}
	for range opt.Users / 50 {
		a, b := in.Users[r.IntN(opt.Users)].ID, in.Users[r.IntN(opt.Users)].ID
		if a != b {
			in.Relations.Blocked[matching.MakePair(a, b)] = true
		}
	}
	return in
}

func user(r *rand.Rand, id string, pack cities.Pack, areas []matching.AreaID, acts []matching.Activity, popularity []float64) matching.User {
	u := matching.User{
		ID:           matching.UserID(id),
		HomeArea:     areas[r.IntN(len(areas))],
		MaxTravelMin: 15 + r.IntN(46),
		Gender:       pick(r, []matching.Gender{matching.Woman, matching.Man, matching.Nonbinary, matching.Unspecified}, 46, 46, 5, 3),
		Size:         pick(r, []matching.SizePref{matching.Small, matching.Medium, matching.Large}, 35, 45, 20),
		Budget:       pick(r, []int{0, 1, 2, 3}, 10, 35, 35, 20),
		Reliability:  0.6 + 0.4*r.Float64(),
		Prefs:        map[matching.ActivityID]int{},
	}
	switch u.Gender {
	case matching.Woman:
		u.SameGenderOnly = r.Float64() < 0.12
	case matching.Man:
		u.SameGenderOnly = r.Float64() < 0.03
	}
	if r.Float64() < 0.5 {
		u.EventsAttended = 1 + r.IntN(10)
	}

	// The city's main language for most, its second for some, another
	// language for a few; everyone speaks at least one.
	for i, p := range []float64{0.85, 0.4} {
		if i < len(pack.Languages) && r.Float64() < p {
			u.Languages = append(u.Languages, matching.Language{Code: pack.Languages[i], Fluent: true})
		}
	}
	if r.Float64() < 0.08 || len(u.Languages) == 0 {
		other := otherLanguages[r.IntN(len(otherLanguages))]
		if !slices.ContainsFunc(u.Languages, func(l matching.Language) bool { return l.Code == other }) {
			u.Languages = append(u.Languages, matching.Language{Code: other, Fluent: true})
		}
	}

	// Weekends are popular, weekday daytimes rare.
	for d := time.Sunday; d <= time.Saturday; d++ {
		weekend := d == time.Saturday || d == time.Sunday
		for _, b := range []matching.Block{matching.Morning, matching.Afternoon, matching.Evening} {
			p := 0.08
			switch {
			case weekend && b == matching.Afternoon:
				p = 0.6
			case weekend:
				p = 0.4
			case b == matching.Evening:
				p = 0.35
			}
			if r.Float64() < p {
				u.Availability = append(u.Availability, matching.Slot{Weekday: d, Block: b})
			}
		}
	}

	for range 8 + r.IntN(13) {
		a := acts[weighted(r, popularity)]
		u.Prefs[a.ID] = pick(r, []int{matching.OpenToTry, matching.Like, matching.Love}, 30, 50, 20)
	}
	return u
}

// activities converts the catalogue as enabled in the city and month.
func activities(pack cities.Pack, catalogue []cities.Activity, month time.Month) []matching.Activity {
	overrides := map[string]cities.ActivityOverride{}
	for _, o := range pack.ActivityOverrides {
		overrides[o.ActivityID] = o
	}
	var out []matching.Activity
	for _, a := range catalogue {
		o := overrides[a.ID]
		if o.Disabled || (month != 0 && len(o.Months) > 0 && !slices.Contains(o.Months, int16(month))) {
			continue
		}
		m := matching.Activity{
			ID: matching.ActivityID(a.ID), Category: a.Category, CostLevel: int(a.CostLevel),
			Conversational: a.Conversational, MinSize: int(a.MinGroupSize), MaxSize: int(a.MaxGroupSize),
		}
		for _, c := range a.VenueCategories {
			m.VenueCategories = append(m.VenueCategories, matching.CategoryID(c))
		}
		for _, b := range a.Blocks {
			m.Blocks = append(m.Blocks, matching.Block(b))
		}
		out = append(out, m)
	}
	return out
}

// travelMinutes estimates the fastest trip: walking at 12 min/km (only under
// 4 km, as tool build-matrix does) or driving at 3 min/km plus 8 minutes to
// get going and park.
func travelMinutes(km float64, modes []string) (int, bool) {
	best := math.Inf(1)
	if slices.Contains(modes, "walk") && km < 4 {
		best = 12 * km
	}
	if slices.Contains(modes, "drive") {
		best = min(best, 8+3*km)
	}
	if math.IsInf(best, 1) {
		return 0, false
	}
	return int(math.Ceil(best)), true
}

func km(a, b cities.Area) float64 {
	const earthKm = 6371
	rad := math.Pi / 180
	dLat, dLng := (b.Lat-a.Lat)*rad, (b.Lng-a.Lng)*rad
	h := math.Pow(math.Sin(dLat/2), 2) + math.Cos(a.Lat*rad)*math.Cos(b.Lat*rad)*math.Pow(math.Sin(dLng/2), 2)
	return 2 * earthKm * math.Asin(math.Sqrt(h))
}

func pick[T any](r *rand.Rand, values []T, weights ...float64) T {
	return values[weighted(r, weights)]
}

func weighted(r *rand.Rand, weights []float64) int {
	total := 0.0
	for _, w := range weights {
		total += w
	}
	x := r.Float64() * total
	for i, w := range weights {
		if x -= w; x < 0 {
			return i
		}
	}
	return len(weights) - 1
}

func sortedKeys[K cmp.Ordered](m map[K]bool) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// Signup behaviour: the chance of signing up for a plan by feed level
// (NotForYou can't), how far down the feed people scroll, and how many plans
// one person signs up for at most.
var (
	signupChance = map[matching.Level]float64{matching.Top: 0.35, matching.Good: 0.15, matching.Maybe: 0.05, matching.Low: 0.01}
	scrollDepth  = 25
	maxSignups   = 3
)

// Stats summarise one run.
type Stats struct {
	Users, Suggestions, Signups, SignedUp int // SignedUp: people with at least one signup
	SignupsBy                             map[matching.Level]int
	Ready, Forced, Unscheduled            int               // plans: scheduled when ready, at the deadline, not at all
	Groups, Invited                       int               // Invited: distinct people
	Sizes                                 map[int]int       // people invited → groups
	Pairs, LoneGender                     int               // groups of 2; groups with an only woman or only man
	InvitedBy                             map[string][2]int // "gender:woman", "lang:ja" → invited, signed up
	TopActivities                         []string
	Violations                            []string // should always be empty
	Duration                              time.Duration
}

// Run generates a city and runs suggest → signups → schedule on it. Plans
// are scheduled most-signups first; each scheduled event blocks its
// occasion in its people's other signups. Plans not ready are then
// scheduled at their deadline (force) if any group can form.
func Run(pack cities.Pack, catalogue []cities.Activity, opt Options, cfg matching.Config) Stats {
	if opt.Start.IsZero() {
		opt.Start = time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	}
	if opt.Days == 0 {
		opt.Days = 14
	}
	city := Generate(pack, catalogue, opt)
	// A second stream for behaviour, so changing it doesn't change the city.
	r := rand.New(rand.NewPCG(opt.Seed, 2))
	window := matching.Occasions(opt.Start, opt.Days)
	start := time.Now()

	users := map[matching.UserID]*matching.User{}
	for i := range city.Users {
		users[city.Users[i].ID] = &city.Users[i]
	}
	acts := map[matching.ActivityID]*matching.Activity{}
	for i := range city.Activities {
		acts[city.Activities[i].ID] = &city.Activities[i]
	}
	suggestions := matching.Suggest(matching.SuggestInput{
		Users: city.Users, Areas: city.Areas, Travel: city.Travel, Activities: city.Activities, Venues: city.Venues,
	}, cfg)
	open := make([]matching.OpenPlan, len(suggestions))
	index := map[matching.Plan]int{}
	for i, s := range suggestions {
		open[i] = matching.OpenPlan{Plan: s.Plan}
		index[s.Plan] = i
	}

	st := Stats{Users: len(city.Users), Suggestions: len(open), SignupsBy: map[matching.Level]int{},
		Sizes: map[int]int{}, InvitedBy: map[string][2]int{}}
	signups := make([][]matching.Signup, len(open))
	// People arrive one by one, so later arrivals see earlier signups as
	// momentum.
	for _, i := range r.Perm(len(city.Users)) {
		u := &city.Users[i]
		n := 0
		for _, item := range matching.Feed(u, open, acts, city.Travel, cfg, nil)[:min(scrollDepth, len(open))] {
			if n == maxSignups || r.Float64() >= signupChance[item.Level] {
				continue
			}
			p := index[item.Plan]
			signups[p] = append(signups[p], signup(r, u, window))
			open[p].Signups++
			st.SignupsBy[item.Level]++
			n++
		}
		st.Signups += n
		if n > 0 {
			st.SignedUp++
		}
	}

	order := make([]int, len(open))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(open[b].Signups, open[a].Signups) })
	busy := map[matching.UserID][]matching.Occasion{}
	invited := map[matching.UserID]bool{}
	acted := map[string]int{}
	done := make([]bool, len(open))
	for _, force := range []bool{false, true} {
		for _, p := range order {
			if done[p] || len(signups[p]) == 0 {
				continue
			}
			ss := make([]matching.Signup, len(signups[p]))
			for i, s := range signups[p] {
				s.Unavailable = slices.Concat(s.Unavailable, busy[s.User])
				ss[i] = s
			}
			got, ok := matching.Schedule(matching.ScheduleInput{
				Plan: open[p].Plan, Activity: acts[open[p].Activity], Signups: ss, Users: users,
				Relations: city.Relations, Window: window,
			}, cfg, force)
			if !ok {
				continue
			}
			done[p] = true
			if force {
				st.Forced++
			} else {
				st.Ready++
			}
			acted[string(open[p].Activity)]++
			st.check(got, ss, users, city.Relations, cfg)
			for _, g := range got.Groups {
				for _, id := range g.Invited {
					busy[id] = append(busy[id], got.Occasion)
					invited[id] = true
				}
			}
		}
	}
	st.Unscheduled = len(open) - st.Ready - st.Forced
	st.Invited = len(invited)

	signedUp := map[matching.UserID]bool{}
	for _, ss := range signups {
		for _, s := range ss {
			signedUp[s.User] = true
		}
	}
	for _, u := range city.Users {
		if !signedUp[u.ID] {
			continue
		}
		keys := []string{"gender:" + string(u.Gender)}
		for _, l := range u.Languages {
			keys = append(keys, "lang:"+l.Code)
		}
		if u.SameGenderOnly {
			keys = append(keys, "same-gender-only")
		}
		for _, k := range keys {
			c := st.InvitedBy[k]
			if invited[u.ID] {
				c[0]++
			}
			c[1]++
			st.InvitedBy[k] = c
		}
	}
	ids := slices.Collect(maps.Keys(acted))
	slices.SortFunc(ids, func(a, b string) int { return cmp.Or(cmp.Compare(acted[b], acted[a]), cmp.Compare(a, b)) })
	st.TopActivities = ids[:min(len(ids), 10)]
	st.Duration = time.Since(start)
	return st
}

// signup makes one person's signup: usually their profile availability;
// sometimes a preset for this plan ("weekday evenings", "weekend
// mornings"), or the profile minus one slot ("not Wednesday evenings");
// sometimes with one date blocked out.
func signup(r *rand.Rand, u *matching.User, window []matching.Occasion) matching.Signup {
	s := matching.Signup{User: u.ID}
	switch x := r.Float64(); {
	case x < 0.25:
		presets := [][]matching.Slot{weekdays(matching.Evening), weekend(matching.Morning), weekend(matching.Afternoon), weekend(matching.Evening)}
		for _, i := range r.Perm(len(presets))[:1+r.IntN(2)] {
			s.Pattern = append(s.Pattern, presets[i]...)
		}
	case x < 0.4 && len(u.Availability) > 1:
		drop := r.IntN(len(u.Availability))
		s.Pattern = slices.Delete(slices.Clone(u.Availability), drop, drop+1)
	}
	if r.Float64() < 0.25 {
		s.Unavailable = []matching.Occasion{{Date: window[r.IntN(len(window))].Date}}
	}
	return s
}

func weekdays(b matching.Block) []matching.Slot {
	var out []matching.Slot
	for d := time.Monday; d <= time.Friday; d++ {
		out = append(out, matching.Slot{Weekday: d, Block: b})
	}
	return out
}

func weekend(b matching.Block) []matching.Slot {
	return []matching.Slot{{Weekday: time.Saturday, Block: b}, {Weekday: time.Sunday, Block: b}}
}

// check verifies a scheduled plan: hard rules, nobody twice, everyone free.
// It also counts sizes.
func (st *Stats) check(got matching.Scheduled, ss []matching.Signup, users map[matching.UserID]*matching.User, rel matching.Relations, cfg matching.Config) {
	byUser := map[matching.UserID]*matching.Signup{}
	for i := range ss {
		byUser[ss[i].User] = &ss[i]
	}
	seen := map[matching.UserID]bool{}
	for _, g := range got.Groups {
		st.Groups++
		st.Sizes[len(g.Invited)]++
		if len(g.Invited) == 2 {
			st.Pairs++
		}
		var ms []*matching.User
		counts := map[matching.Gender]int{}
		for _, id := range g.Invited {
			if seen[id] {
				st.Violations = append(st.Violations, fmt.Sprintf("%s placed twice", id))
			}
			seen[id] = true
			s := byUser[id]
			if s == nil || !s.Available(users[id], got.Occasion) {
				st.Violations = append(st.Violations, fmt.Sprintf("%s invited when not free (or double-booked)", id))
			}
			ms = append(ms, users[id])
			counts[users[id].Gender]++
		}
		if counts[matching.Woman] == 1 || counts[matching.Man] == 1 {
			st.LoneGender++
		}
		if err := matching.Violation(ms, rel, cfg); err != nil {
			st.Violations = append(st.Violations, err.Error())
		}
	}
}

// String formats the stats as a short report.
func (s Stats) String() string {
	var b strings.Builder
	rate := func(n, of int) string {
		return fmt.Sprintf("%d/%d (%.0f%%)", n, of, 100*float64(n)/float64(max(of, 1)))
	}
	fmt.Fprintf(&b, "users %d · plans suggested %d · signups %d from %s people (top %d, good %d, maybe %d, low %d) · %v\n",
		s.Users, s.Suggestions, s.Signups, rate(s.SignedUp, s.Users),
		s.SignupsBy[matching.Top], s.SignupsBy[matching.Good], s.SignupsBy[matching.Maybe], s.SignupsBy[matching.Low],
		s.Duration.Round(time.Millisecond))
	fmt.Fprintf(&b, "plans scheduled when ready %d, at the deadline %d, not at all %d\n", s.Ready, s.Forced, s.Unscheduled)
	fmt.Fprintf(&b, "groups %d · invited %s of people who signed up · pairs %d · with an only woman or only man %d\n",
		s.Groups, rate(s.Invited, s.SignedUp), s.Pairs, s.LoneGender)
	var sizes []string
	for _, n := range slices.Sorted(maps.Keys(s.Sizes)) {
		sizes = append(sizes, fmt.Sprintf("%d×%d", n, s.Sizes[n]))
	}
	fmt.Fprintf(&b, "invited per group: %s\n", strings.Join(sizes, " "))
	fmt.Fprintf(&b, "top activities: %s\n", strings.Join(s.TopActivities, ", "))
	for _, k := range slices.Sorted(maps.Keys(s.InvitedBy)) {
		c := s.InvitedBy[k]
		fmt.Fprintf(&b, "  invited of signed up, %-22s %s\n", k, rate(c[0], c[1]))
	}
	fmt.Fprintf(&b, "rule violations: %d\n", len(s.Violations))
	for _, v := range s.Violations {
		fmt.Fprintf(&b, "  %s\n", v)
	}
	return b.String()
}
