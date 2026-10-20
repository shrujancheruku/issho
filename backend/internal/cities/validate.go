package cities

import (
	"errors"
	"fmt"
	"strings"
)

var travelModes = map[string]bool{"transit": true, "walk": true, "drive": true, "bicycle": true}

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
		if c.Name["en"] == "" {
			bad("venue category %s: missing English name", c.ID)
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
		if a.Name["en"] == "" {
			bad("activity %s: missing English name", a.ID)
		}
		if a.Category == "" {
			bad("activity %s: missing category", a.ID)
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
	if p.Name["en"] == "" {
		bad("missing English name")
	}
	if len(p.Languages) == 0 {
		bad("no languages")
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
		if a.Name["en"] == "" {
			bad("area %s: missing English name", a.ID)
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
