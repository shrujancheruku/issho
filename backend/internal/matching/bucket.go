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
	// Primary is the event's language. In a regular bucket everyone is
	// fluent in it.
	Primary string
	// Secondary is set only for language exchanges: everyone is fluent in
	// one of Primary/Secondary and learning the other. Primary is the side
	// with more people (ties: the lower code), since most of the group can
	// speak it comfortably.
	Secondary string
	Members   []UserID // sorted
}

// IsExchange reports whether the bucket is a language exchange.
func (b Bucket) IsExchange() bool { return b.Secondary != "" }

// Buckets runs step 1: it groups people by where, when and in which
// language they could meet.
//
// Every area is a candidate meeting place, and a person joins the bucket of
// every area they can reach. So one person usually appears in several
// buckets; group assembly places each person at most once. Fixed area
// clusters would split neighbours who live either side of a boundary.
//
// Exchange buckets are made for every pair of languages where some people
// are learning one of them. People fluent in both languages of a pair stay
// out of its exchange: they have nothing to practise.
//
// Buckets with fewer than cfg.MinBucketSize people, or exchanges with fewer
// than cfg.ExchangeMinPerSide on either side, are dropped. The result is
// sorted by slot, area, then languages.
func Buckets(users []User, areas []AreaID, travel TravelTimes, cfg Config) []Bucket {
	type key struct {
		area         AreaID
		slot         Slot
		lang         string // regular bucket's language, or the exchange's lower code
		exchangeWith string // the exchange's higher code
	}
	// Exchange members are kept per side (index 0 = fluent in key.lang) so
	// both the side minimum and the primary language can be decided.
	pools := map[key][2][]UserID{}

	for _, u := range users {
		var reachable []AreaID
		for _, a := range areas {
			if m, ok := travel.Minutes(u.HomeArea, a); ok && m <= u.MaxTravelMin {
				reachable = append(reachable, a)
			}
		}
		langs := languageKeys(u)
		for _, a := range reachable {
			for _, s := range dedupSlots(u.Availability) {
				for _, l := range langs {
					k := key{a, s, l.lang, l.exchangeWith}
					sides := pools[k]
					sides[l.side] = append(sides[l.side], u.ID)
					pools[k] = sides
				}
			}
		}
	}

	var out []Bucket
	for k, sides := range pools {
		b := Bucket{Area: k.area, Slot: k.slot, Primary: k.lang}
		if k.exchangeWith != "" {
			if len(sides[0]) < cfg.ExchangeMinPerSide || len(sides[1]) < cfg.ExchangeMinPerSide {
				continue
			}
			b.Secondary = k.exchangeWith
			if len(sides[1]) > len(sides[0]) {
				b.Primary, b.Secondary = b.Secondary, b.Primary
			}
		}
		b.Members = slices.Concat(sides[0], sides[1])
		if len(b.Members) < cfg.MinBucketSize {
			continue
		}
		slices.Sort(b.Members)
		out = append(out, b)
	}
	// Map iteration order is random; sort so output is deterministic.
	slices.SortFunc(out, func(a, b Bucket) int {
		if a.Slot != b.Slot {
			if a.Slot.less(b.Slot) {
				return -1
			}
			return 1
		}
		return cmp.Or(
			cmp.Compare(a.Area, b.Area),
			cmp.Compare(a.Primary, b.Primary),
			cmp.Compare(a.Secondary, b.Secondary),
		)
	})
	return out
}

// langKey is one language bucket a person belongs to.
type langKey struct {
	lang, exchangeWith string
	side               int // exchanges: 0 if fluent in lang, 1 if fluent in exchangeWith
}

// languageKeys lists the regular and exchange language buckets a person can
// join. Exchange pairs are ordered by code so both sides land in one bucket.
func languageKeys(u User) []langKey {
	var keys []langKey
	seen := map[langKey]bool{}
	add := func(k langKey) {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for _, f := range u.Languages {
		if !f.Fluent {
			continue
		}
		add(langKey{lang: f.Code})
		for _, l := range u.Languages {
			if l.Fluent || l.Code == f.Code || u.fluent(l.Code) {
				continue
			}
			if f.Code < l.Code {
				add(langKey{lang: f.Code, exchangeWith: l.Code, side: 0})
			} else {
				add(langKey{lang: l.Code, exchangeWith: f.Code, side: 1})
			}
		}
	}
	return keys
}

func dedupSlots(slots []Slot) []Slot {
	seen := map[Slot]bool{}
	var out []Slot
	for _, s := range slots {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
