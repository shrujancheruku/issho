package plans

import (
	"slices"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/matching"
)

// Activities converts the catalogue as a city enables it: the city pack's
// overrides drop disabled activities and, in a month (0 = any), the ones out
// of season. It is the Go twin of the ListMatchingActivities query, which
// production uses against the database; the matching simulator has no
// database, so it calls this instead. TestIntegrationActivitiesMatchQuery
// keeps the two filtering the same way.
func Activities(pack cities.Pack, catalogue []cities.Activity, month time.Month) []matching.Activity {
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
		out = append(out, newActivity(a.ID, a.Category, a.CostLevel, a.Conversational, a.MinGroupSize, a.MaxGroupSize,
			a.WalkTheme != "", a.Energy, a.Hours, a.VenueCategories, a.Blocks))
	}
	return out
}

// newActivity is the one field-by-field conversion to matching.Activity,
// shared by the database rows (load.go) and the catalogue (Activities).
func newActivity(id, category string, cost int16, conversational bool, minSize, maxSize int16, walk bool,
	energy int16, hours float64, venueCategories, blocks []string) matching.Activity {
	m := matching.Activity{
		ID: matching.ActivityID(id), Category: category, CostLevel: int(cost),
		Conversational: conversational, MinSize: int(minSize), MaxSize: int(maxSize),
		Walk: walk, Energy: int(energy), Hours: hours,
	}
	for _, v := range venueCategories {
		m.VenueCategories = append(m.VenueCategories, matching.CategoryID(v))
	}
	for _, b := range blocks {
		m.Blocks = append(m.Blocks, matching.Block(b))
	}
	return m
}
