// Package sim generates synthetic users for a city pack and measures what the
// matching engine does with them. It needs no database or Google calls:
// travel times are estimated from straight-line distance and venue counts
// are made up, so results show the engine's behaviour, not real demand.
// Generation is seeded, so a given seed always produces the same city.
package sim

import (
	"cmp"
	"fmt"
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
}

// otherLanguages are spoken by a minority of people in any city, so buckets
// in less common languages get exercised too.
var otherLanguages = []string{"es", "zh", "ko", "fr", "pt"}

// Generate builds a matching input for one city.
func Generate(pack cities.Pack, catalogue []cities.Activity, opt Options) matching.Input {
	r := rand.New(rand.NewPCG(opt.Seed, 1))
	in := matching.Input{
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

// Stats summarise one run.
type Stats struct {
	Users, Proposals, Invited int
	Quorum                    int               // the proposals' quorums added up
	Bilingual                 int               // proposals with two languages
	Sizes                     map[int]int       // people invited → proposals
	PlacedBy                  map[string][2]int // "gender:woman", "lang:ja" → invited, total
	TopActivities             []string
	Violations                []string // should always be empty
	Duration                  time.Duration
}

// Run generates a city, runs the engine and measures the result.
func Run(pack cities.Pack, catalogue []cities.Activity, opt Options, cfg matching.Config) Stats {
	in := Generate(pack, catalogue, opt)
	start := time.Now()
	ps := matching.Run(in, cfg)
	st := Measure(in, ps, cfg)
	st.Duration = time.Since(start)
	return st
}

// Measure checks the proposals against the input: every hard rule holds and
// nobody is placed twice. It also counts who got placed.
func Measure(in matching.Input, ps []matching.Proposal, cfg matching.Config) Stats {
	st := Stats{Users: len(in.Users), Proposals: len(ps), Sizes: map[int]int{}, PlacedBy: map[string][2]int{}}
	byID := map[matching.UserID]*matching.User{}
	for i := range in.Users {
		byID[in.Users[i].ID] = &in.Users[i]
	}
	placed := map[matching.UserID]bool{}
	acts := map[matching.ActivityID]int{}
	for _, p := range ps {
		st.Sizes[len(p.Invited)]++
		st.Invited += len(p.Invited)
		st.Quorum += p.Quorum
		acts[p.Plan.Activity]++
		if p.Secondary != "" {
			st.Bilingual++
		}
		var group []*matching.User
		for _, id := range p.Invited {
			if placed[id] {
				st.Violations = append(st.Violations, fmt.Sprintf("%s placed twice", id))
			}
			placed[id] = true
			group = append(group, byID[id])
		}
		if err := matching.Violation(group, in.Relations, cfg); err != nil {
			st.Violations = append(st.Violations, err.Error())
		}
	}
	for _, u := range in.Users {
		keys := []string{"gender:" + string(u.Gender)}
		for _, l := range u.Languages {
			keys = append(keys, "lang:"+l.Code)
		}
		if u.SameGenderOnly {
			keys = append(keys, "same-gender-only")
		}
		for _, k := range keys {
			c := st.PlacedBy[k]
			if placed[u.ID] {
				c[0]++
			}
			c[1]++
			st.PlacedBy[k] = c
		}
	}
	ids := make([]string, 0, len(acts))
	for a := range acts {
		ids = append(ids, string(a))
	}
	slices.SortFunc(ids, func(a, b string) int {
		return cmp.Or(cmp.Compare(acts[matching.ActivityID(b)], acts[matching.ActivityID(a)]), cmp.Compare(a, b))
	})
	st.TopActivities = ids[:min(len(ids), 10)]
	return st
}

// String formats the stats as a short report.
func (s Stats) String() string {
	var b strings.Builder
	rate := func(n, of int) string {
		return fmt.Sprintf("%d/%d (%.0f%%)", n, of, 100*float64(n)/float64(max(of, 1)))
	}
	fmt.Fprintf(&b, "users %d · proposals %d (%d two-language) · invited %s · seats needed for quorum %d · %v\n",
		s.Users, s.Proposals, s.Bilingual, rate(s.Invited, s.Users), s.Quorum, s.Duration.Round(time.Millisecond))
	var sizes []string
	for _, n := range sortedKeys(boolKeys(s.Sizes)) {
		sizes = append(sizes, fmt.Sprintf("%d×%d", n, s.Sizes[n]))
	}
	fmt.Fprintf(&b, "invited per proposal: %s\n", strings.Join(sizes, " "))
	fmt.Fprintf(&b, "top activities: %s\n", strings.Join(s.TopActivities, ", "))
	keys := make([]string, 0, len(s.PlacedBy))
	for k := range s.PlacedBy {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		c := s.PlacedBy[k]
		fmt.Fprintf(&b, "  invited, %-22s %s\n", k, rate(c[0], c[1]))
	}
	fmt.Fprintf(&b, "rule violations: %d\n", len(s.Violations))
	for _, v := range s.Violations {
		fmt.Fprintf(&b, "  %s\n", v)
	}
	return b.String()
}

func boolKeys(m map[int]int) map[int]bool {
	out := map[int]bool{}
	for k := range m {
		out[k] = true
	}
	return out
}
