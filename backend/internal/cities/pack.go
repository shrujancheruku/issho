// Package cities loads city packs: the data that makes the app work in a new city
// without code changes (areas, languages, local activity overrides).
package cities

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
)

// Text is a localised string keyed by BCP 47 language tag.
type Text map[string]string

type Pack struct {
	ID string `json:"id"`
	// A note for people editing the file (e.g. why a test pack exists);
	// kept when the admin SPA rewrites the file.
	Comment           string             `json:"_comment,omitempty"`
	Name              Text               `json:"name"`
	Timezone          string             `json:"timezone"`
	Currency          string             `json:"currency"`
	Languages         []string           `json:"languages"`
	TravelModes       []string           `json:"travel_modes"`
	Areas             []Area             `json:"areas"`
	ActivityOverrides []ActivityOverride `json:"activity_overrides"`
}

type Area struct {
	ID   string  `json:"id"`
	Name Text    `json:"name"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
}

type ActivityOverride struct {
	ActivityID string  `json:"activity_id"`
	Disabled   bool    `json:"disabled,omitempty"`
	Name       Text    `json:"name,omitempty"`
	Months     []int16 `json:"months,omitempty"`
}

type Activity struct {
	ID             string `json:"id"`
	Name           Text   `json:"name"`
	Category       string `json:"category"`
	Indoor         bool   `json:"indoor"`
	Energy         int16  `json:"energy"`
	CostLevel      int16  `json:"cost_level"`
	Conversational bool   `json:"conversational"`
	SkillDependent bool   `json:"skill_dependent"`
	MinGroupSize   int16  `json:"min_group_size"`
	MaxGroupSize   int16  `json:"max_group_size"`
	// Hours is how long it usually takes a group, for weighing one activity
	// against two in a plan (matching.Activity.Hours).
	Hours float64 `json:"hours"`
	// Venue category ids (venue-categories.json) the activity happens at;
	// empty when it only needs a meeting point.
	VenueCategories []string `json:"venue_categories"`
	// Parts of the day it suits (morning, afternoon, evening); empty = any.
	Blocks []string `json:"blocks,omitempty"`
	// WalkTheme makes the activity a walk: its plans are composed as a route
	// through several stops on this theme (internal/walks), in English, for
	// the model. Empty = the activity happens at one venue.
	WalkTheme string `json:"walk_theme,omitempty"`
	// Ids of retired activities merged into this one. Seeding moves people's
	// preferences for them here and then deletes them.
	Replaces []string `json:"replaces,omitempty"`
}

// Categories are the onboarding tile groups, in display order. The app has
// a label for each (apps/mobile/src/i18n/en.json, "category.<id>").
var Categories = []string{"outdoors", "sports", "wellness", "food", "culture", "games", "creative", "learning"}

// AllBlocks lists the parts of the day, in order (app.availability_slots.block).
var AllBlocks = []string{"morning", "afternoon", "evening"}

// VenueCategory maps one of our venue kinds to provider place types.
type VenueCategory struct {
	ID          string   `json:"id"`
	Name        Text     `json:"name"`
	GoogleTypes []string `json:"google_types"`
	// Nearby-search radius around an area centre. Common places (cafés) use a
	// small radius; rare ones (beaches, campgrounds) need a large one.
	SearchRadiusM int32 `json:"search_radius_m"`
}

// ReadPack reads a city pack without validating or seeding it.
func ReadPack(path string) (Pack, error) {
	var p Pack
	return p, readJSON(path, &p)
}

// ReadActivities reads the activity catalogue without validating it.
func ReadActivities(path string) ([]Activity, error) {
	var acts []Activity
	return acts, readJSON(path, &acts)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// SeedCatalog upserts the venue categories and the global activity catalogue
// from JSON files, after validating them.
func SeedCatalog(ctx context.Context, pool *pgxpool.Pool, venueCategoriesPath, activitiesPath string) (int, error) {
	var cats []VenueCategory
	if err := readJSON(venueCategoriesPath, &cats); err != nil {
		return 0, err
	}
	var acts []Activity
	if err := readJSON(activitiesPath, &acts); err != nil {
		return 0, err
	}
	if err := ValidateCatalog(cats, acts); err != nil {
		return 0, err
	}
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		for _, c := range cats {
			name, _ := json.Marshal(c.Name)
			if err := q.UpsertVenueCategory(ctx, dbgen.UpsertVenueCategoryParams{
				ID: c.ID, Name: name, GoogleTypes: c.GoogleTypes, SearchRadiusM: c.SearchRadiusM,
			}); err != nil {
				return fmt.Errorf("venue category %s: %w", c.ID, err)
			}
		}
		for _, a := range acts {
			name, _ := json.Marshal(a.Name)
			if err := q.UpsertActivity(ctx, dbgen.UpsertActivityParams{
				ID: a.ID, Name: name, Category: a.Category, Indoor: a.Indoor, Energy: a.Energy,
				CostLevel: a.CostLevel, Conversational: a.Conversational, SkillDependent: a.SkillDependent,
				MinGroupSize: a.MinGroupSize, MaxGroupSize: a.MaxGroupSize,
				Column11: cmp.Or(a.Hours, 2), // hours (positional $11; 2 when the catalogue doesn't say)
			}); err != nil {
				return fmt.Errorf("activity %s: %w", a.ID, err)
			}
			// Separate statement so UpsertActivity keeps its positional params.
			vc := a.VenueCategories
			if vc == nil {
				vc = []string{}
			}
			if err := q.SetActivityVenueCategories(ctx, dbgen.SetActivityVenueCategoriesParams{ID: a.ID, VenueCategories: vc}); err != nil {
				return fmt.Errorf("activity %s: %w", a.ID, err)
			}
			blocks := a.Blocks
			if len(blocks) == 0 {
				blocks = AllBlocks
			}
			if err := q.SetActivityBlocks(ctx, dbgen.SetActivityBlocksParams{ID: a.ID, Blocks: blocks}); err != nil {
				return fmt.Errorf("activity %s: %w", a.ID, err)
			}
			var theme *string
			if a.WalkTheme != "" {
				theme = &a.WalkTheme
			}
			if err := q.SetActivityWalkTheme(ctx, dbgen.SetActivityWalkThemeParams{ID: a.ID, WalkTheme: theme}); err != nil {
				return fmt.Errorf("activity %s: %w", a.ID, err)
			}
		}
		// Merges run after every activity exists, and in the same transaction,
		// so nobody's preferences are lost halfway. Deleting a retired activity
		// cascades to its city overrides and anything left of its preferences.
		for _, a := range acts {
			if len(a.Replaces) == 0 {
				continue
			}
			if err := q.MergeActivityPrefs(ctx, dbgen.MergeActivityPrefsParams{ActivityID: a.ID, Replaced: a.Replaces}); err != nil {
				return fmt.Errorf("activity %s: merging preferences: %w", a.ID, err)
			}
			if err := q.DeleteActivities(ctx, a.Replaces); err != nil {
				return fmt.Errorf("activity %s: deleting replaced activities: %w", a.ID, err)
			}
		}
		return nil
	})
	return len(acts), err
}

// SeedPack upserts a city pack in one transaction, so a bad pack never leaves a
// half-seeded city. The catalogue must be seeded first because overrides
// reference activity ids. Seeding is idempotent: re-running updates in place.
// Areas or overrides removed from the JSON are not deleted from the database.
func SeedPack(ctx context.Context, pool *pgxpool.Pool, path string) (Pack, error) {
	var p Pack
	if err := readJSON(path, &p); err != nil {
		return p, err
	}
	if err := ValidatePack(p, nil); err != nil {
		return p, fmt.Errorf("%s: %w", path, err)
	}
	return p, SeedPackData(ctx, pool, p)
}

// SeedPackData seeds a pack already read and validated (the admin SPA reads
// it from GitHub), as SeedPack does.
func SeedPackData(ctx context.Context, pool *pgxpool.Pool, p Pack) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		name, _ := json.Marshal(p.Name)
		if err := q.UpsertCity(ctx, dbgen.UpsertCityParams{
			ID: p.ID, Name: name, Timezone: p.Timezone, Currency: p.Currency, Languages: p.Languages,
			TravelModes: p.TravelModes,
		}); err != nil {
			return err
		}
		for _, a := range p.Areas {
			n, _ := json.Marshal(a.Name)
			if err := q.UpsertArea(ctx, dbgen.UpsertAreaParams{ID: a.ID, CityID: p.ID, Name: n, Lat: a.Lat, Lng: a.Lng}); err != nil {
				return fmt.Errorf("area %s: %w", a.ID, err)
			}
		}
		for _, o := range p.ActivityOverrides {
			var n []byte
			if len(o.Name) > 0 {
				n, _ = json.Marshal(o.Name)
			}
			if err := q.UpsertCityActivityOverride(ctx, dbgen.UpsertCityActivityOverrideParams{
				CityID: p.ID, ActivityID: o.ActivityID, Enabled: !o.Disabled, Name: n, Months: o.Months,
			}); err != nil {
				return fmt.Errorf("override %s: %w", o.ActivityID, err)
			}
		}
		return nil
	})
}

// EncodePack writes a pack the way the files in data/citypacks are
// formatted (two-space indents, non-ASCII and <>& kept as they are, a final
// newline), so a rewrite from the admin SPA changes only what was edited.
func EncodePack(p Pack) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(p); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
