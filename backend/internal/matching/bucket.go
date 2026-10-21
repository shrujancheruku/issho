package matching

import (
	"cmp"
	"slices"
)

// Bucket is a pool of people who could meet: each can reach Area within
// their maximum travel time, is free in Slot, and can talk in the bucket's
// language(s). Groups are later assembled from within one bucket.
type Bucket struct {
	Area AreaID // where the outing would happen
	Slot Slot
	// Primary is the event's language. In a single-language bucket everyone
	// is fluent in it.
	Primary string
	// Secondary is set for two-language buckets (Config.LanguagePairs):
	// everyone is fluent in Primary or Secondary. Primary is the language
	// more of them speak (ties: the lower code).
	Secondary string
	Members   []UserID // sorted
}

// Bilingual reports whether the bucket is a two-language one.
func (b *Bucket) Bilingual() bool { return b.Secondary != "" }

// Buckets runs step 1: it groups people by where, when and in which
// language they could meet.
//
// Every area is a candidate meeting place, and a person joins the bucket of
// every area they can reach. So one person usually appears in several
// buckets; group assembly places each person at most once. Fixed area
// clusters would split neighbours who live either side of a boundary.
//
// Buckets with fewer than cfg.MinBucketSize people are dropped. The result
// is sorted by slot, area, then languages.
func Buckets(users []User, areas []AreaID, travel TravelTimes, cfg Config) []Bucket {
	type key struct {
		area   AreaID
		slot   Slot
		lang   string // a single language, or a pair's first
		second string // a pair's second language
	}
	pools := map[key][]UserID{}
	// Per two-language bucket, how many speak the pair's first language, to
	// decide which becomes the event's primary language.
	firstSpeakers := map[key]int{}
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
			if u.fluent(p[0]) || u.fluent(p[1]) {
				langs = append(langs, lk{p[0], p[1]})
			}
		}
		slots := slices.Clone(u.Availability)
		slices.SortFunc(slots, compareSlots)
		slots = slices.Compact(slots)
		for _, a := range areas {
			if m, ok := travel.Minutes(u.HomeArea, a); !ok || m > u.MaxTravelMin {
				continue
			}
			for _, s := range slots {
				for _, l := range langs {
					k := key{a, s, l.lang, l.second}
					pools[k] = append(pools[k], u.ID)
					if l.second != "" && u.fluent(l.lang) {
						firstSpeakers[k]++
					}
				}
			}
		}
	}

	var out []Bucket
	for k, members := range pools {
		if len(members) < cfg.MinBucketSize {
			continue
		}
		b := Bucket{Area: k.area, Slot: k.slot, Primary: k.lang, Secondary: k.second, Members: members}
		if k.second != "" {
			// Count the second language's speakers the same way (people
			// fluent in both count for both).
			second := 0
			for _, id := range members {
				if byID[id].fluent(k.second) {
					second++
				}
			}
			if second > firstSpeakers[k] || (second == firstSpeakers[k] && k.second < k.lang) {
				b.Primary, b.Secondary = k.second, k.lang
			}
		}
		slices.Sort(b.Members)
		out = append(out, b)
	}
	// Map iteration order is random; sort so output is deterministic.
	slices.SortFunc(out, func(a, b Bucket) int {
		return cmp.Or(
			compareSlots(a.Slot, b.Slot),
			cmp.Compare(a.Area, b.Area),
			cmp.Compare(a.Primary, b.Primary),
			cmp.Compare(a.Secondary, b.Secondary),
		)
	})
	return out
}
