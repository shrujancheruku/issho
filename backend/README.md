# backend

The Go backend for Issho: a modular monolith serving a ConnectRPC API. It uses Postgres (Supabase) through pgx and sqlc, and goose for migrations. Architecture notes are in [../docs/architecture.md](../docs/architecture.md).

## Layout

| Path | Contents |
|---|---|
| `cmd/api/` | API server: wires config, DB pool, the JWKS verifier and handlers, then serves HTTP/1.1 and h2c on `API_ADDR` |
| `cmd/worker/` | Background jobs on River (`internal/worker`): starts the River client and stops it gracefully on SIGINT/SIGTERM |
| `cmd/tool/` | Ops CLI: `migrate <goose command>`, `seed [-catalog file] [-venue-categories file] <citypack.json>...` (also applies catalogue merges: `replaces`), `build-matrix [-dry-run] <city>`, `discover-venues [-dry-run] [-categories ids] <city>`, `email-templates [-source file] [-out dir] [-config file]` (sign-in emails in every language of `supabase/templates/source.json`, machine-translated; needs `GOOGLE_TRANSLATE_API_KEY`), `simulate [-users n] [-seed n] [-month n] [-config json] <citypack.json>...` (no database needed) |
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
| `internal/catalog` | `CatalogService`: activity catalogue with city overrides, cities and areas, language names (CLDR via `golang.org/x/text`) |
| `internal/profile` | `ProfileService`: the caller's own profile (validation in `validate.go`, one-transaction save in `service.go`); `ConductVersion` |
| `internal/tagging` | Free-text interests → suggested activities (`Service.Suggest`: input limits, city catalogue, drops unknown ids, keeps the strongest level per activity) behind the `Tagger` interface |
| `internal/tagging/gemini` | `Tagger` using Gemini (`gemini-3.5-flash-lite`) on Google Cloud's **Agent Platform** (formerly Vertex AI): `generateContent` over REST with a Google Cloud API key, via `internal/platform/googleapi`. A response schema that only allows the city's activity ids, low thinking, 20 s deadline; blocked or safety-stopped answers count as "no suggestion". `testdata/eval.json` + `eval_test.go` is the multilingual eval set |
| `internal/emailtemplates` | Builds the multilingual Supabase sign-in email templates and subjects from one English source: branches on `.Data.lang`, English fallback, escaping and checks that no translation can break the template or `config.toml` |
| `internal/uistrings` | `UIService`: the app's UI text in any language. `en.json` is a copy of `apps/mobile/src/i18n/en.json` made by `make gen`; only those strings are translated |
| `internal/cities` | City-pack, catalogue and venue-category JSON formats; validation and seeding |
| `internal/maps` | Provider-neutral geo interfaces (`Router` for travel-time matrices, `PlaceSearcher` for nearby places) |
| `internal/maps/google` | Google implementation: Routes API (Compute Route Matrix), Places API (New) Nearby Search and Place Details (`Details`: localised name, short address, Maps link, location) over REST, with retries on 429/5xx. `placetypes.txt` is a snapshot of valid place types. |
| `internal/matching` | The matching engine (M4), pure: no I/O, deterministic. `Suggest` (plans without a date), `Feed` (plans ranked for one person, with map levels), `Schedule` (date and groups from signups); `Violation` checks a group's hard rules; `ParseConfig` reads a city's overrides. See [Matching](#matching) |
| `internal/matching/sim` | Synthetic users for any city pack (estimated travel times, made-up venues, guessed signup behaviour); runs suggest → signups → schedule and checks the output |
| `internal/worker` | River jobs: `match_all` (periodic, hourly, and once on start) queues a `match_city` pass per city, which runs `plans.Runner.Schedule` then `Suggest`. One pass per city is queued or running at a time (unique jobs). River's tables live in the `river` schema |
| `internal/plans` | `PlanService` (`handler.go`, `service.go`: the caller's feed, a plan, signing up and withdrawing), `Runner` (the worker's passes: `Schedule` checks every open plan, `Suggest` opens plans up to the city's cap) and `LoadCity` (a city's matching input from the database), plus the store. Stores open plans, signups and scheduled events (M6) and converts them to and from `internal/matching`'s types: `Create`, `ListOpen` (with signup counts), `SaveSignup` (times replaced wholesale), `LoadSignups` (adds the occasions people are already invited to, so nobody is double-booked), `SaveEvent` (event, groups and invitations; invited people stop waiting), `NextRound`, `Close`. Saves lock the plan row, so a signup lands either before a round is scheduled or in the next one |
| `internal/geo` | Builds a city's travel-time matrix and venue list through the `maps` interfaces, and stores them |
| `db/migrations/` | goose SQL migrations (embedded via `db/embed.go`) |
| `db/queries/` | sqlc queries |
| `gen/` | **Generated.** `gen/issho` (protobuf/Connect), `gen/dbgen` (sqlc). Don't edit; run `make gen`. |

## Configuration

| Variable | Required | Example (local) | Notes |
|---|---|---|---|
| `DATABASE_URL` | yes | `postgres://postgres:postgres@127.0.0.1:54322/postgres?sslmode=disable` | Supabase local DB. In production, use the **session pooler** (port 5432) or the direct connection, with `sslmode=require`. **Not** the transaction pooler (port 6543): pgx's prepared statements and River's LISTEN/NOTIFY don't work through it. |
| `WORKER_DATABASE_URL` | worker only; falls back to `DATABASE_URL` | same as `DATABASE_URL` | The worker's connection. River needs LISTEN/NOTIFY: direct connection or session pooler only. |
| `GOOGLE_MAPS_API_KEY` (worker) | no | (empty) | The worker uses it to rebuild travel times before the 30-day Maps limit and to check venues are open before scheduling. Without it, travel times past 30 days are deleted (not rebuilt) and hours aren't checked. |
| `EXPO_ACCESS_TOKEN` | worker; only with enhanced push security | (empty) | **Secret.** Sent to Expo's push service when the Expo project requires it. |
| `GOOGLE_TRANSLATE_API_KEY` (worker) | no | (empty) | The worker also uses it, for push notifications in each person's language; without it they're in English. |
| `SUPABASE_URL` | yes | `http://127.0.0.1:54321` | The JWKS is fetched from `$SUPABASE_URL/auth/v1/.well-known/jwks.json`. The token issuer must be `$SUPABASE_URL/auth/v1`. |
| `API_ADDR` | no | `:8080` | Listen address |
| `SUPABASE_SECRET_KEY` | no | `SECRET_KEY` from `supabase status` | **Server-only secret.** Enables admin calls (account deletion). Without it, `DeleteAccount` returns `failed_precondition`. Never commit it; see [secrets](../docs/secrets.md). |
| `GOOGLE_MAPS_API_KEY` | for `tool build-matrix` / `discover-venues`; optional for the API | (empty) | **Server-only secret**, restricted to the Routes and Places APIs. Not needed for `-dry-run` or tests. The API uses it for venue names and addresses on plans (Place Details, fetched live); without it, plans show no venue names. See [google-maps](../docs/google-maps.md). |
| `GOOGLE_GEMINI_API_KEY` | no | (empty) | **Server-only secret**: a Google Cloud API key for Agent Platform (created in Agent Platform; it starts `AQ.`). It only works on `aiplatform.googleapis.com`, not the Gemini Developer API (`API_KEY_SERVICE_BLOCKED`). Enables interest suggestions (`CatalogService.SuggestActivities`); without it, that RPC returns `failed_precondition` and the app says suggestions aren't available. |
| `GOOGLE_TRANSLATE_API_KEY` | no | (empty) | **Server-only secret**, restricted to the Cloud Translation API. Enables on-demand translation. Without it, text is shown in the languages already stored (English, local names, cached translations). See [google-maps](../docs/google-maps.md#6-cloud-translation). |
| `ALLOWED_ORIGINS` | no | `https://admin.example.com` | Comma-separated browser origins. **Empty = no browser origins allowed.** |

Local defaults are in `../.env.example`: `export $(grep -v '^#' ../.env.example | xargs)`.

## Run

```sh
supabase start                 # from repo root; see ../supabase/README.md
make migrate-up seed api       # from repo root
make worker                    # background jobs (another terminal)
```

Or from this directory: `go run ./cmd/tool migrate up`, `go run ./cmd/tool seed ../data/citypacks/tokyo.json`, `go run ./cmd/api`.

## API endpoints

The full reference, generated from the proto comments, is in **[../docs/generated/api.md](../docs/generated/api.md)**. All RPCs are `POST /<package>.<Service>/<Method>` with a JSON body (`Content-Type: application/json`) or binary protobuf.

| Endpoint | Auth | Purpose |
|---|---|---|
| `POST /issho.v1.HealthService/Check` | public | Database ping and build version |
| `POST /issho.v1.IdentityService/GetMe` | bearer | Returns (and lazily creates) the caller's account: `onboarded` and, once onboarded, the display languages. `unauthenticated: account deleted` for tokens of deleted users. |
| `POST /issho.v1.IdentityService/DeleteAccount` | bearer | Permanently deletes the caller (Supabase auth user → cascades to app data). Safe to retry. |
| `POST /issho.v1.UIService/GetStrings` | public | The app's UI text in `language` (any ISO 639-1 code), machine-translated on demand and cached. Strings still being translated, or whose `{placeholders}` didn't survive, are left out (the app shows English); `complete: false` means ask again later. Public because the sign-in screens need it. |
| `POST /issho.v1.ProfileService/GetProfile` | bearer | The caller's profile and the current code-of-conduct version. `not_found` before onboarding. |
| `POST /issho.v1.ProfileService/SaveProfile` | bearer | Creates or replaces the whole profile (languages, availability and activity preferences included) in one transaction. `invalid_argument` names the field (e.g. `birth_year: must be at least 18`); `failed_precondition` if `GetMe` hasn't created the account. |
| `POST /issho.v1.CatalogService/ListCities` | bearer | Cities with their areas. City names in `locale` + `secondaryLanguage` (translated); area names only as stored (English plus a real local name such as 吉祥寺), never machine-translated. |
| `POST /issho.v1.CatalogService/ListLanguages` | bearer | Every ISO 639-1 language: its own name (`autonym`) and its name in `locale`, from CLDR. |
| `POST /issho.v1.CatalogService/SuggestActivities` | bearer | Suggests activities offered in `cityId` from free `text` (any language, 1–500 characters), each with an interest level, using Google's Gemini API. Suggestions only: the app shows them for the person to confirm. Empty list = nothing matched (or the model declined). `failed_precondition` without `GOOGLE_GEMINI_API_KEY`. |
| `POST /issho.v1.PlanService/ListFeed` | bearer | Every open plan in the caller's city, best for them first (`matching.Feed`), each with a `title` summarising the outing ("Climbing and bouldering, then lunch or dinner out", composed in English and translated into the caller's two display languages; area names are left out because they're never machine-translated), a `level` (top/good/maybe/low/not-for-you) for the feed and map, the signup count (never who), travel minutes from their home area (-1 = no route), the round's window and signup deadline, and `signedUp`. Ids only: names come from `CatalogService`. `failed_precondition` before onboarding. |
| `POST /issho.v1.PlanService/GetPlan` | bearer | One open plan as in the feed, its `steps` (the activity, then any follow-up, each with its venue's Google name in both display languages, short address and Maps link, fetched live, plus an estimated walk from the previous stop), and the caller's `signup` (their weekly pattern and blocked-out dates) if they've signed up. `not_found` for unknown or closed plans and plans in another city. |
| `POST /issho.v1.PlanService/SignUp` | bearer | Signs up for a plan or replaces the times given before. An empty `pattern` means the profile's usual availability. `invalid_argument` for malformed times or dates outside the window; `failed_precondition` when the caller doesn't speak the plan's language; `not_found` once it's closed. |
| `POST /issho.v1.PlanService/Withdraw` | bearer | Removes the caller's signup. Safe to repeat. |
| `POST /issho.v1.EventService/ListMyEvents` | bearer | The caller's events from yesterday on, soonest first: title, date, start time, answer deadline, event and group status, their answer, how many confirmed (never who), and the meeting point once revealed. |
| `POST /issho.v1.EventService/GetEvent` | bearer | One of the caller's events with its stops (as `GetPlan`) and, only if they confirmed, the first names of everyone confirmed in their group. `not_found` unless invited. |
| `POST /issho.v1.EventService/RespondToEvent` | bearer | `CONFIRM` (invited or declined → confirmed) or `DECLINE` until the deadline; `CANCEL` a confirmed place until the start. A group is confirmed as soon as `quorum` confirm; a cancellation below `min_attendance` sends it back to collecting answers, or cancels it after the deadline. `failed_precondition` when too late, cancelled or not applicable. |
| `POST /issho.v1.PushService/RegisterPushToken` | bearer | Stores the device's Expo push token (`ExponentPushToken[…]`) for the caller; a token registered by another account moves to the caller. `invalid_argument` for anything that isn't an Expo push token. |
| `POST /issho.v1.PushService/UnregisterPushToken` | bearer | Stops notifications to that device (only the caller's own token). |
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
- **Deny-by-default:** every RPC requires auth unless it's listed as public in `cmd/api/main.go` (`HealthService.Check`, `UIService.GetStrings`).

## Migrations

```sh
go run ./cmd/tool migrate up        # apply all
go run ./cmd/tool migrate down      # roll back one
go run ./cmd/tool migrate status
```

- Write new files as `db/migrations/000NN_name.sql` with `-- +goose Up` / `-- +goose Down` sections. Wrap multi-statement blocks (`DO $$ … $$`) in `-- +goose StatementBegin/End`.
- goose records its versions in `goose.db_version`, in a schema of its own. Not `public`, which Supabase's Data API would expose. Not `app`, which migration 00001's Down drops. Older databases with `public.app_goose_db_version` are moved automatically on the next run.
- **River's tables** (the worker's job queue) are created by `00008_river.go` (`db/river.go`), a Go migration that runs River's own migrator up to `RiverVersion` in the `river` schema; Down drops the schema. When a River upgrade brings a new schema version, add a goose migration that migrates up to it, so goose stays the one owner of migration order.
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

## Plans, signups and events

A **plan** (`app.plans`) is open for signups in rounds, each with a window (default 14 days) and a signup deadline. **Signups** (`app.plan_signups`, with `plan_signup_slots` for a custom weekly pattern and `plan_signup_unavailable` for blocked-out dates or occasions) are the people waiting for the current round. Scheduling a round creates an **event** (`app.events`, one per round) with one or more **groups** (`app.event_groups`), each inviting people (`app.group_members`: invited → confirmed/declined, confirmed → dropped). Invited people leave the signups; the rest roll over to the next round. A person is in at most one group per event (enforced by the database). Each plan names its venues when it opens (migration 00009: `venue_place_id`, `follow_up_venue_place_id`; approved venues first, never rejected ones, otherwise random so one café isn't in every plan); the groups of an event share them, and `event_groups.venue_place_id` is only for a group sent elsewhere. Only Google place ids are stored. Plans close as `done`, `no_group` or `withdrawn`.

**The worker** (`make worker`) runs a pass per city every hour (`plans.Runner`, timing in `plans.Timing`):

- **Travel-time refresh** (`refresh_maps`, daily, also on start): `geo.Refresh` rebuilds a city's matrix once it's 25 days old (`geo.RefreshAge`), replacing it whole so nothing outlives the 30-day Maps caching limit; without a Maps key it deletes rows past 30 days instead.
- **Opening hours** (in **Schedule**, below): before a round is saved, the plan's venues are checked (Place Details hours, once per venue per pass): the activity's venue must be open at the start and 90 minutes in, a follow-up's 2–3 hours after the start (activities have no stored duration). A closed time is taken out of the window and the engine schedules again; a venue closed for good (or until further notice) withdraws the plan, and a permanently closed one is marked `rejected` so it's never picked again. Lookup failures don't block scheduling.
- **Push notifications** (`plans.Notifier`, a `send_notifications` job every minute): the changes that notify people write rows to `app.notifications` in the same transaction (an outbox: invited when a round is scheduled; group confirmed, at quorum or at the deadline; group cancelled), and the job sends them through Expo (`internal/push`) to every device the person registered: the plan's title, then a line from the UI catalogue in their language (`push.*` keys; no dates, which Go can't format per language). Tokens Expo reports as `DeviceNotRegistered` are deleted; if Expo can't be reached, the batch is retried on later passes (5 attempts).

0. **Settle** groups whose confirmation deadline has passed: confirmed with at least `min_attendance`, otherwise cancelled; the people who had confirmed a cancelled group go back on the plan's waiting list (if it's still open), and an event with no group left is cancelled. Events whose date has passed are marked `done`.
1. **Schedule** every open plan, most signups first (each event blocks its occasion for the people invited). Occasions start 2 days from today (time to confirm). Ready plans get an event; at the signup deadline (7 days into the 14-day window) the round goes ahead with whatever groups can form, or the plan closes as `no_group`. People not placed roll over to the next round, which starts in the same transaction; a plan with nobody left closes as `done`. A plan whose activity was disabled closes as `withdrawn`. Invitees answer by the start of the day before the event (local time). Events start at a set time for their part of the day (`Timing.BlockStarts`: 10:00, 14:00, 18:30 local), and their meeting point is shown to confirmed people from 24 hours before.
2. **Suggest** new plans up to the city's cap, with the activities in season in the month the round starts. Plans that closed as `no_group` in the last 14 days count as open, so they don't reopen the next hour.

Not done yet: chat, push notifications, the travel-matrix refresh and AI-written plan text.

## Matching

`internal/matching` runs **plan first, time second**: suggest plans without a date, rank them for each person, then schedule a plan once its signups can make a good group. The caller (the M6 worker and API) loads everything (users, approved venues, activities enabled and in season, the fastest travel time per area pair, open plans and signups, mutual picks and blocks) and stores the results; the engine itself does no I/O.

1. **`Pools`:** every area is a candidate meeting place. A person joins each area they can reach within `max_travel_min`, in each language they speak fluently. Two-language pools exist only for the city's `language_pairs`, and anyone fluent in either language joins.
2. **`Suggest`:** for each pool, activities with a venue in the area (or needing only a meeting point) and an audience of at least `min_audience` people who want them and are usually free at a part of the day they suit (`activities.blocks`). Ranked by demand (love 2, like 1, open to try 0.5; × 0.3 if over budget) × venue choice, then taken best first across the city: not already open, at most `plans_per_pool` per pool, the same activity in a shared language at least `plan_spacing_min` minutes apart, at most one open plan per `users_per_plan` people. A non-conversational activity gets a food follow-up.
3. **`Feed`:** every open plan for one person, in levels (top = love, good = like, maybe = open to try, low = no preference or too far, not-for-you = doesn't speak its language) for the feed's order and the map's colours. Within a level: interest, momentum (signups so far), fit with their usual availability, then distance.
4. **Signups:** `Signup.Pattern` is the weekly slots that suit them for this plan (nil = profile availability) and `Unavailable` the dates or occasions they can't do. The caller adds the occasions of events they're already going to.
5. **`Schedule`:** for each occasion in the plan's window, form groups from the signups free then (greedy, then local search: fill, replace, swap, spread small groups); keep the best occasion, ties to the earliest. Ready when a group reaches its target size; groups below the preferred size wait for the next round. With `force` (the signup deadline) every group that can form goes ahead. Each group invites its target plus `extra_invites`; `Quorum` confirms straight away and `MinAttendance` (2) is the fewest it can go ahead with at the confirmation deadline.

Hard rules (`Violation`): no blocked pairs; same-gender-only is respected; at most 2 people who already know each other; at least 2 people (`min_group_size`). Removing people never breaks them, so they still hold after declines.

Preferences (in the group score, so the engine avoids them when it can but would rather go ahead than leave someone out): groups of at least 3 or the activity's own minimum (`preferred_min_size`, weight `min_size`); nobody the only woman or only man (weight `balance`; nonbinary and unspecified exempt). The only language rule is the event's: everyone speaks its primary or secondary language.

Settings are in `matching.Config` (`DefaultConfig`). A city overrides any of them with JSON in `app.cities.matching_weights`, e.g. `{"language_pairs": [["ja", "en"]], "users_per_plan": 15, "weights": {"mutual": 2}}`. Unknown keys are rejected.

```sh
make sim                                           # 500 synthetic users per city pack
make sim ARGS="-users 1000 -seed 2 -config '{\"language_pairs\":[[\"ja\",\"en\"]]}'"
```

The simulator runs the whole flow on synthetic users: suggest, signups from each person's feed (with varied availability), then scheduling (ready plans first, then the rest at their deadline), never double-booking anyone. It reports plans, signups, scheduling, group sizes, pairs, gender balance, who got invited by gender and language, and any rule violations, which fail the run. `go test ./internal/matching/sim` runs it for every pack in CI.

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
| `internal/identity/service_test.go` | Account creation keeps an existing email; onboarded and display languages once a profile exists; deletion order (auth first, app data kept if that fails) |
| `internal/profile/validate_test.go` | Every profile rule (age 18+ this year, a fluent language, ≥ 3 activities, same-gender-only needs a gender, current code of conduct, …), normalisation, proto round trip, unknown enums and duplicates rejected |
| `internal/profile/service_test.go` | **Integration** (`TEST_DATABASE_URL`): no account → `ErrNoAccount`; save, read back, a second save replaces every list; unknown areas and activities rejected; a failed save changes nothing |
| `internal/uistrings/uistrings_test.go` | The embedded catalogue parses and uses only `{name}` placeholders; English passes through; untranslated strings make the result incomplete; translations that lose a placeholder are dropped |
| `internal/tagging/tagging_test.go` | Input limits (blank, > 500 characters, no or unknown city), unavailable without a tagger, unknown ids and bad levels dropped, strongest level kept per activity |
| `internal/tagging/gemini/gemini_test.go` | Against a fake Agent Platform server: key only in a header, the `generateContent` path, low thinking, a schema whose enum is exactly the city's ids, catalogue in the system instruction, the person's text only in the user turn wrapped in `<interests>`, thought parts ignored; blocked prompts, safety stops and empty answers → no suggestion; `MAX_TOKENS`, HTTP errors and undecodable answers fail |
| `internal/tagging/gemini/eval_test.go` | **Eval, opt-in and paid** (`TAGGING_EVAL=1`, needs `GOOGLE_GEMINI_API_KEY`; well under $0.01, about 40 s): 30 cases in 8 languages, scoring recall (≥ 90%), interest levels (≥ 80%), activities that must not appear (none) and texts that should match nothing, including a prompt-injection attempt. Never runs in CI. |
| `internal/emailtemplates/emailtemplates_test.go` | Executes the generated body and subject with Go's `text/template` and `html/template`, as Supabase Auth would: the stored language, an unknown language, no `lang`, no metadata and a non-string `lang` (English); right-to-left blocks, escaped translations, the code in each; translations with template or TOML syntax rejected; `config.toml` subjects replaced; the real `source.json` is valid |
| `internal/catalog/languages_test.go` | Every ISO 639-1 language has an autonym and a localised name (English where CLDR has none); area names are read only as stored |
| `internal/maps/google/google_test.go` | Matrix chunking (≤ 625 elements per request), global origin indexes, no-route and element errors, drive is traffic-unaware, key only in a header, field masks, retry on 429 but not on 400 |
| `internal/geo/geo_test.go` | Matrix plan (no self pairs, no long walks), rounding up, missing routes left out, venue plan (wide categories once), nearest-area assignment, dedupe |
| `internal/plans/run_test.go` | `Timing`: the window and deadline in the city's time zone (a UTC evening that's already the next day in Tokyo), occasions left at the deadline, the confirmation deadline, invalid timings. **Integration:** eight people who love one activity: `Suggest` opens one plan (the cap), nothing happens with no signups, all eight sign up and it's scheduled on the first Saturday afternoon with six invited (the activity's maximum) and two rolling over to round 2; the cap stops a second plan; at the deadline one person left can't form a group, so the plan closes as `no_group` and isn't suggested again during the cool-down |
| `internal/plans/hours_test.go` | **Integration:** people free only on Saturday afternoons and a café: open only in the morning, the plan waits (hours looked up once per pass); open in the afternoon, it's scheduled; closed for good, the next round is withdrawn and the café rejected |
| `internal/maps/maps_test.go` | `Hours.OpenAt`: ordinary days, closing time, a bar open past midnight on Saturday (the week wraps), 24/7, no hours known, temporarily closed |
| `internal/plans/events_test.go` | **Integration:** a group of four (quorum 3, minimum 2): two confirmations keep it collecting, the third confirms it; only confirmed people see names; the meeting point appears 24 h before, in the caller's language, only to people going; declining twice, cancelling an unconfirmed place and answering after the deadline are refused; cancelling at the minimum keeps the group, below it sends it back to collecting; at the deadline one confirmation cancels the group and the event and puts that person back on the waiting list; a second group settles as confirmed and is marked done the day after |
| `internal/plans/notify_test.go` | **Integration:** scheduling queues invitations; the sender delivers to each registered device with the title, text, channel and event id; an uninstalled device's token is deleted and a person with no device is finished; a second pass sends nothing; reaching quorum notifies the others but not the person who just confirmed; with Expo down they stay queued and go out on the next pass |
| `internal/push/expo_test.go` | Batches of 100, tickets in order with `DeviceNotRegistered`, the access token header, failure on HTTP errors |
| `internal/plans/service_test.go` | Signup proto conversion (whole days, round trip, bad weekdays, blocks, dates and duplicates rejected). **Integration:** the feed for someone who speaks the plan's language (top level, 0 minutes) and someone who doesn't (not for you, signup refused); no profile → `ErrNoProfile`; unknown plan; a blocked-out date outside the window rejected; signing up twice keeps one signup; the plan returns the times given; withdrawing; a closed plan refuses signups |
| `internal/plans/store_test.go` | **Integration** (`TEST_DATABASE_URL`): a plan through custom and default signups (re-saving replaces the times), scheduling (a stale round is refused; invited people stop waiting), a second plan seeing the event as busy, next round, close, signups refused once closed, withdrawing |
| `internal/geo/store_test.go` | **Integration** (runs when `TEST_DATABASE_URL` is set; CI sets it): area coordinates round-trip, upsert, rows a rebuild didn't write are deleted, one venue in two categories, curation kept on re-discovery; `Refresh`: a fresh matrix is left alone, a 26-day-old one is rebuilt (and a pair it no longer returns goes at once), without a router rows past 30 days are deleted |
| `internal/cities/data_test.go` | Every file in `../data` is valid: English-only catalogue, known categories (each with an app label, and no app label without a category), `replaces` only naming retired ids once, venue category references, Google place types, city packs (area ids, modes, months, override ids, names only in English or the city's own languages) |
| `internal/translate/translate_test.go` | Stored language wins, cache hits skip the translator, duplicates translated once, concurrent requests share one call, slow translations finish in the background after the request is gone, failures aren't cached, batching keeps positions, prefill fails loudly (run with `-race`) |
| `internal/translate/google/google_test.go` | Key only in a header, source/format sent, batching by segment count and size, HTML entities unescaped, API errors and wrong result counts fail |

CI also runs migrations up → down → up, seeds every city pack, runs the geo, profile and plans integration tests and dry-runs both geo tools against a PostGIS container. Tests must never call paid external APIs. Run the integration tests locally with `TEST_DATABASE_URL=$DATABASE_URL go test ./internal/geo/ ./internal/profile/ ./internal/plans/ -run Integration`; they create and remove their own throwaway cities, activity and users (the profile test needs the Tokyo pack seeded).

## Code generation

```sh
make gen   # from root: buf (Go + TS + docs), sqlc, and the UI-text copy (internal/uistrings/en.json)
```

The code generators are pinned as `tool` directives in `go.mod`, and `../go.work` makes `go tool` work from the repo root.
