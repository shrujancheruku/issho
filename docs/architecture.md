# Architecture

## System overview

```
┌──────────────────────┐   ConnectRPC (POST, JSON)   ┌───────────────────────────────┐
│ apps/mobile (Expo)   │ ──────────────────────────▶ │ backend/cmd/api (Go)          │
│ admin SPA (planned)  │  Authorization: Bearer JWT  │  ├ auth interceptor (JWKS)    │
└─────────┬────────────┘                             │  ├ internal/<domain> handlers │
          │ sign-in (supabase-js)                    │  └ pgx pool ─────────────┐    │
          ▼                                          └──────────────────────────┼────┘
┌──────────────────────┐   JWKS (public keys)                                  │
│ Supabase Auth        │ ◀─────────────────────────── API fetches and caches  │
└──────────────────────┘                                                       ▼
                                                     ┌───────────────────────────────┐
  backend/cmd/worker (planned, River jobs) ────────▶ │ Postgres + PostGIS (Supabase) │
                                                     │  schema `app` (ours)          │
                                                     │  schema `auth` (Supabase)     │
                                                     └───────────────────────────────┘
```

- **Clients never talk to the database directly.** The `app` schema is revoked from Supabase's `anon` and `authenticated` roles (see migration `00001`). All reads and writes go through Go services. PostgREST and Realtime are unused.
- **Supabase Auth issues the identity.** The app signs in with Supabase and gets a short-lived access JWT. The Go API verifies that JWT against the project's public JWKS. It never holds a signing secret.
- **The app user row is created lazily.** The first authenticated `GetMe` creates `app.users` with the same UUID as `auth.users`. On Supabase, a foreign key ties the two together and cascades deletes.

## Request lifecycle

1. The client calls, for example, `POST /issho.v1.IdentityService/GetMe` with JSON and `Authorization: Bearer <token>`.
2. **CORS** (`cmd/api`) checks the origin. Only browser origins in `ALLOWED_ORIGINS` pass. Native apps send no Origin header.
3. The **auth interceptor** (`internal/platform/auth`) runs:
   - Public procedures (currently only `HealthService.Check`) skip it.
   - Everything else must have a valid ES256/RS256 token with Supabase's issuer, the `authenticated` audience and an unexpired `exp`. The caller is put on the request context.
4. The **handler** (`internal/<domain>/handler.go`) validates input and calls the domain **service**.
5. The **service** contains the business logic and talks to Postgres through **sqlc** queries (`backend/gen/dbgen`).
6. Errors come back as Connect error codes (`unauthenticated`, `invalid_argument`, …).

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
- **`internal/matching` (M4) will be pure:** plain structs in, proposals out, no I/O. This makes it deterministic and easy to simulate.
- **Every external vendor sits behind a Go interface**, so implementations can be swapped. Planned interfaces: `VenueProvider`, `TravelTimeProvider`, `Tagger`, `Copywriter`, `Verifier`, `Push`, `Storage`.

## Data model

Defined in `backend/db/migrations/00001_init.sql`. All app tables live in the `app` schema.

| Table | Purpose |
|---|---|
| `cities` | One row per city pack: timezone, currency, default languages, matching-weight overrides |
| `areas` | The unit a user chooses as "home" (a neighbourhood or station), with its centre as PostGIS `geography` |
| `area_travel_times` | Precomputed area-to-area minutes per mode (filled by `tool build-matrix`, M3) |
| `activities` | The global catalogue. `name` is JSON keyed by language (`{"en": …, "ja": …}`) |
| `city_activity_overrides` | A city can rename, disable or set seasonal months for an activity |
| `users` | App account; `id` = Supabase `auth.users.id` |
| `profiles` | Matching preferences (home area, travel time, group size, gender rules, …) |
| `user_languages` | Fluent languages, and languages being learned with a level |
| `availability_slots` | Weekday × morning/afternoon/evening |
| `user_activity_prefs` | Weight per activity: 0 = open to try, 1 = like, 2 = love |

Localised text is stored as JSON objects and resolved with `i18n.Pick`: exact locale → base language → English → any value.

## City packs (location independence)

A city is defined entirely by data in `data/citypacks/<city>.json`: areas with coordinates, languages, and activity overrides. `cmd/tool seed` loads it in a single transaction. Adding a city means adding a JSON file and seeding it; no code changes. See [../data/README.md](../data/README.md).

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
- Claude for interest tagging and plan text
- Kamal 2 on a VPS for deploys
- Conventional Commits with release-please for versions and changelogs
