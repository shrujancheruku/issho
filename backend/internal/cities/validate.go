package cities

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/shrujancheruku/issho/backend/internal/platform/i18n"
)

// Walking and driving only; public transport isn't calculated (see 00002_geo.sql).
var travelModes = map[string]bool{"walk": true, "drive": true}

// checkName validates a stored name: English is required (it's the source for
// on-demand translation) and any other language must be in allowed. The
// catalogue allows none (English-only, everything else translated on demand);
// city packs allow their own languages, for real local names like 吉祥寺.
func checkName(t Text, allowed []string) error {
	if t[i18n.Authoring] == "" {
		return fmt.Errorf("missing %q", i18n.Authoring)
	}
	for l := range t {
		if l != i18n.Authoring && !slices.Contains(allowed, l) {
			return fmt.Errorf("language %q not allowed here (only %q, plus the city's own languages in city packs)", l, i18n.Authoring)
		}
	}
	return nil
}

// ValidateCatalog checks the venue categories and activities against each
// other and against the database constraints, so a bad edit fails with a clear
// message (in CI and before seeding) instead of a constraint violation.
func ValidateCatalog(cats []VenueCategory, acts []Activity) error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	catIDs := map[string]bool{}
	for _, c := range cats {
		if c.ID == "" || catIDs[c.ID] {
			bad("venue category %q: empty or duplicate id", c.ID)
		}
		catIDs[c.ID] = true
		if err := checkName(c.Name, nil); err != nil {
			bad("venue category %s: name: %v", c.ID, err)
		}
		if len(c.GoogleTypes) == 0 {
			bad("venue category %s: no google_types", c.ID)
		}
		if c.SearchRadiusM < 100 || c.SearchRadiusM > 50000 {
			// Places Nearby Search accepts radii up to 50 km.
			bad("venue category %s: search_radius_m %d outside 100..50000", c.ID, c.SearchRadiusM)
		}
	}

	actIDs := map[string]bool{}
	for _, a := range acts {
		if a.ID == "" || actIDs[a.ID] {
			bad("activity %q: empty or duplicate id", a.ID)
		}
		actIDs[a.ID] = true
	}
	// A replaced id must be retired: not a current activity, and merged into
	// only one, or seeding would delete a live activity or merge it twice.
	replaced := map[string]string{}
	for _, a := range acts {
		for _, r := range a.Replaces {
			if actIDs[r] {
				bad("activity %s: replaces %q, which is still an activity", a.ID, r)
			}
			if prev, ok := replaced[r]; ok {
				bad("activity %s: %q is already replaced by %s", a.ID, r, prev)
			}
			replaced[r] = a.ID
		}
	}
	for _, a := range acts {
		if err := checkName(a.Name, nil); err != nil {
			bad("activity %s: name: %v", a.ID, err)
		}
		if !slices.Contains(Categories, a.Category) {
			bad("activity %s: category %q is not one of %v", a.ID, a.Category, Categories)
		}
		if a.Energy < 1 || a.Energy > 3 {
			bad("activity %s: energy %d outside 1..3", a.ID, a.Energy)
		}
		if a.CostLevel < 0 || a.CostLevel > 3 {
			bad("activity %s: cost_level %d outside 0..3", a.ID, a.CostLevel)
		}
		if a.MinGroupSize < 2 || a.MinGroupSize > a.MaxGroupSize {
			bad("activity %s: group size %d..%d is invalid", a.ID, a.MinGroupSize, a.MaxGroupSize)
		}
		for _, v := range a.VenueCategories {
			if !catIDs[v] {
				bad("activity %s: unknown venue category %q", a.ID, v)
			}
		}
		seenBlock := map[string]bool{}
		for _, b := range a.Blocks {
			if !slices.Contains(AllBlocks, b) || seenBlock[b] {
				bad("activity %s: block %q unknown or repeated", a.ID, b)
			}
			seenBlock[b] = true
		}
		// The theme goes into the model's prompt: keep it a short description.
		if len(a.WalkTheme) > 300 || strings.TrimSpace(a.WalkTheme) != a.WalkTheme {
			bad("activity %s: walk_theme must be trimmed and at most 300 bytes", a.ID)
		}
	}
	return errors.Join(errs...)
}

// ValidatePack checks a city pack. activityIDs, when non-nil, is the catalogue
// the overrides must refer to; SeedPack passes nil and relies on the foreign key.
func ValidatePack(p Pack, activityIDs map[string]bool) error {
	var errs []error
	bad := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if p.ID == "" || p.Timezone == "" || p.Currency == "" {
		bad("id, timezone and currency are required")
	}
	if err := checkName(p.Name, p.Languages); err != nil {
		bad("name: %v", err)
	}
	// languages = the languages commonly used in the city, most common first.
	// Any language is allowed: they're suggestions for onboarding and events.
	if len(p.Languages) == 0 {
		bad("no languages")
	}
	for _, l := range p.Languages {
		if !i18n.Valid(l) {
			bad("language %q is not an ISO 639-1 code like \"ja\"", l)
		}
	}
	if len(p.TravelModes) == 0 {
		bad("no travel_modes")
	}
	for _, m := range p.TravelModes {
		if !travelModes[m] {
			bad("unknown travel mode %q", m)
		}
	}
	if len(p.Areas) == 0 {
		bad("no areas")
	}
	areaIDs := map[string]bool{}
	for _, a := range p.Areas {
		if !strings.HasPrefix(a.ID, p.ID+"-") || areaIDs[a.ID] {
			// Area ids are global primary keys; the city prefix keeps them unique.
			bad("area %q: id must be unique and start with %q", a.ID, p.ID+"-")
		}
		areaIDs[a.ID] = true
		if err := checkName(a.Name, p.Languages); err != nil {
			bad("area %s: name: %v", a.ID, err)
		}
		if a.Lat < -90 || a.Lat > 90 || a.Lng < -180 || a.Lng > 180 || (a.Lat == 0 && a.Lng == 0) {
			bad("area %s: invalid coordinates %v,%v", a.ID, a.Lat, a.Lng)
		}
	}
	seen := map[string]bool{}
	for _, o := range p.ActivityOverrides {
		if seen[o.ActivityID] {
			bad("override %s: duplicate", o.ActivityID)
		}
		// An override name replaces the catalogue name, so it follows the same
		// rule: English only, other languages on demand.
		if len(o.Name) > 0 {
			if err := checkName(o.Name, nil); err != nil {
				bad("override %s: name: %v", o.ActivityID, err)
			}
		}
		seen[o.ActivityID] = true
		if activityIDs != nil && !activityIDs[o.ActivityID] {
			bad("override %s: unknown activity", o.ActivityID)
		}
		for _, m := range o.Months {
			if m < 1 || m > 12 {
				bad("override %s: month %d outside 1..12", o.ActivityID, m)
			}
		}
	}
	return errors.Join(errs...)
}
