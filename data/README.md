# data

Seed data that makes Issho location-agnostic. Code never hard-codes a city, so adding one means adding data here.

| File | Contents |
|---|---|
| `activities.json` | The **global activity catalogue** (currently 38; target ~200) |
| `citypacks/<city>.json` | One **city pack** per city (currently `tokyo.json`) |

Load with `make seed` (Tokyo) or `cd backend && go run ./cmd/tool seed [-catalog ../data/activities.json] ../data/citypacks/<city>.json`. Seeding runs in a transaction and is idempotent (upserts). **Removing** an entry from the JSON does not delete it from the database; disable it with an override or write a migration instead.

## Activity schema (`activities.json`)

```json
{
  "id": "photo-walk",                       // stable slug; referenced by user prefs. Never rename.
  "name": { "en": "Photo walk", "ja": "フォトウォーク" },
  "category": "outdoors",                   // tile-grid grouping
  "indoor": false,
  "energy": 2,                              // 1 calm … 3 active
  "cost_level": 0,                          // 0 free … 3 expensive
  "conversational": true,                   // can people talk while doing it?
  "skill_dependent": false,                 // does skill level matter for grouping?
  "min_group_size": 3,
  "max_group_size": 8
}
```

## City pack schema (`citypacks/*.json`)

```json
{
  "id": "tokyo",
  "name": { "en": "Tokyo", "ja": "東京" },
  "timezone": "Asia/Tokyo",                 // IANA tz; event times are local to this
  "currency": "JPY",
  "languages": ["ja", "en"],                // defaults offered during onboarding
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
2. Set the id, timezone, currency and languages.
3. Define 10–30 **areas**: neighbourhoods or stations people would recognise as "home". Prefix each id with `<city>-`. Coordinates are the centre of the area.
4. Add overrides for seasonal, local-name or not-applicable activities.
5. `go run ./cmd/tool seed ../data/citypacks/<city>.json`
6. (M3) Build its travel matrix and discover venues with the `tool` commands.
7. Update [../docs/ROADMAP.md](../docs/ROADMAP.md).

Translations: provide at least `en`, plus the city's local language. Missing locales fall back to English (`i18n.Pick`).
