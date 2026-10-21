package matching

import (
	"cmp"
	"slices"
)

// Pool is the people who could join a plan in an area: each can reach Area
// within their maximum travel time and speaks the pool's language(s). Time
// isn't part of it: that's decided per plan, from its signups.
type Pool struct {
	Area AreaID // where the outing would happen
	// Primary is the event's language. In a single-language pool everyone
	// is fluent in it.
	Primary string
	// Secondary is set for two-language pools (Config.LanguagePairs):
	// everyone is fluent in Primary or Secondary. Primary is the language
	// more of them speak (ties: the lower code).
	Secondary string
	Members   []UserID // sorted
}

// Bilingual reports whether the pool is a two-language one.
func (p *Pool) Bilingual() bool { return p.Secondary != "" }

// Pools groups people by where and in which language they could meet.
//
// Every area is a candidate meeting place, and a person joins the pool of
// every area they can reach. So one person usually appears in several
// pools. Fixed area clusters would split neighbours who live either side of
// a boundary.
//
// Pools with fewer than cfg.MinGroupSize people are dropped. The result is
// sorted by area, then languages.
func Pools(users []User, areas []AreaID, travel TravelTimes, cfg Config) []Pool {
	type key struct {
		area   AreaID
		lang   string // a single language, or a pair's first
		second string // a pair's second language
	}
	members := map[key][]UserID{}
	byID := make(map[UserID]*User, len(users))

	for i := range users {
		u := &users[i]
		byID[u.ID] = u
		type lk struct{ lang, second string }
		var langs []lk
		for _, l := range u.Languages {
			if l.Fluent && !slices.Contains(langs, lk{lang: l.Code}) {
				langs = append(langs, lk{lang: l.Code})
			}
		}
		for _, p := range cfg.LanguagePairs {
			if u.speaks(p[0], p[1]) {
				langs = append(langs, lk{p[0], p[1]})
			}
		}
		for _, a := range areas {
			if !travel.reachable(u, a) {
				continue
			}
			for _, l := range langs {
				k := key{a, l.lang, l.second}
				members[k] = append(members[k], u.ID)
			}
		}
	}

	var out []Pool
	for k, ids := range members {
		if len(ids) < cfg.MinGroupSize {
			continue
		}
		p := Pool{Area: k.area, Primary: k.lang, Secondary: k.second, Members: ids}
		if k.second != "" {
			first, second := 0, 0
			for _, id := range ids {
				if byID[id].fluent(k.lang) {
					first++
				}
				if byID[id].fluent(k.second) {
					second++
				}
			}
			if second > first || (second == first && k.second < k.lang) {
				p.Primary, p.Secondary = k.second, k.lang
			}
		}
		slices.Sort(p.Members)
		out = append(out, p)
	}
	// Map iteration order is random; sort so output is deterministic.
	slices.SortFunc(out, func(a, b Pool) int {
		return cmp.Or(cmp.Compare(a.Area, b.Area), cmp.Compare(a.Primary, b.Primary), cmp.Compare(a.Secondary, b.Secondary))
	})
	return out
}
