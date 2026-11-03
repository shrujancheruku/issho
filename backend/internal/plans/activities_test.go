package plans

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shrujancheruku/issho/backend/internal/cities"
	"github.com/shrujancheruku/issho/backend/internal/matching"
	"github.com/shrujancheruku/issho/backend/internal/testfixture"
)

// fixtureCatalogue: a plain activity, a disabled one, a seasonal one
// (June to August) and one with an override that sets neither.
func fixtureCatalogue(prefix string) ([]cities.Activity, cities.Pack) {
	mk := func(id string) cities.Activity {
		return cities.Activity{
			ID: prefix + id, Category: "test", Energy: 1, CostLevel: 1, Conversational: true,
			MinGroupSize: 2, MaxGroupSize: 6, Hours: 2, Blocks: []string{"morning", "afternoon", "evening"},
		}
	}
	cat := []cities.Activity{mk("plain"), mk("off"), mk("summer"), mk("renamed")}
	pack := cities.Pack{ActivityOverrides: []cities.ActivityOverride{
		{ActivityID: prefix + "off", Disabled: true},
		{ActivityID: prefix + "summer", Months: []int16{6, 7, 8}},
		{ActivityID: prefix + "renamed", Name: cities.Text{"en": "Other"}},
	}}
	return cat, pack
}

func activityIDs(acts []matching.Activity) []string {
	var out []string
	for _, a := range acts {
		out = append(out, string(a.ID))
	}
	return out
}

func TestActivitiesDisabledAndSeasonal(t *testing.T) {
	cat, pack := fixtureCatalogue("")
	for _, tc := range []struct {
		name  string
		month time.Month
		want  []string
	}{
		{"in season", time.July, []string{"plain", "summer", "renamed"}},
		{"out of season", time.January, []string{"plain", "renamed"}},
		{"month 0 ignores seasons, not the disabled", 0, []string{"plain", "summer", "renamed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := activityIDs(Activities(pack, cat, tc.month)); !slices.Equal(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestIntegrationActivitiesMatchQuery: Activities (what the simulator uses)
// and ListMatchingActivities (what the worker uses) keep the same
// activities, and convert them to the same matching.Activity.
func TestIntegrationActivitiesMatchQuery(t *testing.T) {
	const prefix = "t-actmatch-"
	f := testfixture.New(t, "t-actmatch", "UTC", 0)
	ctx := context.Background()
	cat, pack := fixtureCatalogue(prefix)
	cat[0].Blocks = []string{"evening"}
	cat[0].Hours = 1.5
	cat[2].WalkTheme = "nature"
	t.Cleanup(func() {
		for _, a := range cat {
			_, _ = f.Pool.Exec(ctx, "delete from app.activities where id = $1", a.ID)
		}
	})
	for _, a := range cat {
		_, err := f.Pool.Exec(ctx, `insert into app.activities (id, name, category, indoor, energy, cost_level,
			conversational, min_group_size, max_group_size, hours, blocks, walk_theme)
			values ($1, '{"en":"T"}', $2, true, $3, $4, $5, $6, $7, $8, coalesce($9::text[], '{}'), nullif($10, ''))`,
			a.ID, a.Category, a.Energy, a.CostLevel, a.Conversational, a.MinGroupSize, a.MaxGroupSize, a.Hours,
			a.Blocks, a.WalkTheme)
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, o := range pack.ActivityOverrides {
		if _, err := f.Pool.Exec(ctx, `insert into app.city_activity_overrides (city_id, activity_id, enabled, months)
			values ($1, $2, $3, $4)`, f.City, o.ActivityID, !o.Disabled, o.Months); err != nil {
			t.Fatal(err)
		}
	}
	// Only this test's activities: the table is shared with other tests and
	// seeded data.
	ours := func(in []matching.Activity) []matching.Activity {
		return slices.DeleteFunc(slices.Clone(in), func(a matching.Activity) bool {
			return !strings.HasPrefix(string(a.ID), prefix)
		})
	}
	for _, month := range []time.Month{0, time.January, time.July} {
		city, err := loadPlaces(ctx, f.Q, f.City, month)
		if err != nil {
			t.Fatal(err)
		}
		// The query orders by id; Activities keeps the catalogue's order, which
		// the simulator's seeded draws depend on. Order isn't part of the rule.
		byID := func(a, b matching.Activity) int { return strings.Compare(string(a.ID), string(b.ID)) }
		got, want := ours(city.Activities), Activities(pack, cat, month)
		slices.SortFunc(got, byID)
		slices.SortFunc(want, byID)
		if len(got) == 0 {
			t.Fatalf("month %d: query returned none of the fixture activities", month)
		}
		if !slices.EqualFunc(got, want, func(a, b matching.Activity) bool {
			return a.ID == b.ID && a.Category == b.Category && a.CostLevel == b.CostLevel &&
				a.Conversational == b.Conversational && a.MinSize == b.MinSize && a.MaxSize == b.MaxSize &&
				a.Walk == b.Walk && a.Energy == b.Energy && a.Hours == b.Hours &&
				slices.Equal(a.Blocks, b.Blocks) && slices.Equal(a.VenueCategories, b.VenueCategories)
		}) {
			t.Errorf("month %d: query %v, Activities %v", month, activityIDs(got), activityIDs(want))
		}
	}
}
