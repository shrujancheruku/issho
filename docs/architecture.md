# Architecture

## System overview

```
┌──────────────────────┐   ConnectRPC (POST, JSON)   ┌───────────────────────────────┐
│ apps/mobile (Expo)   │ ──────────────────────────▶ │ backend/cmd/api (Go)          │
│ apps/admin (Vite)    │  Authorization: Bearer JWT  │  ├ auth interceptor (JWKS)    │
└─────────┬────────────┘                             │  ├ internal/<domain> handlers │
          │ sign-in (supabase-js)                    │  └ pgx pool ─────────────┐    │
          ▼                                          └──────────────────────────┼────┘
┌──────────────────────┐   JWKS (public keys)                                  │
│ Supabase Auth        │ ◀─────────────────────────── API fetches and caches  │
└──────────────────────┘                                                       ▼
                                                     ┌───────────────────────────────┐
  backend/cmd/worker (River jobs) ──────────────────▶ │ Postgres + PostGIS (Supabase) │
                                                     │  schema `app` (ours)          │
                                                     │  schema `auth` (Supabase)     │
                                                     └───────────────────────────────┘
```

- **Clients never talk to the database directly.** The `app` schema is revoked from Supabase's `anon` and `authenticated` roles (see migration `00001`). All reads and writes go through Go services. PostgREST and Realtime are unused. Supabase's Data API (PostgREST) is **disabled**, locally and on the hosted project, so `anon` can't reach any table, including PostGIS's `public.spatial_ref_sys`, whose grants we can't revoke. goose's version table lives in its own `goose` schema.
- **Supabase Auth issues the identity.** The app signs in with Supabase and gets a short-lived access JWT. The Go API verifies that JWT against the project's public JWKS. It never holds a signing secret.
- **The app user row is created lazily.** The first authenticated `GetMe` creates `app.users` with the same UUID as `auth.users`. On Supabase, a foreign key ties the two together and cascades deletes.

## Authentication flow

1. **Sign-in** (mobile): `supabase.auth.signInWithOtp({ email })` → Supabase emails a 6-digit code → `verifyOtp` → session (short-lived ES256 access token plus refresh token).
2. The session is stored in the iOS Keychain / Android Keystore through a chunked `expo-secure-store` adapter (`apps/mobile/src/lib/secure-storage.ts`).
3. Every API call sends `Authorization: Bearer <access token>`. The API verifies it against Supabase's JWKS; there's no call to Supabase per request.
4. The first `IdentityService.GetMe` creates the `app.users` row. `onboarded` decides whether the app shows onboarding or the main tabs.
5. **Account deletion:** `DeleteAccount` uses the server-only secret key to delete the Supabase auth user. The foreign key cascades to all app data. Tokens already issued stay cryptographically valid until they expire, but `GetMe` then answers `unauthenticated: account deleted`, and the app signs out.

## Request lifecycle

1. The client calls, for example, `POST /issho.v1.IdentityService/GetMe` with JSON and `Authorization: Bearer <token>`.
2. **CORS** (`cmd/api`) checks the origin. Only browser origins in `ALLOWED_ORIGINS` pass. Native apps send no Origin header.
3. The **auth interceptor** (`internal/platform/auth`) runs:
   - Public procedures (`HealthService.Check`, and `UIService.GetStrings` because the sign-in screens need their text) skip it.
   - Everything else must have a valid ES256/RS256 token with Supabase's issuer, the `authenticated` audience and an unexpired `exp`. The caller is put on the request context.
   - Admin-only services (`ModerationService`, `ReviewService` and `CurationService`, for the admin SPA) also need `app_metadata.admin` in the token (`auth.AdminOnly`), else `permission_denied`. Admins are ordinary Supabase users given that role with `tool set-admin`; there's no admin list in our config.
4. The **handler** (`internal/<domain>/handler.go`) validates input and calls the domain **service**.
5. The **service** contains the business logic and talks to Postgres through **sqlc** queries (`backend/gen/dbgen`).
6. Errors come back as Connect error codes (`unauthenticated`, `invalid_argument`, …). The outermost **errs interceptor** replaces any non-Connect error with a generic `internal error`, after logging it, so database and upstream details never reach clients.

