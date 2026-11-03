# data

Seed data that makes Issho location-agnostic. Code never hard-codes a city, so adding one means adding data here.

| File | Contents |
|---|---|
| `activities.json` | The **global activity catalogue** (66 activities in 8 categories; broad on purpose, see below) |
| `venue-categories.json` | **Venue categories** (48): our kinds of place, mapped to Google place types |
| `citypacks/<city>.json` | One **city pack** per city: `tokyo.json` (launch city), `austin.json` (a **synthetic test pack**: a real car-based city that proves a second city needs no code changes; not a launch city) and `da-nang-hoi-an.json` (two cities in one pack, below) |

City packs can also be edited (or created) in the admin SPA's **Cities** page, which opens a pull request with the change; once merged, the page loads it into the database. Keep hand edits in the same format (`cities.EncodePack`: two-space indents, unescaped non-ASCII; a test checks it) so the editor's pull requests change only what was edited.

Load with `make seed` (everything) or `cd backend && go run ./cmd/tool seed ../data/citypacks/<city>.json`. Each pack goes through `cities.Apply`: with `GOOGLE_MAPS_API_KEY` set, new or moved areas get their travel times; without it the pack is seeded and the travel times are skipped (run `build-matrix` later).

**Validation:** `go test ./internal/cities/` (part of `make test` and CI) checks every file here: required fields, ranges, duplicate ids, references between files, and that every Google type is a valid Places API (New) type. Seeding validates too. Seeding runs in a transaction and is idempotent (upserts). **Catalogue activities:** removing one from `activities.json` does not delete it from the database; disable it with an override, or merge it into another activity with `replaces` (below). **City packs are authoritative**, see "Applying a pack" below.

## Activity schema (`activities.json`)

```json
{
  "id": "hike",                             // stable slug; referenced by user prefs. Never rename; merge instead (replaces)
  "name": { "en": "Hiking and nature walks" }, // English only; other languages are translated on demand
  "category": "outdoors",                   // tile-grid grouping (one of the 8 below)
  "indoor": false,
  "energy": 2,                              // 1 calm … 3 active
  "cost_level": 0,                          // 0 free … 3 expensive
  "conversational": true,                   // can people talk while doing it?
  "skill_dependent": false,                 // does skill level matter for grouping?
  "min_group_size": 3,                      // at least 2
  "max_group_size": 8,
  "venue_categories": [],                   // where it happens (ids from venue-categories.json); [] = meeting point only
  "blocks": ["morning", "afternoon"],       // optional: parts of the day it suits (morning, afternoon, evening); omitted = any
  "walk_theme": "temples and shrines: …",  // optional: makes it a walk (see below); English, at most 300 bytes
  "replaces": ["waterfall-hike", "birdwatching"] // optional: retired activity ids merged into this one
}
```

**Walks.** An activity with a `walk_theme` is planned as a walk: 2–4 hours from a station or café, through 4–7 stops on the theme, to somewhere to sit and talk (composed by the worker, `internal/walks`). The theme goes into the model's prompt, so it says what the stops should be and what makes them good ("several different ones, with what makes each distinct"). Walks today: city walk, photography, temples and shrines, seasonal sights, food tour, cafés and tea. A walk's `venue_categories` only tell matching the area has such places.

Set `blocks` only for activities tied to a time of day: bars and dinners in the evening, outdoor and museum activities in daylight, brunch in the morning. Matching uses it because venue opening hours can't be stored (Google Maps terms).

`category` groups the onboarding tile grid, in this order: `outdoors`, `sports`, `wellness`, `food` (and drink), `culture`, `games`, `creative`, `learning` (and volunteering). The list is `cities.Categories`; the app needs a `category.<id>` label in `apps/mobile/src/i18n/en.json` for each (a test checks both ways). Activities that only make sense in some cities (e.g. `temple-shrine`, `japanese-arts`) stay in the global catalogue and are disabled in other cities' packs.

