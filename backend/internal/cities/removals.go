package cities

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
)

// ErrRefused wraps the reason Apply wrote nothing: an area the pack dropped
// still has people, crews or plans, or curated venues, or a replaced id isn't
// this city's. The message names the areas and counts, and is safe to show.
var ErrRefused = errors.New("pack not applied")

// Options are the explicit consents Apply needs for losing data.
type Options struct {
	// DropCuratedVenues lets an apply delete venues an admin approved,
	// rejected or added by hand, because their area is removed or replaced.
	// Without it such an apply is refused.
	DropCuratedVenues bool
}

// ReplacedArea is an area that a pack's `replaces` retired: its people,
// crews and plans moved to New and it was deleted.
type ReplacedArea struct {
	Old, New             string
	People, Crews, Plans int
}

// areaPlan is what a pack does to the areas the database has for its city.
type areaPlan struct {
	// replaced are the `replaces` entries the database still has (old, new);
	// entries it doesn't have were applied before and are ignored.
	replaced []ReplacedArea
	// removed are database areas the pack neither lists nor replaces.
	removed []string
	// replaces is every id the pack retires, to check which city owns it.
	replaces []string
}

// planAreas is pure: was is the city's areas in the database.
func planAreas(was map[string]maps.LatLng, p Pack) areaPlan {
	var plan areaPlan
	inPack := map[string]bool{}
	retired := map[string]bool{}
	for _, a := range p.Areas {
		inPack[a.ID] = true
		for _, old := range a.Replaces {
			retired[old] = true
			plan.replaces = append(plan.replaces, old)
			if _, ok := was[old]; ok {
				plan.replaced = append(plan.replaced, ReplacedArea{Old: old, New: a.ID})
			}
		}
	}
	for id := range was {
		if !inPack[id] && !retired[id] {
			plan.removed = append(plan.removed, id)
		}
	}
	slices.Sort(plan.removed)
	return plan
}

// checkRemovals refuses (ErrRefused, nothing written yet) what would lose
// data without being asked to, and returns the replaced areas with their
// counts and the areas to delete outright. It runs in Apply's transaction.
func checkRemovals(ctx context.Context, q *dbgen.Queries, cityID string, plan areaPlan, opts Options) ([]ReplacedArea, []string, error) {
	var problems []string

	// A replaced id must be this city's: a typo must not delete another
	// city's area.
	if len(plan.replaces) > 0 {
		owners, err := q.ListAreaCities(ctx, plan.replaces)
		if err != nil {
			return nil, nil, err
		}
		for _, o := range owners {
			if o.CityID != cityID {
				problems = append(problems, fmt.Sprintf("%s: replaced, but it belongs to city %s", o.ID, o.CityID))
			}
		}
	}
	if len(problems) > 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrRefused, strings.Join(problems, "; "))
	}

	ids := append(replacedIDs(plan.replaced), plan.removed...)
	counts := map[string]dbgen.CountAreaReferencesRow{}
	if len(ids) > 0 {
		rows, err := q.CountAreaReferences(ctx, ids)
		if err != nil {
			return nil, nil, err
		}
		for _, r := range rows {
			counts[r.ID] = r
		}
	}
	replaced := slices.Clone(plan.replaced)
	for i, r := range replaced {
		c := counts[r.Old]
		replaced[i].People, replaced[i].Crews, replaced[i].Plans = int(c.People), int(c.Crews), int(c.Plans)
	}
	curated := func(id string) {
		if c := counts[id]; c.CuratedVenues > 0 && !opts.DropCuratedVenues {
			problems = append(problems, fmt.Sprintf("%s: %d curated venues would be deleted; pass the drop-curated-venues option to allow it", id, c.CuratedVenues))
		}
	}
	for _, r := range replaced {
		curated(r.Old)
	}
	for _, id := range plan.removed {
		c := counts[id]
		var refs []string
		for _, n := range []struct {
			n    int32
			noun string
		}{{c.People, "people"}, {c.Crews, "crews"}, {c.Plans, "plans"}} {
			if n.n > 0 {
				refs = append(refs, fmt.Sprintf("%d %s", n.n, n.noun))
			}
		}
		if len(refs) > 0 {
			problems = append(problems, fmt.Sprintf("%s: %s; list it in an area's replaces", id, strings.Join(refs, ", ")))
		}
		curated(id)
	}
	if len(problems) > 0 {
		return nil, nil, fmt.Errorf("%w: %s", ErrRefused, strings.Join(problems, "; "))
	}
	return replaced, plan.removed, nil
}

func replacedIDs(rs []ReplacedArea) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Old
	}
	return out
}
