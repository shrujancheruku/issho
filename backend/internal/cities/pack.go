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
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shrujancheruku/issho/backend/gen/dbgen"
	"github.com/shrujancheruku/issho/backend/internal/maps"
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
	// Matching is a partial matching.Config overlaid on the defaults: weights,
	// group sizes and timing (block start times, signup days). Optional;
	// checked with matching.ParseConfig. Edited in the file, not in the admin
	// SPA.
	Matching json.RawMessage `json:"matching,omitempty"`
	// MeetingPlaces is the ordered search for a meeting place near an area's
	// point when a plan starts without a venue; the first search that finds a
	// place wins. Empty = DefaultMeetingPlaces. Edited in the file, not in the
	// admin SPA.
	MeetingPlaces []MeetingPlace `json:"meeting_places,omitempty"`
}

// MeetingPlace is one search for a meeting place: places of these provider
// types within RadiusM metres of an area's point.
type MeetingPlace struct {
	Types   []string `json:"types"`
	RadiusM int      `json:"radius_m"`
}

// DefaultMeetingPlaces is the search of a city that sets none: a station,
// then a park. (Tokyo's areas are named after stations; a car-based or
// ward-based city can lead with parks or cafés instead.)
func DefaultMeetingPlaces() []MeetingPlace {
	return []MeetingPlace{
		{Types: []string{"train_station", "subway_station"}, RadiusM: 1000},
		{Types: []string{"park"}, RadiusM: 1500},
	}
}

// ParseMeetingPlaces reads cities.meeting_places; an empty list is the
// defaults.
func ParseMeetingPlaces(raw []byte) ([]MeetingPlace, error) {
	var m []MeetingPlace
	if len(bytes.TrimSpace(raw)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&m); err != nil {
			return nil, fmt.Errorf("meeting_places: %w", err)
		}
	}
	if len(m) == 0 {
		return DefaultMeetingPlaces(), nil
	}
	return m, nil
}

