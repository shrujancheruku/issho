# Issho: Build Plan & Status

> **This is the source of truth for what's planned, built and pending.** CI (`roadmap` job) fails any push or PR that changes code without updating this file. When you change code, update the status lines below. If the change made a decision, add it to the Decision log.

**Last updated:** 2026-10-21 · **Version:** 0.4.0 (pre-release; [releases](https://github.com/shrujancheruku/issho/releases)) · **Current milestone:** M5 Onboarding & profile (in progress: profile, onboarding and app i18n built; interest tagging and localised auth emails next)

Legend: ✅ done · 🟡 in progress / partial · ⬜ not started · ⏸ deferred (post-MVP)

---

## 1. Product

Issho suggests outings to nearby people aged 18–40 who share interests, then turns them into small groups. An outing starts as a **plan** such as *"Photo walk from Kichijoji station, ending at Café X"*, with no date yet. People see the plans they'd most like first (in a feed and on a map), sign up and say which times suit them. Once enough people have signed up, the plan gets a **date and time** and they're split into groups, matched on interests, availability, a shared language and preferred group size.

After each event, attendees **privately** pick who they'd meet again. **Mutual** picks make those people more likely to be grouped again. Groups that click are offered a recurring **crew**.

- **Platonic only.** These rules discourage one-sided romantic pursuit:
  - only mutual picks count
  - no profile browsing and no 1:1 messages
  - photos are shown only to confirmed attendees
  - gender-composition rules
  - a "here for dating" report option
- **Any language, two at a time.** People can speak any language and events can be held in any language; nobody needs English. Everything is shown in a primary language plus an optional secondary one. For events, primary = the event's language; secondary = the event's second language (e.g. a JP–ES exchange), else the user's own language. Elsewhere, it's the user's primary language plus their secondary choice. Short text side by side, longer text behind a switch. Only English is stored for our own content; every other language is machine-translated on demand and cached.
- **Location-agnostic.** Everything local (areas, travel times, languages, venues, activity names) is data in a **city pack**, not code. The first city is Tokyo.
- **North-star metric:** weekly attendees who have been to more than one event.

## 2. Architecture at a glance

See [architecture.md](architecture.md) for details.

```
Expo app (iOS/Android) ──┐                          ┌─ Supabase Postgres + PostGIS (app schema)
Admin SPA (browser) ─────┼─ ConnectRPC (HTTP/JSON) ─┤  Go API (cmd/api) ─┼─ Google Maps Platform (Places, Routes)
                         │                          │  Go worker (River) ┼─ Claude API (tagging, plan copy)
Supabase Auth ───────────┘  (JWT, verified via JWKS)│                    └─ Expo Push
```

## 3. Milestones

### M1 · Foundations (week 1): ✅ done
- ✅ Monorepo: pnpm workspaces + Turborepo (TypeScript), a Go module with pinned tools (`go tool` buf/sqlc/goose/protoc-gen-doc), and a `Makefile` as the single entry point.
- ✅ API contract in protobuf (buf + ConnectRPC). Generates the Go server, the TypeScript client and an [API reference](generated/api.md).
- ✅ Go API skeleton: config, pgx pool, goose migrations embedded in the binary, Supabase JWKS auth interceptor (deny-by-default), CORS deny-by-default.
- ✅ Services: `HealthService.Check`, `IdentityService.GetMe`, `CatalogService.ListActivities`.
- ✅ Schema v1: cities, areas, travel times, activity catalogue, city overrides, users, profiles, languages, availability, activity preferences.
- ✅ Starter data: 38 activities (English/Japanese) and the Tokyo pack (12 areas). Loaded with `cmd/tool seed`.
- ✅ Local Supabase stack (`supabase start`) with ES256 signing keys. Verified end to end: sign-up → API with a real token.
- ✅ Expo SDK 58 app wired to the generated client (health check on the home screen).
- ✅ CI (green from a clean checkout; actions on Node 24 majors): secret scan, roadmap check, proto lint/breaking, codegen drift, Go vet/test, migration round-trip + seed on PostGIS, TypeScript typecheck, commitlint.
- ✅ Releases: Conventional Commits + SemVer via release-please. Needs the repo setting Settings → Actions → General → "Allow GitHub Actions to create and approve pull requests" (enabled 2026-10-20). Can be re-run manually from the Actions tab.
- ✅ Secrets management: gitleaks (custom rules), pre-commit hook, CI scan, gitignore patterns. See [secrets.md](secrets.md).
- ✅ Onboarding documentation.
- ✅ Marketing site: static landing page in `site/` (dark, logo-led, Dela Gothic One headings, favicon on a solid dark circle, seigaiha (wave) pattern dividers between sections, a thread-and-sun timeline for the steps (nodes joined centre to centre, thread passes through each sun's centre), kanji breakdown coloured like the logo and set in its font (Noto Serif JP), placeholder email + city signup at the end that does nothing yet; copy rewritten for flow and voice, with less repetition), deployed to GitHub Pages by `.github/workflows/pages.yml`. Wire the form to a backend before collecting real data.
- ✅ Brand: logo and mark (一緒 with gold thread and hinomaru sun), one version for every background (black outlined in gold, gold and red outlined in black), generated reproducibly from Noto Serif JP (`assets/brand/`). App icon and splash still use Expo defaults; wire them up in M9.
- ⬜ Sentry and PostHog SDKs. *Moved to M9: nothing worth tracking yet.*

### M2 · Authentication (week 2): ✅ done
- ✅ **Email OTP sign-in** (local): 6-digit code emails (EN/JA templates, 10-minute expiry). The Expo sign-in screen has an email step and a code step (with iOS autofill and auto-submit). Verified end to end with supabase-js against local Supabase.
- ✅ **Secure session storage:** Keychain/Keystore through a chunked `expo-secure-store` adapter (sessions are ~2.2 KB, over SecureStore's ~2 KB limit). Per-write generation IDs, so a torn write can only sign the user out. Covered by unit tests (Jest).
- ✅ **API client sends the Supabase access token.** Auth-gated navigation (`Stack.Protected`): sign-in → onboarding placeholder → tabs. Rejected sessions, e.g. after deletion on another device, sign out automatically.
- ✅ **Account deletion:** `IdentityService.DeleteAccount` (Supabase Admin API, cascade to app data, safe to retry). Sign out and delete in the app. Stale tokens of deleted users get `unauthenticated: account deleted`.
- ✅ **API error hygiene:** internal errors are logged server-side and returned as a generic `internal error`. Database details no longer leak.
- ✅ **Hosted `issho-dev` project**, configured with the Supabase CLI ([supabase-hosted.md](supabase-hosted.md)):
  - config pushed from `config.toml` with `[remotes.issho-dev]` overrides
  - Data API off, SSL enforced
  - Resend SMTP with the code templates
  - legacy keys disabled, ES256 signing
  - schema and seed applied
  - verified: DB state, Data API 503, advisors (only the known PostGIS warning), Go API health against the project
- ✅ **Email OTP expiry set to 600 s** on `issho-dev`, in the dashboard: `config push` (CLI 2.121) doesn't manage it, so repeat this by hand for each new project (see [supabase-hosted.md](supabase-hosted.md#3-push-the-configuration)).
- ✅ **End-to-end sign-in against `issho-dev` on a real device** (Pixel 8 Pro, Android 17, Expo Go 58 via USB): code delivered by Resend, signed in, onboarding shown. The API was also tested against `issho-dev` with a throwaway user (all RPCs, deletion and cleanup verified).
- ✅ **On-device test of the sign-in flow** (see above). It found an Android bug, now fixed: the keyboard covered the code field, because SDK 58 Android apps are edge-to-edge.

### M3 · Cities, catalogue & geo (weeks 2–3): ✅ done
- ✅ City pack format and seeding tool. Packs now declare `travel_modes`, and everything in `data/` is validated by a test (CI) and before seeding.
- ✅ **Activity catalogue: 230 activities** in 12 categories (merged into 63 in 8 categories in M5) (English only; other languages on demand), each linked to the **venue categories** it happens at (48 categories mapped to Google place types, checked against a snapshot of Google's type list).
- ✅ **Google Cloud project** `issho-dev` with Maps and Translation keys (kept in the gitignored `.env.issho-dev`). Confirm the quotas and budget alert from [google-maps.md](google-maps.md) are set. Mobile keys can wait until M6.
- ✅ **`tool build-matrix`** (`make matrix CITY=…`): Routes API Compute Route Matrix per city, **walking and driving only** (public transport dropped; see the decision log).
  - no same-area calls; walking only for areas < 4 km apart; driving is traffic-unaware; minutes rounded up
  - stored in `area_travel_times`; rows older than 30 days deleted (Maps caching terms)
  - `-dry-run` prints requests and billed elements (Tokyo 166, Austin 102)
  - tested with fakes and a DB integration test
  - **run against Google (local DB) for both cities:** Tokyo drive 132 pairs (6–44 min, e.g. Kichijoji → Shinjuku 29) and walk 34 pairs (18–63 min); Austin drive 90 pairs (8–32 min) and walk 12 pairs. Same results on the hosted `issho-dev` DB.
- ✅ **`tool discover-venues`** (`make venues CITY=…`): Places API (New) Nearby Search per area × venue category.
  - field mask: id, types, location only
  - matches on the place's **primary type** (`includedPrimaryTypes`, plus our own check of the returned primary type). Matching on any type pulled in gyms for "sauna", supermarkets for "market" and **hostess bars (スナック)** for "night_club".
  - wide categories (≥ 10 km) searched once per city
  - each place assigned to its nearest area; stores only `google_place_id` + our data
  - `-categories` for partial runs; `-dry-run` (Tokyo 488 requests, Austin 408)
  - **run against Google (local DB):** Tokyo 4,424 venues in all 48 categories, Austin 1,373 in 45. Names spot-checked live per category (e.g. Inokashira Park / 井の頭恩賜公園, Big Echo karaoke, Jo's Coffee).
- ✅ **Hosted `issho-dev` updated:** migrations 00002–00004 applied, both cities seeded, matrices built (312 rows) and venues discovered (5,797). Verified that the new tables aren't granted to `anon`/`authenticated`, goose bookkeeping is out of `public`, and the Data API still answers 503 (disabled).
- ✅ **Synthetic second city pack: Austin** (car-based, walk + drive, 10 areas, Japan-specific activities disabled, local renames and seasons). Seeded, validated, matrix built and venues discovered with no code changes.
- ✅ **Clearer English activity names** for machine translation: 27 idioms and ambiguous words replaced (Pottery → Pottery class: 陶器 "pottery ware" became 陶芸教室; Kickabout → Casual soccer game; "Go" → Shogi or Go board games; "crawl" → tour). The translator also keeps the source's leading capital ("Concierto de banda indie").
- ✅ **Language model** (decided mid-M3, before anything depended on it; see [architecture](architecture.md#languages)):
  - `i18n.ForEvent` / `i18n.ForApp` pick the primary/secondary pair; `i18n.Pair` resolves text, showing the secondary alone rather than an unreadable fallback
  - `ListActivities` takes `locale` (primary) plus `secondary_language` and returns `name` plus `secondary_name`; any language code accepted; verified end to end against local Supabase
  - `profiles.primary_language` + `secondary_language` (migration 00003); language codes validated in the DB
- ✅ **On-demand translation** (`internal/translate`): English is the only language that must be stored. Other languages come from stored text (local names, prefilled events), then the `app.translations` cache (migration 00004), then Google Cloud Translation, cached for everyone. Requests wait up to 8 s, and slower translations finish in the background. Concurrent requests share one call; failures aren't cached; `Prefill` for events. Off without `GOOGLE_TRANSLATE_API_KEY`. Cache path and fallbacks verified end to end; **Live translation verified** (local DB): 230 names into ja/es/fr/ko; cold request 0.65 s for two languages, cached 0.01 s. Short labels without context translated poorly until the English names were clarified (below).
  - the catalogue and overrides are English-only; city packs keep only real local names (東京, 吉祥寺)

### M4 · Matching engine (weeks 3–5): ✅ done
**Plan first, time second** (owner's decision, 2026-10-21): plans are suggested without a date; people sign up and say which times suit them; once a plan's signups can make a good group, it's scheduled and groups are formed. Group size stays open-ended until then.

- ✅ `internal/matching` as a pure package (no I/O, no clock, no randomness; deterministic output) with table-driven tests (90% coverage). See the [backend README](../backend/README.md#matching).
- ✅ Pools (`Pools`): people by meeting area (travel matrix) and a shared fluent language, which becomes the event's primary language (any language). No time: that's decided per plan from its signups.
  - every area is a candidate meeting place; a person joins each area within their `max_travel_min` (fastest mode; no route = unreachable)
  - **two-language events are designed in but not a primary feature:** only for pairs a city lists in `language_pairs` (none by default); anyone fluent in either language joins; primary = the language more members speak. Language learning (`user_languages.fluent = false`) isn't used yet.
- ✅ Suggest (`Suggest`): plans (activity + area + language, no date) ranked by demand × feasibility, taken best first across the city.
  - feasible = a venue in the area (or needs only a meeting point) and an audience of at least 6 people who want it and are usually free at a part of the day it suits (`activities.blocks`, migration 00005; 99 of 230 activities restricted)
  - demand = love 2 / like 1 / open to try 0.5, × 0.3 when over budget; fewer than 3 venues scale the score down
  - combinations: a non-conversational activity gets a conversational food follow-up (bouldering → ramen)
  - **concentration:** no two plans for the same activity in a shared language within 20 minutes (`plan_spacing_min`); at most one open plan per 10 users (`users_per_plan`). Without these the simulator opened 297 plans for 500 people, signups spread thin and 30% of groups were pairs.
  - opening hours can't be stored (Maps terms): check the chosen venue live at event creation (M6)
- ✅ Feed (`Feed`): every open plan ranked for one person, with a level for the map's colours: top (love), good (like), maybe (open to try), low (no preference or beyond their travel time), not for you (doesn't speak the language: shown greyed, can't sign up). Momentum from signups so far lifts plans close to happening; signup counts are shown, never who.
- ✅ Signups (`Signup`): the person's profile availability is the default; per plan they can pick other weekly slots (e.g. weekday evenings + weekend mornings) and block out dates or single occasions. Events they're already going to are blocked out by the caller.
- ✅ Schedule (`Schedule`): for each occasion in the plan's window (default 14 days), form groups from the signups free then; keep the best occasion (ties: earliest). **Ready** when a group reaches its target size; until then the plan keeps collecting. Remnants below the preferred size wait for the next round. `force` (signup deadline) goes ahead with whatever groups can form.
  - assembly: greedy, then local search (fill, replace, swap, spread small groups into bigger ones)
  - score: interest, size fit, reaching the preferred minimum, gender balance, one known pair, newcomers mixed in (not a majority), reliability
  - hard rules (`Violation`; still hold after declines): blocks, same-gender-only, at most 2 who know each other, at least 2 people. **Preferences, not rules** (owner's decision): groups of at least 3 (or the activity's own minimum), nobody the only woman or only man.
  - each group invites its target + 2; `Quorum` confirms straight away; `MinAttendance` (2) is the fewest it can go ahead with at the deadline, so a decline in a group of 3 doesn't cancel it
- ✅ Every setting tunable per city (`ParseConfig` on `cities.matching_weights`; unknown keys rejected).
- ✅ Simulator (`internal/matching/sim`, `make sim`): 500 synthetic users × 2 cities in CI, the whole flow: suggest → each person scrolls their feed and signs up (by level) with varied availability → schedule (ready first, then at the deadline) with double-booking prevented. No DB or keys. Seed 1:
  - Tokyo: 50 plans, 68% of people signed up, 30 plans ready + 20 at the deadline, 72% of signups invited, 78 groups (mostly 4–6), 7 pairs, 0 rule violations
  - Austin: 50 plans, 58% signed up, 26 ready + 22 at the deadline (2 never), 73% invited, 66 groups, 9 pairs, 0 violations
  - one round only: in the real flow, people waiting roll over and plans keep collecting
- ✅ Migration 00005 (`activities.blocks`) applied to the hosted `issho-dev` project and the catalogue re-seeded (2026-10-21). Verified: goose version 5, 99 of 230 activities restricted, `anon` still has no access to `app`. Locally: up, down, up, seed.
- ➡ Storing plans and signups, loading the engine's input and running suggest/schedule are the M6 worker's job.

### M5 · Onboarding & profile (weeks 5–6): 🟡
- ✅ **App i18n framework** (moved up from M9):
  - UI text written in English in `apps/mobile/src/i18n/en.json` (112 strings); `make gen` copies it into the API (`internal/uistrings`), which translates only those strings
  - `UIService.GetStrings` (public, for the sign-in screens): the whole catalogue in one language through the `app.translations` cache; translations that lose a `{placeholder}` are dropped (English shown instead)
  - on the device: kept in the documents directory, shown on the next launch straight away (also offline), refetched while incomplete
  - `useI18n()`: `t` (the language shown, falling back to the other display language, then English), `pair` (both, side by side) and the primary ↔ secondary `LanguageSwitch`
  - sign-in follows the device language (`expo-localization`); signed in, the profile's languages from `GetMe`; onboarding and settings preview the languages being picked
  - **live check (local DB, Google Translation):** 112 strings into Japanese in 0.6 s on a cold cache, placeholders intact. It caught one ambiguous string ("Pick at least {count}" became "tap at least {count} times"), now reworded
- ⬜ Auth emails in the user's language(s), if Supabase templates can branch on user metadata (currently EN/JA for everyone).
- ✅ **Profile RPCs** (`ProfileService.GetProfile` / `SaveProfile`, migration 00006): the whole profile saved in one transaction (languages, availability and activity preferences replaced wholesale). Rules: 18+ this year, a fluent spoken language, distinct display languages, ≥ 1 availability slot, ≥ 3 activities offered in the home area's city, same-gender-only needs a gender, the current code-of-conduct version (`conduct_version` + `conduct_accepted_at`). `GetMe` now also returns the display languages.
- ✅ **Reference data for the pickers:** `CatalogService.ListCities` (cities with areas) and `ListLanguages` (all 183 ISO 639-1 languages, each named in itself and in the viewer's language, from CLDR via `golang.org/x/text`).
- ✅ **Onboarding screens** (seven steps, saved at the end): languages (main, optional second, spoken with fluent/learning + level, searchable picker), about you (name, birth year, gender, same-gender-only), city/area and longest trip, weekly availability grid (localised weekday names), activity tiles by category (tap: open to try → like → love, both languages side by side), group size/energy/budget, code of conduct. **Profile tab** edits each section; sign-out and deletion moved there. The Expo template's Explore tab and unused components were removed.
- ✅ **Verified end to end** against local Supabase with a throwaway user (since deleted): `GetMe` before and after, `GetProfile` not found, an underage save rejected with the field named, a full save round-trips, `GetMe` reports onboarded with ja + es, `ListCities` in es + ja, `ListLanguages` in ja. Not yet run on a device.
- ✅ **Activity catalogue merged from 230 into 63 activities in 8 categories** after the first on-device test (Pixel 8 Pro, Expo Go over Wi-Fi, local stack): onboarding worked, but the tile grid was too long and full of near-duplicates (ten kinds of walk, nine kinds of volunteering). Each merged activity lists the ids it `replaces`; seeding moves saved preferences across (strongest interest kept) and deletes the old ones. Verified on the local DB: a test profile's 50 preferences became 24, a second seed is a no-op. A first cut of 57 grouped team sports and racket sports into one tile each; the owner split them again (people pick a sport, not "team sports"), giving 63. Simulator, seed 1 (230 → 63): Tokyo 67% → 78% of people signed up, 33 → 38 plans ready before the deadline, 67 → 84 groups, 6 → 2 pairs; Austin 52% → 62%, 26 → 28, 56 → 67, 8 → 8.
- ✅ **Interest levels made obvious:** a "How to choose activities" card above the grid, outlined in gold, pairs each instruction ("Tap once", "Tap twice", "Tap three times") with a pill in that level's colour (open to try pale gold, like gold, love red). The tiles use the same colours, with fixed text colours so they stay legible in dark mode.
- ⬜ **Apply migration 00006 to `issho-dev` and re-seed** (merges the catalogue there) after merging.
- ⬜ Free-text interests → activity tags with Claude (Haiku). The user confirms the suggested tags. Includes a tagging eval set in several languages.

### M6 · Plans, signups & events (weeks 6–8): ⬜
- ⬜ Schema: open plans (window, signup deadline, status), signups (per-plan weekly pattern + blocked-out dates/occasions), scheduled events, groups and confirmations.
- ⬜ The River worker (`cmd/worker`):
  - suggest new plans as plans close or fill (`matching.Suggest`), within the city cap
  - daily readiness check per open plan (`matching.Schedule`), forced at the signup deadline; people waiting roll over to the plan's next round or a similar plan; plans that can't form close and say so
  - confirmation deadlines: confirm at `Quorum`, go ahead with at least `MinAttendance`, re-check `matching.Violation` as people drop out
  - meeting-point reveal 24h before the event
  - travel-matrix refresh for each city at least every 30 days (Maps caching terms)
  - chat opens 48h before the event
  - push via Expo Push (a plan you signed up for is scheduled; please confirm)
- ⬜ App: **feed** of open plans ranked for you (`matching.Feed`), scrollable to everything; **map view** of every open plan nearby, colour-coded by level; plan screen with the interest count ("7 interested", never who) and signup with availability pre-filled from the profile (pick weekly windows, block out dates). Event screen: attendees (after confirmation), meeting point after the reveal, Google map, WebSocket group chat, check-in, share with a trusted contact.
- ⬜ Events have a `primary_language` (any language) and an optional `secondary_language` (e.g. language exchanges). Event text is **prefilled in English plus the event's own languages** when the event is created (`translate.Service.Prefill`); each attendee's other language is translated on demand. Plan descriptions are written by Claude, with a template fallback.
- ⬜ Venue names, addresses and hours fetched **live** from Places in each displayed language (Google's own localised names; nothing stored); the scheduled venue's opening hours checked live.

### M7 · Feedback loop & crews (weeks 8–9): ⬜
- ⬜ Post-event feedback: rate the plan, rate the group, pick who you'd meet again (private), report.
- ⬜ Mutual picks recorded as matching links. A person's picks count for less when they're rarely returned.
- ⬜ Flag people whose picks lean heavily towards one gender, for moderation. Crews form from groups that rated each other highly.

### M8 · Admin SPA (weeks 9–11): ⬜
- ⬜ Vite + React app on Cloudflare Pages, signed in with Supabase Auth (admin role claim).
- ⬜ City-pack editor, venue/route curation, plan and group review/override (suggest plans and build groups by hand for the first city), moderation queue, domain metrics.

### M9 · Release prep (weeks 11–12): ⬜
- ⬜ **SMTP sending domain** (moved from M2): Resend is configured for `issho-dev` (free tier, `onboarding@resend.dev`, delivers only to the owner's address). Before real users, it needs a **verified sending domain**.
- ⬜ Spot-check machine translations for the launch languages (admin can mark `app.translations.reviewed`; M8).
- ⬜ Sentry (crashes and errors, mobile and Go) and PostHog (product analytics, feature flags).
- ⬜ Production deploy:
  - Dockerfiles for `api`/`worker`
  - **Kamal 2 (with kamal-proxy for TLS/routing)** to one VPS behind Cloudflare
  - Supabase upgraded to Pro
  - nightly `pg_dump` → Cloudflare R2 as an off-site copy
- ⬜ EAS Build/Submit → TestFlight and Play internal testing.
- ⬜ Stripe Identity verification behind a feature flag, for the public launch.

### Launch (after M9): ⬜
- ⬜ Waitlist page (Cloudflare Pages + PostHog) with counters per area. An area unlocks at a density threshold.
- ⬜ 5–10 hosts and partner venues per city. Seed users from university, language-exchange, newcomer and coworking communities.

### Post-MVP (deferred): ⏸
- ⏸ Sign in with Apple and Google (native, `signInWithIdToken`). Email codes are enough for the MVP. If any social login is added on iOS, Sign in with Apple becomes mandatory.
- ⏸ Paid tier (per-event fee or membership).
- ⏸ OTA updates (EAS Update).
- ⏸ Learned matching weights.
- ⏸ OSM provider as a fallback for Google.
- ⏸ Grafana Cloud / OpenTelemetry (see the decision log).
- ⏸ Neon branching.

## 4. Decision log

| Date | Decision | Why |
|---|---|---|
| 2026-10-20 | React Native + **Expo** for the app | Default for TypeScript teams; New Architecture closed the performance gap; EAS covers builds and signing. |
| 2026-10-20 | **Go** modular monolith, **ConnectRPC** + protobuf | One contract for the Go server and TS client; works over HTTP/1.1 from mobile. |
| 2026-10-20 | pgx + **sqlc** + **goose** + **River** | Type-safe SQL; migrations embedded in the binary; Postgres-backed jobs with no Redis. |
| 2026-10-20 | **Supabase** (DB + Auth + Storage) instead of Neon + Clerk | Open source; users live in our own Postgres; free auth up to 50k MAU; one vendor. Neon's better branching wasn't worth a second vendor. |
| 2026-10-20 | **Google Maps Platform** instead of OSM/Valhalla/OpenTripPlanner | Built-in transit (later found unavailable in Japan; transit dropped, see below); much better venue data (hours, ratings, "good for groups"); no routing servers to run. Fits the free tier at MVP scale. |
| 2026-10-20 | **Claude (Haiku)** for tagging and plan text instead of self-hosted embeddings | Better multilingual understanding; nothing to host; costs cents. |
| 2026-10-20 | Sentry, PostHog, EAS, Expo Push, Cloudflare, GitHub Actions (free tiers) | Each has a clear advantage over self-hosting for a solo developer. |
| 2026-10-20 | Prefer open source; paid/closed only when clearly better; every vendor behind a Go interface | Project principle. |
| 2026-10-20 | Conventional Commits + SemVer (release-please), first release `0.1.0` (`initial-version`) | Automated changelog and versioning. Without `initial-version`, release-please makes the first release 1.0.0 even with `bump-minor-pre-major`. |
| 2026-10-20 | Supabase config at repo root (`supabase/`), not `deploy/supabase` | The Supabase CLI expects `./supabase`. goose (not the CLI) owns migrations. |
| 2026-10-20 | `Makefile` as the task entry point; Turborepo runs the TS tasks | `just`/`mise` weren't installed and add nothing over make here. |
| 2026-10-20 | **Redundancy review: Caddy dropped.** | Kamal 2's built-in kamal-proxy already does TLS and routing, and Cloudflare sits in front. |
| 2026-10-20 | **Redundancy review: Grafana Cloud/OpenTelemetry deferred.** | Sentry already covers backend errors and performance tracing, and `kamal app logs` covers logs at MVP scale. Revisit when we need metrics dashboards. |
| 2026-10-20 | **Redundancy review: one error tracker.** | Sentry handles errors and crashes; PostHog's exception autocapture stays **off** so errors don't land in two places. |
| 2026-10-20 | **Redundancy review: no testcontainers.** | CI already runs a PostGIS service container and local dev uses `supabase start`. A third way to get a database adds nothing. |
| 2026-10-20 | **Redundancy review: Zustand and connect-query deferred.** | TanStack Query plus the generated client cover current needs. Add them only when local UI state or query-key boilerplate actually hurts. |
| 2026-10-20 | **Redundancy review: R2 kept, but only for off-site `pg_dump`.** | Supabase Pro does daily backups, but they live with the same vendor. A cheap off-site copy is real disaster recovery, not duplication. Photos use Supabase Storage. |
| 2026-10-20 | Admin SPA and waitlist on **Cloudflare Pages** | Static hosting is free, and with Caddy gone the VPS serves only the API. |
| 2026-10-20 | Logo built from Noto Serif JP outlines by a script (`scripts/brand/build_logo.py`) | Reproducible, OFL-licensed, no font dependency at display time. Colours: red `#BC002D`, gold `#C9A227`, black `#111111`. |
| 2026-10-20 | Logo polish: black-outlined parts (sun, thread) layered over gold-outlined parts; wordmark in **Dela Gothic One** | The serif wordmark was bland. A chunky, Japanese-designed display face gives the name personality next to the brush-like kanji. |
| 2026-10-20 | One logo version everywhere: no dark variant. Contrasting outlines instead. | Brand consistency. Gold outlines on black and black outlines on gold/red keep it legible on light and dark backgrounds. |
| 2026-10-20 | pnpm dependency build scripts deny-by-default (`allowBuilds` in `pnpm-workspace.yaml`) | pnpm 12 fails installs on unapproved install scripts. Each package is approved or denied explicitly; `unrs-resolver` is denied because its script only fetches fallback binaries. |
| 2026-10-20 | Commit `apps/mobile/src/types/expo.d.ts` | Expo's generated `expo-env.d.ts` is gitignored by convention, so fresh clones and CI couldn't typecheck CSS imports. |
| 2026-10-20 | Passwordless **email OTP codes**, not magic links | Links can't reliably open a native app, so the user types a 6-digit code instead (iOS autofills it from Mail). Codes expire in 10 minutes, down from 1 hour. |
| 2026-10-20 | Sessions in **`expo-secure-store`** (chunked), not expo-sqlite/AsyncStorage | Refresh tokens are long-lived credentials and belong in the Keychain/Keystore. Chunking works around the ~2 KB value limit; generation IDs make torn writes detectable. |
| 2026-10-20 | Error-sanitising Connect interceptor | connect-go returns `err.Error()` for plain errors, which leaked SQL constraint names to clients. |
| 2026-10-20 | Deleting an account: auth user first, then app data | If deleting the auth user fails, nothing is lost and the user can retry. The reverse order could leave a live login with no data. |
| 2026-10-20 | **Email OTP is the only sign-in method for the MVP**; Apple/Google deferred | Not needed to validate the product. It avoids OAuth setup and secrets, and the App Store rule that requires Sign in with Apple once any social login exists. |
| 2026-10-20 | **Supabase Data API disabled**; goose's version table moved from `public` to its own `goose` schema | Found while preparing the hosted project: through the Data API, anonymous clients could **write** to `public.app_goose_db_version` (marking migrations as applied) and modify PostGIS's `spatial_ref_sys`. Its grants are owned by `supabase_admin`, so a migration can't revoke them. Auth works without the Data API. CI now checks the version table isn't in `public`. |
| 2026-10-20 | Production `DATABASE_URL` uses the **session pooler** (or direct connection), not the transaction pooler | pgx uses prepared statements by default, and River needs LISTEN/NOTIFY. The transaction pooler supports neither. |
| 2026-10-20 | **Device testing on Android over USB** (`adb` plus Expo CLI) while the store builds of Expo Go lag behind SDK 58 | The App Store/Play Store Expo Go only ran SDK 57. Local iOS needs Xcode 26.4+, which needs macOS 26.2+ (the dev Mac runs 15.7). The CLI installs a matching Expo Go over USB with just `adb`. |
| 2026-10-20 | Hosted Supabase configured **declaratively with the CLI** (`config push`, `[remotes.<name>]` overrides) | Reproducible and reviewable via `config diff`. The diff caught local-only values that would have leaked: confirmations off, a 1 s resend interval, a 2/hour email cap. |
| 2026-10-20 | **Custom SMTP (Resend) is required even in development** | On the Free plan, Supabase blocks custom email templates while its built-in email service is in use, and the built-in templates only contain links, which can't drive code sign-in. |
| 2026-10-20 | Official **Supabase agent skills** in `.claude/skills/` (installed with a pinned `skills` CLI, verified byte-for-byte against upstream) plus a root `CLAUDE.md` | Gives coding agents current Supabase and Postgres guidance. `CLAUDE.md` overrides where the generic skills conflict with this repo (goose migrations, Data API off, MCP not configured). |
| 2026-10-20 | **Dela Gothic One** for standout/hero text in the app (`title`, `subtitle`); system font for body text | Ties the app to the wordmark. Loaded at runtime (`useFonts`) so it works in Expo Go; it has a single weight, so no `fontWeight` on display text. |
| 2026-10-20 | commitlint also runs as a local `commit-msg` hook | The first M2 PR failed CI on over-long commit bodies. Catching that at commit time is cheaper than rewriting history after a push. |
| 2026-10-20 | **jest-expo** for mobile unit tests; runs in CI with lint | Security-critical client code (session storage) needs tests. jest-expo is Expo's standard preset. |
| 2026-10-20 | CORS deny-by-default via an explicit origin allowlist | rs/cors treats an empty `AllowedOrigins` as allow-all. |
| 2026-10-20 | **Symmetric languages, two at a time.** No default language; people speak and events run in any language. Event: its language + its second language, else the viewer's language. App: the user's primary + secondary choice. Side by side for short text, a switch for long text. | Product decision: people who don't speak English must be fully served. A short-lived "English-first" version was replaced before it shipped. Centralised in `i18n.ForEvent`/`ForApp` so every surface applies the same rule. |
| 2026-10-20 | **English is the only stored language; everything else is translated on demand and cached**, not maintained by hand | No translation files to keep in sync, and any language works from day one. Caching makes it a one-off cost per text and language (cents), and everyone sees the same wording. Events are prefilled in their own languages because those are certain to be needed. Replaces hand-written ja/es/fr catalogue names. |
| 2026-10-20 | Machine translation via **Google Cloud Translation (Basic, v2)** instead of Claude Haiku | Owner's choice. Purpose-built MT on the Google Cloud project we already use for Maps (one vendor, one bill, the same API-key REST client), with a free monthly allowance. Basic can't take per-string context; the `Translator` interface allows switching (Advanced v3 or an LLM) if label quality suffers. |
| 2026-10-20 | Server-side translation, not on-device (ML Kit / Apple Translation) | The result is consistent across iOS and Android, cached once for everyone, can be reviewed, and needs no native modules (keeps Expo Go). On-device stays an option for user-written text such as chat (M6). |
| 2026-10-20 | **Google place names are not stored**, even when Google has several languages; they're fetched live per displayed language | Maps terms allow storing only place IDs. Live fetches give Google's own local names, which beat machine-translating proper nouns. |
| 2026-10-20 | Language codes limited to **ISO 639-1** | Every language people use is covered, and clients can't make the server pay to translate into made-up codes. |
| 2026-10-20 | Missing translations fall back to the **other displayed language** first, English last | For someone who doesn't read English, showing only the language they do read beats a fallback they can't. English stays the authoring language for our data so a last resort always exists. |
| 2026-10-20 | Display languages (`profiles.primary_language`/`secondary_language`) kept **separate from spoken languages** (`user_languages`) | What someone wants to read and what they can talk in at an event are different questions; matching uses only the latter. |
| 2026-10-20 | APIs return primary **and** secondary text (`name` + `secondary_name`) instead of one localised string | Clients need both for side-by-side display; the switch is then a client-side choice with no extra round trip. Identical translations come back empty so nothing is shown twice. |
| 2026-10-20 | Google Maps over **plain REST** (`net/http`), not Google's Go client libraries | Two endpoints. The REST client gives full control over field masks (which set the billing SKU), keeps the key in a header and adds no dependency tree. |
| 2026-10-20 | **Venues store only `google_place_id` + our data**, and one row per place × category | Maps terms allow storing place IDs indefinitely but not names, addresses or locations. A karaoke bar is legitimately both `bar` and `karaoke`. |
| 2026-10-20 | **Travel times are a 30-day cache**, rebuilt per city | Routes content may only be cached temporarily. A full rebuild is cheap (~170 elements for Tokyo). |
| 2026-10-20 | Travel modes are **per city** (`travel_modes` in the pack) | Lets a city skip a mode that makes no sense there, saving billed elements. |
| 2026-10-20 | **Walking and driving only; no public transport** in travel-time calculations | Google's Routes API returns no transit routes in Japan, which ruled out the Tokyo launch city. A Japan-specific transit API or a self-hosted OpenTripPlanner would add a vendor or a routing server. Walking and driving are available everywhere from the same API. Driving times understate a train city like Tokyo, so treat them as a relative measure of distance, not a promise. Removed from the schema (`area_travel_times`, `cities.travel_modes`), the Google client and the tool. |
| 2026-10-20 | Venue discovery assigns each place to its **nearest area**, and searches wide categories (≥ 10 km) **once per city** | Search circles overlap. Nearest-area assignment gives the matcher the right travel time, and searching beaches or trails from every area would pay for the same results again. |
| 2026-10-20 | Activities link to **venue categories** (our ids), not Google types directly | Keeps the catalogue provider-neutral, and gives M4 the "is there a venue for this?" check. |
| 2026-10-20 | Marketing site is a **single static HTML file in `site/`**, deployed with the official Pages Actions workflow | No build step or dependency to maintain, and the site stays separate from the app. Signup form is a deliberate no-op until a backend and privacy wording exist. |
| 2026-10-20 | Second city pack is **Austin** (synthetic, not a launch city) | A real car-based city with real coordinates, so the Google tools can be exercised end to end. It's Japan-free on purpose, which forced per-city disables and renames. |
| 2026-10-21 | Matching pools are **per meeting area** (everyone who can reach it), not fixed area clusters | Clusters split neighbours who live either side of a boundary. Overlapping pools cost nothing at city scale (areas × languages). |
| 2026-10-21 | Pool travel uses the **fastest** of the city's modes per area pair | Someone who'd walk 20 minutes won't drive 25. The DB loader takes the minimum so the engine sees one number per pair. |
| 2026-10-21 | **Two-language events are not an MVP feature, only designed for:** pools for city-listed `language_pairs` (default none), open to anyone fluent in either language; learner data unused | Owner's decision. Keeps the schema and engine ready (`Plan.Secondary`, event `secondary_language`) without building language-exchange matching yet. They're discounted (`bilingual_weight` 0.5) so most plans are in one language everyone shares. |
| 2026-10-21 | Activity **time-of-day blocks** in the catalogue (`activities.blocks`, migration 00005) | The roadmap's "time of day suits the activity" needs data, and Maps terms forbid storing opening hours. Coarse blocks cover the common cases (bars in the evening, hikes in daylight); live hours are checked when the event is created. |
| 2026-10-21 | **Only 2 people is a hard minimum**; groups of at least 3 (or the activity's own minimum) are a **preference** in the score | Owner's decision: a smaller outing beats no outing. Catalogue minimums are mostly social choices (chess says 4). Replaces a same-day hard minimum of 3. A city can make 3 strict again with `min_group_size`. |
| 2026-10-21 | **Confirm at `Quorum`, go ahead at the deadline with `MinAttendance` (2)** | Owner's decision: a decline in a group of 3 shouldn't cancel an event that can still happen with 2. |
| 2026-10-21 | **Nobody the only woman or only man is a preference**, not a rule (`balance` weight 2) | Owner's decision: better to invite someone into an unbalanced group than to leave them out. Same-gender-only stays a hard rule for those who ask for it. |
| 2026-10-21 | The **only language requirement** is the event's own: everyone speaks its primary or secondary language | Owner's decision. Two-language events are expected to mix people who share only one of the two. |
| 2026-10-21 | **Plan first, time second:** suggest plans without a date, collect signups with per-plan availability, schedule once the signups can form a good group | Owner's decision. Pools interest over time instead of needing a week's slot to line up (the main density risk), makes group size open-ended, mostly removes leftover pairs, and gives an explicit interest signal. Replaces weekly proposals with accept/decline. |
| 2026-10-21 | Signup availability: **profile by default, per-plan weekly windows and blocked-out dates** | Owner's decision. One tap for most people, while still capturing "not Wednesday nights" so the scheduled time is one they can make. |
| 2026-10-21 | **Interest counts are shown** ("7 interested"), never who | Owner's decision: momentum helps plans fill. Names stay hidden (no profile browsing). |
| 2026-10-21 | **Targeted feed plus everything:** plans ranked by likely interest, then the rest; a map of all nearby plans colour-coded by level | Owner's decision. People who don't speak a plan's language see it greyed out and can't sign up. |
| 2026-10-21 | Suggestions are **concentrated**: same activity + shared language ≥ 20 min apart, at most one open plan per 10 users | The simulator showed 297 plans for 500 people without it: signups spread thin, plans only went ahead at the deadline and 30% of groups were pairs. With it: 50 plans, 7 pairs in 78 groups. |
| 2026-10-21 | A ready plan schedules only groups at the preferred size; smaller remnants wait for the next round (all groups are kept at the deadline) | Waiting may turn a pair into a proper group; at the deadline something beats nothing. |
| 2026-10-21 | Over-invite **inside the group**: invite target + 2, confirm at the target (`Quorum`) | Inviting leftovers as extras after assembly almost never happened: leftovers were used for new groups first (the simulator showed 0 extras in Tokyo), so one decline would sink an event. Still applies after the switch to signups: some who signed up won't confirm the chosen time. |
| 2026-10-21 | The gender-balance preference applies to **women and men**; nonbinary and unspecified are exempt | Requiring two nonbinary people per group would leave most nonbinary users unmatched. Revisit with real data and feedback. |
| 2026-10-21 | **App UI text: English source in the app, translated on the server**, whole catalogue per language, cached on the device | Same model as the rest of the content (English stored, everything else on demand) with no translation files to maintain. One request per language keeps the client simple; the device copy makes later launches instant and works offline. |
| 2026-10-21 | `UIService.GetStrings` is **public**, but translates only the server's copy of `en.json` | The sign-in screens need translated text before there's a token. An allowlist bounds the cost (one translation of ~110 strings per language, ever) and stops it being used as a free translator. |
| 2026-10-21 | Translations that **lose a `{placeholder}` are dropped**, not shown | Cloud Translation Basic can translate or drop placeholders. English is better than a sentence with a hole in it. |
| 2026-10-21 | **Area names are never machine-translated**; city names are | Live check: Spanish turned "The Domain" into "El dominio" and "Cedar Park" into "Parque Cedar". Neighbourhoods are proper nouns: show English plus a real local name (吉祥寺). Cities have genuine exonyms (Tokio). |
| 2026-10-21 | **Language names from CLDR** (`golang.org/x/text/language/display`), not machine translation | Exactly what CLDR is for, and free. About a third of ISO 639-1 codes have no own-language name there; those show their English name. |
| 2026-10-21 | **`SaveProfile` replaces the whole profile** in one transaction; no per-field RPCs | Onboarding saves once at the end and settings edits one section of a loaded profile, so one write path covers both, and a profile is never half-saved. |
| 2026-10-21 | **Age: born in or before (this year − 18)** | Only the birth year is asked, so someone turning 18 later this year passes. Identity verification (Stripe Identity, M9) is the real check. |
| 2026-10-21 | **At least 3 activities** and **a fluent spoken language** to finish onboarding | The feed needs preferences to rank by, and matching pools people by a fluent language. |
| 2026-10-21 | The **code-of-conduct version** is stored with the profile; the rules text is UI copy | Bumping `ConductVersion` (server and app) makes everyone accept again; settings never accepts on the person's behalf. |
| 2026-10-21 | **Broad activities: 230 merged into 63**, in 8 categories; **each sport stays its own activity** | Owner's feedback from the device test: overlapping options and too many of them would lose people in onboarding. Broad activities also pool interest per plan (the main density risk); the simulator signed up more people and formed fewer pairs. Specifics (waterfall hike, sake tasting) move into each plan's description. Sports are the exception (owner's decision): tennis and basketball aren't interchangeable, so "Team sports" and "Racket sports" were split back into individual sports. |
| 2026-10-21 | Merges are **data, not migrations**: `replaces` in the catalogue, applied by seeding | The catalogue is seed data, so its history belongs with it. Seeding moves preferences to the surviving activity in the same transaction, and only ever deletes ids named in `replaces`. |
| 2026-10-21 | Interest levels shown as **colours shared by the legend and the tiles** | Owner's feedback: the tap-to-cycle levels weren't obvious. A legend in the tiles' own colours explains it before the first tap, without an extra screen. An animated example tile was tried and removed (distracting). |
| 2026-10-21 | The code of conduct asks for **platonic intentions and respect for boundaries**; it doesn't ban romantic interest | Owner's decision: feelings can't be prohibited, and the rule should be one people can keep. Unwanted pursuit is still reportable ("here for dating"). Worded without idioms so machine translation keeps the meaning. |
| 2026-10-21 | Profile **settings live in a Profile tab**, replacing the Expo template's Explore tab | Somewhere to edit the profile and to reach sign-out and deletion, which used to be on the onboarding placeholder. |
| 2026-10-21 | ~~One proposal per person per matching run~~ | Superseded the same day by signups: people can sign up for several plans; an event they're going to blocks its occasion in their other signups, so they're never double-booked. |

## 5. Known gaps & risks
- **Machine-translation quality on short labels.** Cloud Translation Basic has no context, so short names can come out wrong. Clearer English names fixed the cases found so far; when writing new names, avoid idioms and words with two meanings. Further fixes: admin corrections of cached rows (`reviewed`, M8) or Advanced (v3) glossaries.
- **UI text quality and coverage.** Every non-English UI string is machine-translated without context (Basic v2). Spot-check the launch languages (M9) and write unambiguous English. A string translated badly but with its placeholders intact is cached and shown until an admin corrects it (`reviewed`, M8).
- **Re-accepting the code of conduct isn't built.** Bumping `ConductVersion` would make settings saves fail until the app asks people to accept the new version; add that screen before the first revision.
- **Matching untested on real behaviour.** The simulator's users, venues, travel times and signup behaviour (who signs up, how far they scroll) are guesses, so its numbers show the engine works, not that the density or conversion is enough. ~10% of simulated groups are pairs, effectively 1:1 meetings, which the platonic rules try to avoid; M6 should watch how pairs (especially mixed pairs) go and tune `min_size`/`balance`, or set `min_group_size` to 3 per city.
- **Drop-off between signup and confirmation.** Signing up is cheap; some won't confirm the chosen time. Per-plan availability and extra invites reduce it; M6 should measure it and tune `extra_invites`, the window and deadlines.
- **Not enough nearby users** is the main product risk. Plan-first scheduling pools interest over time, and the admin override lets the first city be hand-curated.
- **PostGIS lives in the `public` schema** (migration 00001). With the Data API disabled, no anonymous path reaches it, but Supabase's advisors flag it. Move it into the `extensions` schema with a migration **before production data exists**: PostGIS can't be relocated in place, so the geography column has to be recreated.
- **Google Maps terms** restrict caching. What we do: store only place IDs plus our own data, use place locations in memory only, and delete travel times after 30 days ([google-maps.md](google-maps.md#5-terms-what-we-may-store)). The Routes policy says most content is cache-restricted but doesn't state a window for durations; 30 days follows the Service Specific Terms' limit for other content. Re-check with the current terms before launch.
- **Venue quality** comes from popularity ranking and primary-type matching only (ratings and "good for groups" are pricier Enterprise fields). Some unsuitable places still share a valid primary type: Japanese **snack bars (スナック, hostess bars) are typed `bar`**, and a tattoo studio appeared as `art_studio`. Every venue starts as `candidate`; admin curation (M8) must approve venues before the matcher uses them, and the M8 review should look for snack bars specifically.
- **`postgis/postgis` has no arm64 image.** CI (amd64) is unaffected. On Apple Silicon, use `supabase start` (arm64) or run that image with `--platform linux/amd64`.
- **Local iOS builds need macOS 26.2+** (Xcode 26.4+ for SDK 58). The dev Mac is on macOS 15.7. Either upgrade before M9, or build iOS in the cloud with EAS.
- **Disk space on dev machines:** Supabase images take several GB. OrbStack's VM disk can report "no space left" on its first launch; retrying fixes it.