## Backend layout (modular monolith)

```
backend/
  cmd/api      HTTP server: wires config, DB, auth and handlers
  cmd/tool     operational CLI: migrations and seeding
  cmd/worker   (M6) River background jobs
  internal/
    platform/  cross-cutting: config, db, auth, i18n (no business logic)
    <domain>/  identity, catalog, cities, … each has handler.go (transport) and service.go (logic)
  db/migrations  goose SQL (embedded into binaries)
  db/queries     sqlc input SQL
  gen/           generated: protobuf/Connect (gen/issho) and sqlc (gen/dbgen). Never edit.
```

Rules:
- **Domains talk to each other only through service types**, never through each other's queries.
- **`internal/matching` (M4) is pure:** plain structs in, results out, no I/O, no clock, no randomness. This makes it deterministic and easy to simulate (`internal/matching/sim`). It works **plan first, time second**: `Suggest` opens plans without a date, `Feed` ranks them per person, and `Schedule` picks a date and forms groups once a plan's signups are enough. **Every area is a candidate meeting place**: a person joins the pool of each area they can reach. Steps and rules: [backend README](../backend/README.md#matching).
- **Every external vendor sits behind a Go interface**, so implementations can be swapped. Done: `maps.Router` (travel times) and `maps.PlaceSearcher` (venues), with Google implementations in `internal/maps/google`; `tagging.Tagger` (free-text interests → activities), with a Gemini implementation in `internal/tagging/gemini`; `walks.Proposer` (a walk from the model's own knowledge) in `internal/walks/gemini`, checked by `maps.PlaceFinder` and `maps.WalkRouter`. Planned: `Copywriter`, `Verifier`, `Push`, `Storage`.

## Data model

Defined in `backend/db/migrations/` (`00001_init.sql`, `00002_geo.sql`). All app tables live in the `app` schema.

| Table | Purpose |
|---|---|
| `cities` | One row per city pack: timezone, currency, default languages, travel modes (`walk`/`drive`; no public transport), matching-weight overrides |
| `areas` | The unit a user chooses as "home" (a neighbourhood or station), with its centre as PostGIS `geography` |
| `area_travel_times` | Area-to-area minutes per mode, from the Routes API (`tool build-matrix`). A cache: rows older than 30 days are deleted. |
| `activities` | The global catalogue. `name` is JSON keyed by language (`{"en": …, "ja": …}`); `venue_categories` lists where it can happen; `blocks` the parts of the day it suits |
| `venue_categories` | Our venue kinds (cafe, park, karaoke …) mapped to Google place types, with a search radius |
| `venues` | Discovered places: `google_place_id` + category, nearest area, curation status (per place, set by admins: who and when, or who added it), our tags. A city's `approved_venues_only` makes the matcher use only approved ones. **No Google content** (names, addresses, locations) is stored; see [google-maps.md](google-maps.md#5-terms-what-we-may-store). |
| `city_activity_overrides` | A city can rename, disable or set seasonal months for an activity |
| `users` | App account; `id` = Supabase `auth.users.id` |
| `profiles` | Matching preferences (home area, travel time, group size, gender rules, …), display languages (`primary_language`, optional `secondary_language`) and the accepted code-of-conduct version. Written only whole, by `ProfileService.SaveProfile` (onboarding and settings) |
| `user_languages` | Languages a person **speaks** (fluent, or learning with a level); used for matching |
| `availability_slots` | Weekday × morning/afternoon/evening |
| `user_activity_prefs` | Weight per activity: 0 = open to try, 1 = like, 2 = love |

### Languages

Languages are **symmetric**: no language is the default, and people who don't speak English are fully supported. People can speak any language, and events can be held in any language. Everything is shown in a **primary** language plus an optional **secondary** one:

| Where | Primary | Secondary |
|---|---|---|
| An event | The event's language | The event's second language if it has one (e.g. a JP–ES exchange). Otherwise the user's primary language, if different; failing that, the user's secondary choice. |
| The rest of the app | The user's primary language | The user's secondary choice, if any |

Examples: a JP–ES exchange in Tokyo shows Japanese + Spanish to everyone; a Japanese-only event shows Japanese + the viewer's own language (Spanish for a Spanish speaker).

`i18n.ForEvent` and `i18n.ForApp` implement these rules; nothing else should re-implement them. Short text (names, titles) is shown **side by side** ("陶芸 · Cerámica"); longer text (descriptions, UI copy) uses a **switch** between the two languages.

**Storage: English always, everything else on demand.** Localised text is stored as JSON objects (`{"en": …, "ja": …}`). English is the only language that must always be present; our own content (activity catalogue, venue categories) is stored in English only. Other languages come from `internal/translate`:

1. text already stored in that language wins: real local names in city packs (`吉祥寺`), and events prefilled in their own languages (M6)
2. otherwise a cached machine translation from `app.translations`
3. otherwise Google Cloud Translation translates it now, and the result is cached and reused for everyone. A request waits up to 8 s; a slower translation finishes in the background for the next request. Without `GOOGLE_TRANSLATE_API_KEY`, step 3 is skipped.

`i18n.Pair` then decides what to show: if the primary language isn't available yet, the secondary is shown alone; English is only the last resort. APIs return both texts (e.g. `name` + `secondary_name`) and clients choose the layout. Any ISO 639-1 language is accepted.

**Events** (M6) are stored with English **plus each of their own languages prefilled** (`translate.Service.Prefill`), so a JP–ES exchange exists in en, ja and es from the start; any other viewer's language is translated on demand.

**The app's own UI text** is written in English in `apps/mobile/src/i18n/en.json`. `UIService.GetStrings` translates the whole catalogue into one language through the same cache; the app keeps a copy on the device and asks again while translations are still running. The server only translates its own copy of that file (`make gen` copies it into `backend/internal/uistrings`), so the RPC can be public, for the sign-in screens, without translating arbitrary text. Placeholders (`{email}`) are checked: a translation that loses one is dropped, and the app shows English for that string. Before sign-in the app uses the device language; afterwards, the profile's two languages.

**Area names are never machine-translated.** They're proper nouns, and translation mangled them ("The Domain" → "El dominio"). Only stored names are shown: English plus a real local name such as 吉祥寺. City names are translated (Tokyo → Tokio).

**Google place names** are never stored (Maps terms allow storing only place IDs). They're fetched live from Places in each displayed language, so people see Google's own local names rather than machine translations of proper nouns.

What a person **reads** (`profiles.primary_language`, `secondary_language`) is separate from what they can **speak** (`user_languages`, used for matching).

## City packs (location independence)

A city is defined entirely by data in `data/citypacks/<city>.json`: areas with coordinates, languages, travel modes, and activity overrides. `cmd/tool seed` validates it and loads it in a single transaction; `tool build-matrix` and `tool discover-venues` then fill its travel times and venues from Google Maps. Adding a city means adding a JSON file and running those three commands; no code changes. `austin.json` (car-based) exists to prove this. See [../data/README.md](../data/README.md).

## Code generation

| Source | Generator | Output |
|---|---|---|
| `proto/**/*.proto` | buf → protoc-gen-go, protoc-gen-connect-go | `backend/gen/issho/…` |
| `proto/**/*.proto` | buf → protoc-gen-es | `packages/api-client/src/gen/…` |
| `proto/**/*.proto` | buf → protoc-gen-doc | `docs/generated/api.md` |
| `backend/db/queries/*.sql` + migrations | sqlc | `backend/gen/dbgen` |

Run `make gen` after editing protos or queries. CI fails if the generated code is stale.

## Key choices

See the decision log in [ROADMAP.md](ROADMAP.md#4-decision-log). The most important ones:
- Supabase for the database, auth and storage
- Google Maps Platform for venues and travel times
- Gemini on Google Cloud's Agent Platform for interest tagging and for composing walks (titles, summaries and notes included)
- Kamal 2 on a VPS for deploys
- Conventional Commits with release-please for versions and changelogs
