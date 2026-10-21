# backend

The Go backend for Issho: a modular monolith serving a ConnectRPC API. It uses Postgres (Supabase) through pgx and sqlc, and goose for migrations. Architecture notes are in [../docs/architecture.md](../docs/architecture.md).

## Layout

| Path | Contents |
|---|---|
| `cmd/api/` | API server: wires config, DB pool, the JWKS verifier and handlers, then serves HTTP/1.1 and h2c on `API_ADDR` |
| `cmd/tool/` | Ops CLI: `migrate <goose command>`, `seed [-catalog file] [-venue-categories file] <citypack.json>...`, `build-matrix [-dry-run] <city>`, `discover-venues [-dry-run] [-categories ids] <city>`, `simulate [-users n] [-seed n] [-month n] [-config json] <citypack.json>...` (no database needed) |
| `internal/platform/config` | Environment variables → `Config` |
| `internal/platform/db` | pgx pool and goose migration runner |
| `internal/platform/auth` | Supabase JWT verification (JWKS) and the Connect auth interceptor |
| `internal/platform/i18n` | Language rules: which primary/secondary pair to show (`ForEvent`, `ForApp`), what to display from resolved text (`Pair`), ISO 639-1 validation |
| `internal/translate` | Text in any language on demand: stored text → `app.translations` cache → `Translator`, with in-flight de-duplication, a bounded wait and background completion; `Prefill` for content that must exist in given languages (events) |
| `internal/translate/google` | `Translator` using Google Cloud Translation Basic (v2): plain-text mode, batches of ≤ 128 strings and ~5,000 characters |
| `internal/platform/googleapi` | Shared transport for key-based Google APIs (Maps, Translation): key in a header, retries on 429/5xx |
| `internal/platform/errs` | Connect interceptor: logs unexpected errors and returns a generic `internal error` (no leaked details) |
| `internal/platform/supabase` | Supabase admin API client (account deletion), using the secret key |
| `internal/health` | `HealthService` |
| `internal/identity` | `IdentityService`: app account on top of Supabase users (get, delete) |
| `internal/catalog` | `CatalogService`: activity catalogue with city overrides |
| `internal/cities` | City-pack, catalogue and venue-category JSON formats; validation and seeding |
| `internal/maps` | Provider-neutral geo interfaces (`Router` for travel-time matrices, `PlaceSearcher` for nearby places) |
| `internal/maps/google` | Google implementation: Routes API (Compute Route Matrix) and Places API (New) Nearby Search over REST, with retries on 429/5xx. `placetypes.txt` is a snapshot of valid place types. |
| `internal/matching` | The matching engine (M4), pure: no I/O, deterministic. `Run` chains `Buckets` → `Plans` → `Assemble`; `Violation` checks a group's hard rules; `ParseConfig` reads a city's overrides. See [Matching](#matching) |
| `internal/matching/sim` | Synthetic users for any city pack (estimated travel times, made-up venues), and checks of the engine's output |
| `internal/geo` | Builds a city's travel-time matrix and venue list through the `maps` interfaces, and stores them |
| `db/migrations/` | goose SQL migrations (embedded via `db/embed.go`) |
| `db/queries/` | sqlc queries |
| `gen/` | **Generated.** `gen/issho` (protobuf/Connect), `gen/dbgen` (sqlc). Don't edit; run `make gen`. |

## Configuration

| Variable | Required | Example (local) | Notes |
|---|---|---|---|
| `DATABASE_URL` | yes | `postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable` | Supabase local DB. In production, use the **session pooler** (port 5432) or the direct connection, with `sslmode=require`. **Not** the transaction pooler (port 6543): pgx's prepared statements and River's LISTEN/NOTIFY don't work through it. |
| `SUPABASE_URL` | yes | `http://127.0.0.1:54321` | The JWKS is fetched from `$SUPABASE_URL/auth/v1/.well-known/jwks.json`. The token issuer must be `$SUPABASE_URL/auth/v1`. |
| `API_ADDR` | no | `:8080` | Listen address |
| `SUPABASE_SECRET_KEY` | no | `SECRET_KEY` from `supabase status` | **Server-only secret.** Enables admin calls (account deletion). Without it, `DeleteAccount` returns `failed_precondition`. Never commit it; see [secrets](../docs/secrets.md). |
| `GOOGLE_MAPS_API_KEY` | only for `tool build-matrix` / `discover-venues` | (empty) | **Server-only secret**, restricted to the Routes and Places APIs. Not needed for `-dry-run`, tests or the API server. See [google-maps](../docs/google-maps.md). |
| `GOOGLE_TRANSLATE_API_KEY` | no | (empty) | **Server-only secret**, restricted to the Cloud Translation API. Enables on-demand translation. Without it, text is shown in the languages already stored (English, local names, cached translations). See [google-maps](../docs/google-maps.md#6-cloud-translation). |
| `ALLOWED_ORIGINS` | no | `https://admin.example.com` | Comma-separated browser origins. **Empty = no browser origins allowed.** |

Local defaults are in `../.env.example`: `export $(grep -v '^#' ../.env.example | xargs)`.

## Run

```sh
supabase start                 # from repo root; see ../supabase/README.md
make migrate-up seed api       # from repo root
```

Or from this directory: `go run ./cmd/tool migrate up`, `go run ./cmd/tool seed ../data/citypacks/tokyo.json`, `go run ./cmd/api`.

## API endpoints

The full reference, generated from the proto comments, is in **[../docs/generated/api.md](../docs/generated/api.md)**. All RPCs are `POST /<package>.<Service>/<Method>` with a JSON body (`Content-Type: application/json`) or binary protobuf.

| Endpoint | Auth | Purpose |
|---|---|---|
| `POST /issho.v1.HealthService/Check` | public | Database ping and build version |
| `POST /issho.v1.IdentityService/GetMe` | bearer | Returns (and lazily creates) the caller's account. `unauthenticated: account deleted` for tokens of deleted users. |
| `POST /issho.v1.IdentityService/DeleteAccount` | bearer | Permanently deletes the caller (Supabase auth user → cascades to app data). Safe to retry. |
| `POST /issho.v1.CatalogService/ListActivities` | bearer | Activities enabled in a city: `name` in `locale` (required, the user's primary language) and `secondaryName` in the optional `secondaryLanguage`. Any ISO 639-1 code; names are stored in English and translated on demand (the first request for a language can take a few seconds and may return English for names not ready yet). Other codes → `invalid_argument`. |

Example:
```sh
curl -s -X POST localhost:8080/issho.v1.CatalogService/ListActivities \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" \
  -d '{"cityId":"tokyo","locale":"ja","secondaryLanguage":"es"}'
# → {"activities":[…, {"id":"pottery","name":"陶芸","secondaryName":"Cerámica", …}]}
```

Errors use Connect codes, e.g. `{"code":"unauthenticated","message":"missing bearer token"}` with HTTP 401.

**Error hygiene:** handlers return `connect.NewError(code, err)` for anything the caller should see. Any other error is logged server-side (`unhandled error` with the procedure name) and the client receives only `{"code":"internal","message":"internal error"}`. So a raw database or upstream error never leaks to callers.

### Authentication
- Clients send `Authorization: Bearer <Supabase access token>`.
- Tokens must:
  - be signed **ES256 or RS256** with a key from the project JWKS (HS256 is rejected)
  - have issuer `$SUPABASE_URL/auth/v1`
  - have audience `authenticated`
  - have an `exp` claim that hasn't passed
- **Deny-by-default:** every RPC requires auth unless it's listed as public in `cmd/api/main.go`.

## Migrations

```sh
go run ./cmd/tool migrate up        # apply all
go run ./cmd/tool migrate down      # roll back one
go run ./cmd/tool migrate status
```

- Write new files as `db/migrations/000NN_name.sql` with `-- +goose Up` / `-- +goose Down` sections. Wrap multi-statement blocks (`DO $$ … $$`) in `-- +goose StatementBegin/End`.
- goose records its versions in `goose.db_version`, in a schema of its own. Not `public`, which Supabase's Data API would expose. Not `app`, which migration 00001's Down drops. Older databases with `public.app_goose_db_version` are moved automatically on the next run.
- Supabase-only statements (the `auth.users` foreign key, revoking `anon`) are guarded with `IF EXISTS` checks, so the same migrations also run on plain Postgres in CI.

## Seeding

`cmd/tool seed` validates and upserts `../data/venue-categories.json` and `../data/activities.json`, then each city pack given (`make seed` loads them all). It's idempotent. Note that removing an area from the JSON does **not** delete it from the database. See [../data/README.md](../data/README.md).

## Geo data (Google Maps)

Set up keys first: [../docs/google-maps.md](../docs/google-maps.md). Then, for a seeded city:

```sh
go run ./cmd/tool build-matrix -dry-run tokyo     # plan only: requests and billed elements
go run ./cmd/tool build-matrix tokyo              # fills app.area_travel_times
go run ./cmd/tool discover-venues -dry-run tokyo
go run ./cmd/tool discover-venues -categories cafe,park tokyo
```

- **`build-matrix`** computes travel times between every pair of the city's areas, for each of the city's `travel_modes`. Same-area pairs are stored as 0 minutes without an API call. Only walking and driving are calculated (no public transport). Walking is only requested for areas less than 4 km apart; driving uses typical, traffic-unaware times. Minutes are rounded up. Pairs with no route are left out. Rows older than 30 days are deleted (Google caching terms), so **rebuild each city at least every 30 days**.
- **`discover-venues`** runs a Nearby Search per area × venue category, for categories used by an activity enabled in the city. Only places whose **primary** type is one of the category's types are kept. Categories with a radius of 10 km or more are searched once from the city centre. Each place is assigned to its **nearest** area and stored as `google_place_id` + category. Re-running only bumps `last_seen_at`; curation status and tags are kept.

## Matching

`internal/matching` turns one city's users into a week of proposals. The caller loads everything (users without an event that week, approved venues, enabled and in-season activities, the fastest travel time per area pair, mutual picks and blocks) and stores the result; the engine itself does no I/O.

1. **Buckets:** every area is a candidate meeting place. A person joins each area they can reach within `max_travel_min`, for each weekly slot they're free, in each language they speak fluently. Two-language buckets exist only for the city's `language_pairs`, and anyone fluent in either language joins.
2. **Plans:** activities that suit the slot's part of the day (`activities.blocks`), have a venue in the area (or need only a meeting point), and interest enough members. Ranked by demand (love 2, like 1, open to try 0.5; × 0.3 if over budget) × venue choice. A non-conversational activity gets a food follow-up.
3. **Assembly:** best candidates first. Each is filled greedily, then a local search replaces and swaps people while the total score rises. Score: interest, size fit, one known pair, newcomers mixed in (not a majority), reliability.
4. **Invitations:** each group invites its target size plus 2 (`extra_invites`). `Quorum` is the target (lower if fewer could be invited, never below 3).

Hard rules (`Violation`): no blocked pairs; same-gender-only is respected; nobody is the only woman or the only man; at most 2 people who already know each other; groups of at least 3 (`min_group_size`). Each person is invited to at most one proposal per run.

Settings are in `matching.Config` (`DefaultConfig`). A city overrides any of them with JSON in `app.cities.matching_weights`, e.g. `{"language_pairs": [["ja", "en"]], "weights": {"mutual": 2}}`. Unknown keys are rejected.

```sh
make sim                                           # 500 synthetic users per city pack
make sim ARGS="-users 1000 -seed 2 -config '{\"language_pairs\":[[\"ja\",\"en\"]]}'"
```

The simulator reports how many people were invited (overall, per gender and language), group sizes, top activities and any rule violations, which fail the run. `go test ./internal/matching/sim` runs it for every pack in CI.

## Testing

```sh
go test ./...          # or `make test` from the root
```

| Test | Covers |
|---|---|
| `internal/platform/auth/auth_test.go` | Valid token; expired, missing `exp`, wrong issuer/audience/key/subject; HS256 rejected |
| `internal/platform/i18n/i18n_test.go` | Event and app language pairs (JP–ES exchange, single-language events, any language), text pairs (missing primary shows the secondary alone, identical text shown once, last-resort fallback), code validation |
| `cmd/api/cors_test.go` | CORS allowlist; empty list denies all |
| `internal/platform/errs/errs_test.go` | Raw errors become generic `internal`; Connect errors pass through |
| `internal/platform/supabase/admin_test.go` | Admin delete: request shape, 404 treated as already deleted, other errors surfaced |
| `internal/identity/service_test.go` | Account creation keeps an existing email; deletion order (auth first, app data kept if that fails) |
| `internal/maps/google/google_test.go` | Matrix chunking (≤ 625 elements per request), global origin indexes, no-route and element errors, drive is traffic-unaware, key only in a header, field masks, retry on 429 but not on 400 |
| `internal/geo/geo_test.go` | Matrix plan (no self pairs, no long walks), rounding up, missing routes left out, venue plan (wide categories once), nearest-area assignment, dedupe |
| `internal/geo/store_test.go` | **Integration** (runs when `TEST_DATABASE_URL` is set; CI sets it): area coordinates round-trip, upsert, 30-day stale deletion, one venue in two categories, curation kept on re-discovery |
| `internal/cities/data_test.go` | Every file in `../data` is valid: English-only catalogue, venue category references, Google place types, city packs (area ids, modes, months, override ids, names only in English or the city's own languages) |
| `internal/translate/translate_test.go` | Stored language wins, cache hits skip the translator, duplicates translated once, concurrent requests share one call, slow translations finish in the background after the request is gone, failures aren't cached, batching keeps positions, prefill fails loudly (run with `-race`) |
| `internal/translate/google/google_test.go` | Key only in a header, source/format sent, batching by segment count and size, HTML entities unescaped, API errors and wrong result counts fail |

CI also runs migrations up → down → up, seeds every city pack, runs the geo integration test and dry-runs both geo tools against a PostGIS container. Tests must never call paid external APIs. Run the integration test locally with `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/geo/ -run Integration`; it creates and removes its own throwaway city.

## Code generation

```sh
make gen   # from root: buf (Go + TS + docs) and sqlc
```

The code generators are pinned as `tool` directives in `go.mod`, and `../go.work` makes `go tool` work from the repo root.