type Area struct {
	ID   string  `json:"id"`
	Name Text    `json:"name"`
	Lat  float64 `json:"lat"`
	Lng  float64 `json:"lng"`
	// OSMID is the OpenStreetMap relation of the area's administrative
	// boundary, which `tool area-boundaries` fetches Boundary from.
	OSMID int64 `json:"osm_id,omitempty"`
	// Boundary is the area's outline: one encoded polyline (maps.Polyline)
	// per outer ring. Venues are searched for over all of it; without one,
	// only around the point.
	Boundary []string `json:"boundary,omitempty"`
	// Replaces lists ids of retired areas this one takes over. Applying the
	// pack moves their people, crews and plans here, then deletes them (and
	// their venues and travel times). Keep an entry until every environment
	// has applied the pack. Like OSMID and Boundary it is edited in the file,
	// not in the admin SPA.
	Replaces []string `json:"replaces,omitempty"`
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

// seedPack makes the database match a validated pack in one transaction, so
// a bad pack never leaves a half-seeded city. The catalogue must be seeded
// first because overrides reference activity ids. Seeding is idempotent.
// The pack is authoritative:
//   - overrides it doesn't list are deleted (the activity falls back to the
//     catalogue default);
//   - areas listed in an area's Replaces hand their people, crews and plans
//     to it, then are deleted;
//   - other areas it no longer lists are deleted, but only when nothing
//     references them: otherwise the whole apply is refused (ErrRefused)
//     before anything is written.
//
// Deleting an area cascades to its venues, travel times and walk failures;
// when those include venues an admin curated, it is refused too unless
// opts.DropCuratedVenues. It is private so that Apply is the one way to
// write a pack: it also works out what needs Google again.
func seedPack(ctx context.Context, pool *pgxpool.Pool, p Pack, opts Options) (Diff, error) {
	var d Diff
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		d = Diff{}
		q := dbgen.New(tx)
		// Read inside the transaction so the pre-check and the writes see the
		// same rows.
		before, err := q.ListCityAreas(ctx, p.ID)
		if err != nil {
			return err
		}
		was := make(map[string]maps.LatLng, len(before))
		for _, a := range before {
			was[a.ID] = maps.LatLng{Lat: a.Lat, Lng: a.Lng}
		}
		// The counts (people, crews, plans, curated venues) are read without a
		// row lock. A reference added between this read and the delete is caught
		// by the foreign keys (people, crews, plans: the transaction fails and
		// rolls back). A venue curated in that instant is the one gap we accept:
		// the window is milliseconds, only an admin curating that very area
		// could hit it, and discovery brings the place back as a candidate.
		plan := planAreas(was, p)
		replaced, removed, err := checkRemovals(ctx, q, p.ID, plan, opts)
		if err != nil {
			return err
		}
		d.NewAreas, d.MovedAreas = DiffAreas(was, p.Areas)
		d.RemovedAreas, d.ReplacedAreas = removed, replaced

		name, _ := json.Marshal(p.Name)
		// Absent = the defaults, so removing a block from the pack resets it.
		matchingJSON, places := []byte("{}"), []byte("[]")
		if len(bytes.TrimSpace(p.Matching)) > 0 {
			matchingJSON = p.Matching
		}
		if len(p.MeetingPlaces) > 0 {
			places, _ = json.Marshal(p.MeetingPlaces)
		}
		if err := q.UpsertCity(ctx, dbgen.UpsertCityParams{
			ID: p.ID, Name: name, Timezone: p.Timezone, Currency: p.Currency, Languages: p.Languages,
			TravelModes: p.TravelModes, Matching: matchingJSON, MeetingPlaces: places,
		}); err != nil {
			return err
		}
		for _, a := range p.Areas {
			n, _ := json.Marshal(a.Name)
			b, err := maps.DecodeBoundary(a.Boundary)
			if err != nil {
				return fmt.Errorf("area %s: boundary: %w", a.ID, err)
			}
			if err := q.UpsertArea(ctx, dbgen.UpsertAreaParams{ID: a.ID, CityID: p.ID, Name: n, Lat: a.Lat, Lng: a.Lng, Boundary: wkt(b)}); err != nil {
				return fmt.Errorf("area %s: %w", a.ID, err)
			}
		}
		// Move references before deleting: profiles, crews and plans have no
		// cascade, so the delete would otherwise fail.
		for _, r := range d.ReplacedAreas {
			if _, err := q.MoveProfilesToArea(ctx, dbgen.MoveProfilesToAreaParams{OldID: r.Old, NewID: r.New}); err != nil {
				return fmt.Errorf("moving people from %s to %s: %w", r.Old, r.New, err)
			}
			if _, err := q.MoveCrewsToArea(ctx, dbgen.MoveCrewsToAreaParams{OldID: &r.Old, NewID: &r.New}); err != nil {
				return fmt.Errorf("moving crews from %s to %s: %w", r.Old, r.New, err)
			}
			if _, err := q.MovePlansToArea(ctx, dbgen.MovePlansToAreaParams{OldID: r.Old, NewID: r.New}); err != nil {
				return fmt.Errorf("moving plans from %s to %s: %w", r.Old, r.New, err)
			}
		}
		if gone := append(replacedIDs(d.ReplacedAreas), d.RemovedAreas...); len(gone) > 0 {
			if err := q.DeleteAreas(ctx, gone); err != nil {
				return fmt.Errorf("deleting areas %v: %w", gone, err)
			}
		}
		keep := make([]string, 0, len(p.ActivityOverrides))
		for _, o := range p.ActivityOverrides {
			keep = append(keep, o.ActivityID)
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
		d.RemovedOverrides, err = q.DeleteCityOverridesExcept(ctx, dbgen.DeleteCityOverridesExceptParams{CityID: p.ID, Keep: keep})
		return err
	})
	if err != nil {
		return Diff{}, err
	}
	return d, nil
}

// wkt writes a boundary as a WKT MULTIPOLYGON for PostGIS ("" for none),
// closing each ring as WKT requires.
func wkt(b maps.Boundary) string {
	if len(b) == 0 {
		return ""
	}
	var s strings.Builder
	s.WriteString("MULTIPOLYGON(")
	for i, r := range b {
		if r[0] != r[len(r)-1] {
			r = append(r[:len(r):len(r)], r[0])
		}
		if i > 0 {
			s.WriteByte(',')
		}
		s.WriteString("((")
		for k, q := range r {
			if k > 0 {
				s.WriteByte(',')
			}
			fmt.Fprintf(&s, "%g %g", q.Lng, q.Lat)
		}
		s.WriteString("))")
	}
	s.WriteByte(')')
	return s.String()
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
