package matching

import (
	"cmp"
	"slices"
)

// Level is how likely someone is to want a plan, for the feed's order and
// the map's colours.
type Level int

const (
	// NotForYou: they don't speak the plan's language(s), so they can't
	// sign up. Still shown on the map, greyed out.
	NotForYou Level = iota
	// Low: no preference for the activity, or it's beyond their travel time.
	Low
	Maybe // open to try
	Good  // like
	Top   // love
)

var levelNames = [...]string{"not-for-you", "low", "maybe", "good", "top"}

func (l Level) String() string { return levelNames[l] }

// OpenPlan is a plan taking signups.
type OpenPlan struct {
	Plan
	Signups int // shown to everyone as a count ("7 interested"), never who
}

// FeedItem is one open plan as one person sees it.
type FeedItem struct {
	OpenPlan
	Level   Level
	Score   float64 // the feed's order within a level
	Minutes int     // travel time from their home area; -1 = no route
}

// Feed ranks every open plan for one person: the ones they'd most likely
// want first, then the rest, so scrolling far enough shows everything. The
// same levels colour the map.
//
// Within a level, plans gain from momentum (signups so far, up to
// cfg.MomentumAt) and lose when the activity suits no part of the day
// they're usually free, or costs more than their budget; nearer plans break
// ties.
func Feed(u *User, plans []OpenPlan, acts map[ActivityID]*Activity, travel TravelTimes, cfg Config) []FeedItem {
	out := make([]FeedItem, 0, len(plans))
	for _, p := range plans {
		a := acts[p.Activity]
		item := FeedItem{OpenPlan: p, Minutes: -1}
		if m, ok := travel.Minutes(u.HomeArea, p.Area); ok {
			item.Minutes = m
		}
		v, hasPref := interest(u, a, cfg)
		switch {
		case !u.speaks(p.Primary, p.Secondary):
			item.Level = NotForYou
		case !travel.reachable(u, p.Area) || !hasPref:
			item.Level = Low
		default:
			item.Level = Maybe + Level(u.Prefs[a.ID])
		}
		item.Score = v + cfg.Weights.Momentum*min(1, float64(p.Signups)/float64(cfg.MomentumAt))
		if !usuallyFree(u, a) {
			item.Score *= 0.5
		}
		out = append(out, item)
	}
	slices.SortFunc(out, func(x, y FeedItem) int {
		return cmp.Or(cmp.Compare(y.Level, x.Level), cmp.Compare(y.Score, x.Score),
			cmp.Compare(minutesOrder(x.Minutes), minutesOrder(y.Minutes)),
			cmp.Compare(x.Area, y.Area), cmp.Compare(x.Activity, y.Activity),
			cmp.Compare(x.Primary, y.Primary), cmp.Compare(x.Secondary, y.Secondary))
	})
	return out
}

// minutesOrder sorts unreachable plans after reachable ones.
func minutesOrder(m int) int {
	if m < 0 {
		return int(^uint(0) >> 1)
	}
	return m
}