**Keep activities broad.** Onboarding shows every activity as a tile, and every tap is a decision; 230 narrow activities (ten kinds of walk, nine kinds of volunteering) made it a chore. Broad activities also pool interest: matching suggests plans per activity, so "Hiking and nature walks" gathers people who'd otherwise be split across waterfall hikes and birdwatching. The specific flavour belongs to the plan (its description, M6), not the catalogue. Before adding an activity, check whether it's a variant of an existing one; examples in a name ("Craft workshop (knitting, candles, woodwork …)") help people recognise what's covered. Don't merge activities people choose between, though: someone who plays tennis doesn't necessarily want basketball, so sports are one activity each (with close variants such as beach volleyball folded in). And an activity must be something people do, not a way of feeling about it: "Try a new sport" was removed because "open to try" on a specific sport already says that.

**Merging activities:** list the retired ids in the surviving activity's `replaces` and delete their entries. Seeding (in one transaction) moves each person's preferences to the surviving activity, keeping their strongest interest, then deletes the retired activities; their city overrides go with them, so update the packs too. A replaced id can't still be an activity or be replaced twice (validated). Keep `replaces` entries until every environment has been seeded.

## Venue category schema (`venue-categories.json`)

```json
{
  "id": "cafe",                             // stable slug; referenced by activities and app.venues
  "name": { "en": "Café" },                // English only
  "google_types": ["cafe", "coffee_shop"],  // Places API (New) Table A types; the place's PRIMARY type must be one of these
  "search_radius_m": 800                    // Nearby Search radius around each area (100–50000)
}
```

Matching is on the primary type, so prefer specific types (`flea_market`, `dance_hall`) over broad ones: `market` matched supermarkets and `night_club` matched hostess bars. Common places get a small radius, rare ones a large one. Categories with a radius of 10 km or more (beaches, trails, campgrounds) are searched once per city instead of once per area. If Google adds new types, refresh `backend/internal/maps/google/placetypes.txt`.

## City pack schema (`citypacks/*.json`)

```json
{
  "id": "tokyo",
  "name": { "en": "Tokyo", "ja": "東京" },
  "timezone": "Asia/Tokyo",                 // IANA tz; event times are local to this
  "currency": "JPY",
  "languages": ["ja", "en"],                // languages commonly used here, most common first; any language code. Suggestions for onboarding/events; the first is used for venue search
  "travel_modes": ["walk", "drive"],        // modes to calculate: walk and/or drive (no public transport)
  "areas": [                                // the unit users pick as "home"
    { "id": "tokyo-musashino", "name": { "en": "Musashino", "ja": "武蔵野市" }, "lat": 35.7054, "lng": 139.5660, "osm_id": 1761804, "replaces": ["tokyo-kichijoji"] },
    { "id": "da-nang-hoi-an-hai-chau", "name": { "en": "Hai Chau", "vi": "Hải Châu" }, "lat": 16.059, "lng": 108.2195,
      "osm_id": 13969677,                   // optional: the area's administrative boundary in OpenStreetMap (a relation id)
      "boundary": ["emfaBw_lsS…"],          // optional: its outline, one encoded polyline per ring; written by make boundaries
      "replaces": ["da-nang-hoi-an-old"] }  // optional: retired area ids this one takes over (see "Applying a pack")
  ],
  "activity_overrides": [
    { "activity_id": "seasonal-viewing", "months": [3, 4] },    // seasonal
    { "activity_id": "beach-day", "disabled": true },           // hide in this city
    { "activity_id": "sento", "name": { "en": "Public bath" } } // local name
  ]
}
```

