# data

Seed data that makes Issho location-agnostic. Code never hard-codes a city, so adding one means adding data here.

| File | Contents |
|---|---|
| `activities.json` | The **global activity catalogue** (230 activities in 12 categories) |
| `venue-categories.json` | **Venue categories** (48): our kinds of place, mapped to Google place types |
| `citypacks/<city>.json` | One **city pack** per city: `tokyo.json` (launch city, transit) and `austin.json` (a **synthetic test pack**: a real car-based city that proves a second city needs no code changes; not a launch city) |

Load with `make seed` (everything) or `cd backend && go run ./cmd/tool seed ../data/citypacks/<city>.json`.

**Validation:** `go test ./internal/cities/` (part of `make test` and CI) checks every file here: required fields, ranges, duplicate ids, references between files, and that every Google type is a valid Places API (New) type. Seeding validates too. Seeding runs in a transaction and is idempotent (upserts). **Removing** an entry from the JSON does not delete it from the database; disable it with an override or write a migration instead.

## Activity schema (`activities.json`)

```json
{
  "id": "photo-walk",                       // stable slug; referenced by user prefs. Never rename.
  "name": { "en": "Photo walk", "ja": "フォトウォーク", "es": "Paseo fotográfico", "fr": "Balade photo" }, // all four required
  "category": "outdoors",                   // tile-grid grouping
  "indoor": false,
  "energy": 2,                              // 1 calm … 3 active
  "cost_level": 0,                          // 0 free … 3 expensive
  "conversational": true,                   // can people talk while doing it?
  "skill_dependent": false,                 // does skill level matter for grouping?
  "min_group_size": 3,                      // at least 2
  "max_group_size": 8,
  "venue_categories": []                    // where it happens (ids from venue-categories.json); [] = meeting point only
}
```

`category` groups the onboarding tile grid: `outdoors`, `sports`, `food`, `wellness`, `culture`, `games`, `creative`, `learning`, `shopping`, `seasonal`, `community`, `nightlife`. Activities that only make sense in some cities (e.g. `rakugo`, `hanami`) stay in the global catalogue and are disabled in other cities' packs.

## Venue category schema (`venue-categories.json`)

```json
{
  "id": "cafe",                             // stable slug; referenced by activities and app.venues
  "name": { "en": "Café", "ja": "カフェ" },
  "google_types": ["cafe", "coffee_shop"],  // Places API (New) Table A types; any match counts
  "search_radius_m": 800                    // Nearby Search radius around each area (100–50000)
}
```

Common places get a small radius, rare ones a large one. Categories with a radius of 10 km or more (beaches, trails, campgrounds) are searched once per city instead of once per area. If Google adds new types, refresh `backend/internal/maps/google/placetypes.txt`.

## City pack schema (`citypacks/*.json`)

```json
{
  "id": "tokyo",
  "name": { "en": "Tokyo", "ja": "東京" },
  "timezone": "Asia/Tokyo",                 // IANA tz; event times are local to this
  "currency": "JPY",
  "languages": ["ja", "en"],                // languages commonly used here, most common first; any language code. Suggestions for onboarding/events; the first is used for venue search
  "travel_modes": ["transit", "walk"],      // how people get around: transit, walk, drive, bicycle. Car-based cities: ["drive"]
  "areas": [                                // the unit users pick as "home"
    { "id": "tokyo-kichijoji", "name": { "en": "Kichijoji", "ja": "吉祥寺" }, "lat": 35.7030, "lng": 139.5798 }
  ],
  "activity_overrides": [
    { "activity_id": "hanami", "months": [3, 4] },              // seasonal
    { "activity_id": "beach-day", "disabled": true },           // hide in this city
    { "activity_id": "sento", "name": { "en": "Public bath" } } // local name
  ]
}
```

## Adding a city

1. Copy `citypacks/tokyo.json` to `citypacks/<city>.json`.
2. Set the id, timezone, currency, languages and travel modes. The file name must be `<id>.json`.
3. Define 10–30 **areas**: neighbourhoods or stations people would recognise as "home". Prefix each id with `<city>-`. Coordinates are the centre of the area.
4. Add overrides for seasonal, local-name or not-applicable activities.
5. `go test ./internal/cities/`, then `make seed`.
6. Build its travel matrix and discover venues: `make matrix CITY=<city>` and `make venues CITY=<city>` (dry-run first with `ARGS=-dry-run`; see [../docs/google-maps.md](../docs/google-maps.md)). Rebuild the matrix at least every 30 days.
7. Update [../docs/ROADMAP.md](../docs/ROADMAP.md).

Translations (how languages are shown: [../docs/architecture.md](../docs/architecture.md#languages)):
- Activity and venue-category names **must** have every language in `i18n.Translated` (`en`, `ja`, `es`, `fr`); validation fails otherwise. An override `name` replaces the whole name, so it must be complete too.
- City and area names need `en` (the authoring language, used as the last-resort fallback); add other languages where the name actually differs (`東京`, `Tokio`).
- Adding a translated language: add it to `i18n.Translated`, translate every catalogue name, and add the app's strings. People and events can already use any language; this only adds our own translations.