**Boundaries.** An area with a `boundary` has its venues searched over the whole of it, and every venue (found or added by hand) goes to the area that contains it; places outside every area are left out. Without one, venues are searched within each category's radius of the point and go to the nearest point, as for Austin today. `make boundaries CITY=<city>` fetches each area's outline from OpenStreetMap by its `osm_id` (Nominatim, simplified to about 20 m) and rewrites the pack; validation checks that each point is inside its own boundary and nobody else's. The admin SPA's editor keeps boundaries as they are (it doesn't edit them). Boundaries are OpenStreetMap data, © OpenStreetMap contributors, available under the [Open Database License](https://opendatacommons.org/licenses/odbl/).

## Applying a pack

A pack is the source of truth for its city's areas and activity overrides: applying it (`tool seed`, or **Apply from main** in the admin SPA) makes the database match it, in one transaction. Both go through `cities.Apply`.

| The pack... | Applying it... |
|---|---|
| lists an area that is new or moved more than 100 m | upserts it; travel times are rebuilt (with a Maps key) |
| no longer lists an override | **deletes the override row**: the activity is offered again under its catalogue name, in every month. A `disabled: true` override that is removed re-enables the activity |
| no longer lists an area **nothing references** (no profile, crew or plan) | **deletes the area and cascades** to its venues, travel times and walk failures |
| no longer lists an area that people, crews or plans still use | **refuses before writing anything**, naming the area and counts: `tokyo-shibuya: 12 people, 3 plans; list it in an area's replaces` |
| lists `replaces: [old ids]` on an area | moves `profiles.home_area_id`, `crews.area_id` and `plans.area_id` from each old area to this one, then deletes the old areas (venues, travel times and walk failures cascade) |

An apply is also **refused when it would delete curated venues** (approved, rejected or added by hand) of a removed or replaced area, since discovery only brings back candidates. `tool seed -drop-curated-venues` allows it; the admin SPA cannot. Nothing else in a pack touches users' data, and no apply ever deletes a city, or a profile, crew or plan.

`tool seed` logs a warning listing the removed areas, the replaced areas (old, new, and how many people, crews and plans moved) and the removed overrides. The admin SPA's apply answers with an error that names them when it refuses. There is no dry run: the pull request review is the preview.

**Retiring an area** (a hand edit in a pull request; the editor can't remove areas): delete its entry and list its id in the `replaces` of the area that covers it. Validation rejects a replaced id that is still an area of any pack, or replaced twice; `Apply` rejects one that belongs to another city. Keep the `replaces` entry until every environment (local, `issho-dev`, production) has applied the pack; after that it is a no-op and can go. The editor keeps `osm_id`, `boundary` and `replaces` from `main` in its pull requests.

## Adding a city

A pack is a matching pool: people are only grouped with others in the same pack, and each person's travel limit decides how far within it. A town too small for many plans on its own can join a nearby city's pack as a few more areas, as Hoi An does in `da-nang-hoi-an.json` (three wards about 30 km from Da Nang); people there who won't travel far still meet each other, and those who will also meet Da Nang's.


1. Copy `citypacks/tokyo.json` to `citypacks/<city>.json`.
2. Set the id, timezone, currency, languages and travel modes. The file name must be `<id>.json`.
3. Define the **areas**: the government's own administrative units at the level below the city (Da Nang's wards, for example), with their official names as the local name. They cover the city without gaps and have boundaries anyone can check. Prefix each id with `<city>-`. For the coordinates, take the unit's administrative point from OpenStreetMap (Nominatim) rather than the middle of its boundary, which for a large unit can be in the hills: the point is where travel times start, so it should be in the built-up part. Set each area's `osm_id` to the unit's OpenStreetMap relation and run `make boundaries CITY=<city>`, then draw the boundaries and points on a map to check. This is the rule for every city, unless there is a very good reason not to: the local government's units, boundaries and names (owner's decision, 2026-11-03). Tokyo's areas are 7 municipalities (区/市) in the west of the city, named in Japanese as on addresses (`武蔵野市`, `杉並区`, …); its old station areas were retired with `replaces`.
4. Add overrides for seasonal, local-name or not-applicable activities.
5. `go test ./internal/cities/`, then `make seed`.
6. Build its travel matrix and discover venues: `make matrix CITY=<city>` and `make venues CITY=<city>` (dry-run first with `ARGS=-dry-run`; with boundaries, discovery stops at `-max-requests`, 4,000 by default; see [../docs/google-maps.md](../docs/google-maps.md)). Rebuild the matrix at least every 30 days.
7. Update [../docs/ROADMAP.md](../docs/ROADMAP.md).

Languages (how they're shown: [../docs/architecture.md](../docs/architecture.md#languages)):
- **English is the only language stored here.** Activity names, venue-category names and override names are English-only (validation rejects anything else). Every other language is machine-translated on demand and cached in the database, so there are no translations to maintain.
- City and area names need `en`, and may also carry the **real local name** in one of the city's own `languages` (`東京`, `吉祥寺`). These are names, not translations, so they're kept and used as is.
