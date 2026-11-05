# Feature and knowledge inventory

Research for [#415](https://github.com/shrujancheruku/issho/issues/415), a child of the map [#411](https://github.com/shrujancheruku/issho/issues/411). It records what today's build contains and where each feature's hard-won knowledge is written down. It describes facts only and makes no keep, renovate or rebuild judgments.

**Snapshot:** `main` at `4d13143` (2026-11-05).

**How things were counted**
- Line counts come from `git ls-files … | xargs wc -l`. They are non-generated lines: `gen/`, `node_modules` and the generated `docs/generated/` are left out.
- "Src" means non-test files. "Test" means `*_test.go`, `__tests__/` and `*.test.*`.
- Comment lines are lines that start with `//` in non-test Go files.
- Issue and PR numbers come from `gh issue list` and `gh pr list --state merged --limit 400`. That gave 222 merged PRs, of which about 140 are not dependency updates. Rows dated `YYYY-MM-DD` refer to rows in [docs/decisions-2026.md](../decisions-2026.md).

## 1. Totals

| Area | Src lines | Test lines | Notes |
|---|---:|---:|---|
| `backend/internal/*` (29 packages) | ~26,300 | ~19,800 | `plans` alone is 6,223 / 4,621 |
| `backend/cmd/{api,worker,tool}` | 1,817 | 1,142 | `tool` has 14 subcommands |
| `backend/db/migrations` | 825 SQL | – | `00001_baseline.sql` (squashed 00001–00028), `00002_river.go`, `00003_domain_rule_views.sql` |
| `backend/db/queries` | 2,059 SQL (21 files) | – | sqlc input |
| `apps/mobile/src` | ~10,950 | ~3,600 | routes ~2,990 with almost no route-level tests |
| `apps/admin/src` | 2,308 pages + 809 other | 1,166 | 7 of 16 pages untested |
| `packages/api-client` | 123 | 57 | |
| `data/` | 62 KB JSON | via `cities/data_test.go` | 66 activities, 48 venue categories, 3 city packs |
| CI, deploy and scripts | ~1,900 (YAML, sh, rb) | 98 (`ci-changes.test.sh`) | |
| `site/index.html` | 1,102 | – | |

## 2. Where knowledge is kept: the stores

| Store | Size | What it holds |
|---|---|---|
| [docs/decisions-2026.md](../decisions-2026.md) (frozen) | 312 lines, ~300 dated rows, 2026-10-20 to 2026-11-04 | The why of most product and technical rules. Rows matching each topic (a row can match several): test/CI 155, translation/language 68, release/EAS/store 58, moderation/report/block 50, maps/Routes/travel 50, venues/curation 49, city pack/area 39, admin 39, matching 36, River/worker 33, push 31, Supabase/auth 30, review 30, feedback/attendance 29, crews 28, Kamal/deploy 27, Gemini/Claude/model 25, proto/Connect 24, walks 22, errors 22, design 20, web 15, paging 14, chat 13, Sentry 10, email 10, metrics 4. |
| [docs/adr/](../adr/) | 1 ADR (`0001-issues-replace-the-roadmap.md`) | Only the process decision. No product or technical ADRs since the log froze on 2026-11-04. |
| [GLOSSARY.md](../../GLOSSARY.md) | 69 lines, 17 terms | Plan, Outing, Area, Stage, Display/Shown language, Crew, Room, Attended, Met, Suspended, Kept apart, Lapsed, Pending change, Soft launch, Unlock. |
| [docs/architecture.md](../architecture.md) | 178 lines | Overview, vendor interfaces, the language model, known risks. Several statements are out of date (see §10). |
| [backend/README.md](../../backend/README.md) | 418 lines | The densest store: a per-package table, the endpoint table, the config/env table, migration policy, the worker failure policy and queues, matching steps and rules, and a ~90-row test table. |
| [apps/mobile/README.md](../../apps/mobile/README.md) and [AGENTS.md](../../apps/mobile/AGENTS.md) | 244 + 42 | Structure, push, maps, builds, Expo rules. |
| [apps/admin/README.md](../../apps/admin/README.md) | 83 | Pages, roles, testing approach. |
| [data/README.md](../../data/README.md) | 130 | City-pack format and rules, catalogue merges (`replaces`), boundaries. |
| Other docs | deploy.md 231, onboarding.md 147, contributing.md 144, supabase-hosted.md 130, google-maps.md 94, design-system.md 77, secrets.md 69; supabase/, site/, proto/ and api-client READMEs 20–50 each | Ops, vendor setup, design tokens. |
| Migration and query SQL comments | 825 + 2,059 lines | Table meanings, and the domain views `app.attendance`, `app.suspended_users` and `app.kept_apart`. |
| Go code comments | 4,176 comment lines in `internal/` | plans 753 (12% of src), platform 535 (23%), matching 517 (20%), walks 279 (11%), cities 214, maps 184, uistrings 173, translate 140, crews 128, geo 111, worker 110, chatws 92, notifications 83, adminjobs 79, chat 78, curation 74, moderation 69 (8%), repoedit 61, citypacks 40, tagging 38, push 35, profile 34, catalog 26, emailtemplates 24, identity 18, translations 16 (6%), metrics 11, health 9. |
| Tests as specifications | see each feature | Pure-function tests pin rules, e.g. `outing.test.ts`, `crew-view.test.ts`, `sort.test.ts` and `plans/*_test.go`. |
| Issue resolution comments | open issues #323–#333, #396–#408 and the wayfinder maps #334 and #411 | Post-freeze decisions, e.g. the owner's calls recorded in the 2026-11 architecture reviews. |

## 3. Backend domains (`backend/internal`)

Format: **Src/Test** lines. "Knowledge" says where the rules are written down beyond the code.

### 3.1 Identity, auth and profile

| Feature | Code | Src/Test | Docs | Issues/PRs | Knowledge |
|---|---|---|---|---|---|
| Sign-in (email OTP via Supabase), JWKS token verification, admin and translator roles | `platform/auth`, `identity`, `platform/supabase` | 200/239, 160/138, 96/140 | backend README (auth, config), supabase/README, docs/supabase-hosted.md, CLAUDE.md | PR #3; roles: 2026-10-23 and 2026-10-26 rows | Decision log (OTP not links, secure-store sessions, delete auth user first, `app_metadata.admin` and `translator_langs`). README. |
| Suspension gate (in-memory list, 15 s) | `identity`, `moderation` | – | backend README, GLOSSARY "Suspended" | PRs #55, #276, #286; open #402 (one Standing) | Decision log 2026-10-24 and 2026-11-04. SQL view `app.suspended_users`. |
| Profile and onboarding (`SaveProfile` whole-replace, age rule, ≥3 activities, conduct version) | `profile` | 517/256 | backend README | PR #20; open #358 (re-accept conduct) | Decision log 2026-10-21 rows. Tests are thin relative to the rules (256 lines). |
| Interest tagging (free text → activities, Gemini with an enum schema) | `tagging`, `tagging/gemini` | 242/331 | backend README; architecture.md vendor list | PR #23 | Decision log 2026-10-21. `gemini/eval_test.go` is a live eval skipped unless an env var is set. |

### 3.2 Catalogue, cities, areas and geography

| Feature | Code | Src/Test | Docs | Issues/PRs | Knowledge |
|---|---|---|---|---|---|
| Activity catalogue (66 activities, 8 categories, time blocks, `replaces` merges) | `catalog`, `data/activities.json`, `data/venue-categories.json` | 261/100 | data/README | PR #5; open #357, #347, #348 | data/README. Decision log 2026-10-21 (230 merged into 66). |
| City packs: `cities.Apply` is the only writer; packs are authoritative for areas and overrides; matching overlay, timing and meeting places live in the pack | `cities`, `citypacks` | 1,288/846, 340/561 | data/README, backend README | PRs #130, #136, #235, #266, #287, #302; open #406, #407, #320 | Decision log 2026-11-03 and 2026-11-04. `cities/data_test.go` validates the committed packs. |
| Areas as local government units, with OSM boundaries (ODbL) | `geo`, `cmd/tool/boundaries.go` (175) | 592/587 | data/README, GLOSSARY "Area" | PR #130 | Decision log 2026-10-29 and 2026-11-04; memory note. architecture.md still says "neighbourhood or station". |
| Venue discovery (Places New, request budget 4,000, nearest area, wide categories once per city) | `maps`, `maps/google`, `cmd/tool discover-venues` | 1,288/679 in total (google 607/427, mapstest 275/148) | docs/google-maps.md, backend README | PR #5; open #290, #317, #359 | Decision log 2026-10-20 and 2026-10-29 (store only place ids, budget). |
| Travel times (Routes, 30-day cache, walk and drive only, per-city modes) | `maps`, `cmd/tool build-matrix`, worker `maintenance` queue | in `maps` | docs/google-maps.md, backend README | PR #5, #294 | Decision log 2026-10-20 (no transit in Japan, cache terms). |

### 3.3 Plans, matching and outings

| Feature | Code | Src/Test | Docs | Issues/PRs | Knowledge |
|---|---|---|---|---|---|
| Matching engine: pools per meeting area, Suggest, Schedule, scoring (balance, friends, pairs) | `matching` | 2,642/1,147 (sim 539/57) | backend README "Matching" (steps 0–2 plus rules) | PRs #17, #89, #279, #287, #294; open #349, #401, #406, #408 | Decision log 2026-10-21 to 2026-10-26 (~36 rows of owner decisions: quorum, min 2, gender balance, concentration, over-invite). README. 20% comment density. |
| Simulator (`tool simulate`) | `matching/sim` | 539/57 | backend README | PR #279 | Thin tests (57 lines). |
| Plans: rounds, scheduling, signups, the feed, plan cards, the plan view | `plans` (`service.go` 648, `run.go` 517, `store.go` 478, `load.go` 277, `rounds.go`) | part of 6,223/4,621 | backend README, GLOSSARY "Plan" and "Stage" | PRs #131, #284; open #326, #327, #329–#333 | Decision log 2026-10-21, 2026-10-22 and 2026-10-29 (`plans.Membership`, `rounds.go`, `plans.City`, `Runner.Match`). 12% comments. |
| Outings (events, groups, invites, confirm, check-in, meeting point, maps and routes) | `plans/events.go` 472, `events_handler.go` 324, `membership.go` 194, `routes.go` 171 | in `plans` | backend README, GLOSSARY "Outing", "Attended", "Met" | PRs #34, #39, #42, #43; open #397, #329, #330 | GLOSSARY plus the SQL view `app.attendance`. Decision log (opening hours check, "goes ahead" rules). Open #331 notes that "goes ahead" and "feedback due" are still defined in two places. |
| Opening-hours check before scheduling | `plans` | in `plans` | backend README line ~270 | 2026-11-04 row | README and decision log. |
| Walks (Gemini composes the itinerary, Places and Routes check it; failures rest 7 days) | `walks`, `walks/gemini`, `plans/walks.go` 452 | walks 2,649/907 (including bench 1,766/342) | backend README | PR #45 | Decision log 2026-10-22 and 2026-10-23 (no grounding, the name must match). `walks/gemini/eval_test.go` is a live eval skipped unless `WALKS_EVAL` is set. |
| Walk benchmark (Gemini vs Claude, blind pairwise judges) | `walks/bench` (`cases.json` 443 lines), `walks/claude` (86/82), `cmd/tool/walkbench.go` 299 | 1,766/342 | backend README (subcommand row only) | PR #45 | Decision log 2026-10-22 (benchmark method, Claude via Claude Code). `walkbench.go` has no tests. |
| Admin plan and group review, plans made by hand, stop editing | `plans/review.go` 480, `review_events.go` 292, `proposals.go` 295, `stops.go` 356 | in `plans` | admin README, backend README | PRs #56, #57, #59 | Decision log 2026-10-24 (review switch, proposal rest, approval = scheduling). |
| Feedback and relationships ("rather not" picks, pick strength, gender-lean flag) | `plans/feedback.go` 319, `queries/feedback.sql` 147 | in `plans` | backend README | PR #47; open #321 | Decision log 2026-10-23 (windows, pick strength formula, gender-lean thresholds). |

### 3.4 Crews, chat and notifications

| Feature | Code | Src/Test | Docs | Issues/PRs | Knowledge |
|---|---|---|---|---|---|
| Crews (from the relationship graph, offers, weekly suggestions, leaving silently, the `crews_city` job) | `crews` | 1,013/773 | backend README, GLOSSARY "Crew", "Lapsed" | PRs #47, #48, #134, #278, #284; open #356, #398, #399 | Decision log 2026-10-23 and 2026-11-02. |
| Chat (outing and crew rooms, room lock, retention) | `chat` (482/549), `chatws` (468/536) | 950/1,085 | backend README, GLOSSARY "Room" | PRs #36, #138, #230, #276; open #396, #397 | Decision log 2026-10-22 (plain WebSocket, `NOTIFY`), 2026-10-26 (typing) and 2026-11-02/04 (one module, locks). |
| Notifications outbox and renderers | `notifications` | 470/288 | backend README "Push notifications" | PRs #34, #43, #135, #223; open #325, #328 | Decision log 2026-10-22 (outbox) and 2026-11-02 (one module, partial sends). |
| Expo push sending and receipts | `push` | 229/130 | backend README; mobile README push section | PRs #34, #43, #135 | Decision log. |

### 3.5 Languages and translation

| Feature | Code | Src/Test | Docs | Issues/PRs | Knowledge |
|---|---|---|---|---|---|
| On-demand translation with cache (`translate.Service`, `ResolvePair`, `Prefill`, placeholder drop) | `translate`, `translate/gemini` (218/175), `translate/google` (106/131), `translate/translators` | 805/737 | architecture.md "Languages", docs/google-maps.md, backend README config | PRs #73, #74, #106 | Decision log (~68 rows: English-only storage, fallback order, ISO 639-1, CLDR names, area names never translated, Gemini default). |
| UI strings: catalogue, notes, human versions from `main` every 5 min, `catalog_version` | `uistrings`, `apps/mobile/src/i18n/{en.json,notes.json,human/}`, `cmd/tool ui-strings` and `ui-bundle` | 894/426 | backend README, mobile README | PRs #20, #79; open #315, #350, #352 | Decision log 2026-10-21 and 2026-10-26 (human versions per key, hash of the English). |
| Translation editor service (translator role, rolling PRs per language) | `translations`, `repoedit` (341/430), `platform/github` (292/219) | 267/660 | admin README, backend README | PRs #76, #77, #78, #302, #303 | Decision log 2026-10-26 and 2026-11-04. 6% comment density in `translations`. |
| Sign-in email templates (19 languages, generated and committed) | `emailtemplates`, `cmd/tool email-templates`, `supabase/templates/` | 150/134 | supabase/README, backend README | PR #24 | Decision log 2026-10-21. |

### 3.6 Moderation, curation, metrics and admin plumbing

| Feature | Code | Src/Test | Docs | Issues/PRs | Knowledge |
|---|---|---|---|---|---|
| Reports, moderation queue, suspension with a `remove_suspended` job, kept apart | `moderation`, `queries/moderation.sql` 184 | 826/1,111 | admin README, backend README, GLOSSARY "Suspended", "Kept apart" | PRs #51, #55, #65, #119, #277, #286; open #402, #403, #404 | Decision log 2026-10-23, 2026-10-24 and 2026-11-04 (a report blocks the pair for good, atomic suspend). SQL view `app.kept_apart`. 8% comments. |
| Venue curation (per place, approved-only switch, add by name) | `curation` | 636/604 | admin README | PRs #58, #136 | Decision log 2026-10-24. |
| Domain metrics | `metrics`, `queries/metrics.sql` | 99/151 | admin README | PR #61 | 4 decision-log rows, 11 comment lines; the definitions live in SQL. |
| Admin long-running jobs (River) and `GetAdminJob` | `adminjobs` | 342/221 | backend README | PR #281 | Decision log 2026-11-04. |
| Pending changes as rolling GitHub PRs | `repoedit` | 341/430 | backend README, GLOSSARY "Pending change" | PR #303; open #351 (token expiry) | Decision log 2026-10-26 (no GitHub App, PRs never name the editor) and 2026-11-04. |
| Admin list paging (AIP-158 keyset) | `platform/paging` | 193/88 | admin README, backend README | PR #233 | Decision log 2026-11-03. The admin README notes "not yet checked in a browser (#233)". |

### 3.7 Worker and platform

| Feature | Code | Src/Test | Docs | Issues/PRs | Knowledge |
|---|---|---|---|---|---|
| River worker: queues `notify`, `passes` and `maintenance`; per-kind timeouts and attempts; failure policy | `worker`, `cmd/worker` (109/167), `db/river.go` | 524/524 | backend README "Queues, time and attempts", failure policy | PRs #134, #223, #275; open #405 | Decision log 2026-10-22, 2026-11-02, 2026-11-03 and 2026-11-04. README. |
| API server: interceptors, body cap, deadlines, CORS | `cmd/api` | 347/181 | backend README | PRs #3 and others | Decision log 2026-10-20 and 2026-11-03. |
| Errors (`errs` kinds, `ErrorReasonDetail`) | `platform/errs` | 196/232 | backend README | issue #163 (row) | Decision log 2026-11-03. |
| Validation (`buf.validate` limits) | `platform/validate`, `proto/` | 57/89 | proto/README, backend README | 2026-11-04 row | Decision log. |
| Outside HTTP policy (3 attempts, jittered backoff) | `platform/httpjson` | 318/350 | backend README | issue #182 (row) | Decision log 2026-11-04. |
| Config and `APP_ENV` | `platform/config` | 266/198 | backend README config table | 2026-11-03 row | README. |
| Sentry | `platform/sentry`, `cmd/tool sentry-test` | 218/130 | docs/deploy.md, mobile README | PRs #91, #101, #104 | Decision log 2026-10-26. |
| Gemini client (Agent Platform) | `platform/gemini` | 150/116 | docs/google-maps.md, backend README | PR #23 | Decision log 2026-10-21. |
| Small helpers | `bound` 55/38, `db` 78/75, `deadline` 51/77, `i18n` 117/94, `googleapi` 54/**0**, `pgconv` 24/**0** | | backend README package table (one row each) | | README row plus doc comments. |
| Health | `health` | 57/63 | backend README | | |
| Test fixtures (`testfixture`, `mapstest.Fake`) | `testfixture` 147/241 | | backend README | 2026-11-04 rows | Decision log. |

### 3.8 `cmd/tool` subcommands

The subcommands are migrate, seed, build-matrix, discover-venues, area-boundaries, simulate, walk-bench, ui-strings, ui-bundle, translate-compare, email-templates, sentry-test, set-admin and set-translator. They total 1,361 src and 794 test lines. They are documented in the backend README's `cmd/tool/` row and the Makefile targets (`seed`, `matrix`, `venues`, `boundaries`, `sim`, `email-templates`, `ui-bundle`).

**No dedicated tests:** walk-bench (299 lines), ui-bundle (123), ui-strings, translate-compare and sentry-test. set-translator is covered in `setadmin_test.go`. The 2026-11-04 row says the tool's writes are tested as functions against fakes.

## 4. Mobile app (`apps/mobile`, Expo SDK 58)

| Feature | Code | Src/Test | Docs | Knowledge |
|---|---|---|---|---|
| Tabs Together / Plans / You (the redesign) | `app/(tabs)` (index 362, plans 300, profile 177, `_layout` 30) | 869 / `plans-tab.test` 96 only | mobile README; decision log 2026-10-26 (redesign row) | PR #109; open #400 (check on phones), #306 |
| Plans: feed, cards, signup sheet, maps (native and web), route polylines | `features/plans` | 1,757/560 (10 files) | mobile README maps section, docs/google-maps.md | Tests cover format, map-data, polyline, route-path, sections, signup and text. Decision log 2026-10-21 (interest levels, colours), 2026-10-26 (map motion) and 2026-11-04 (#203 map parity). |
| Plan screen | `app/plan` | 219/0 | mobile README | Code only. |
| Outing screen along the gold thread; feedback; share | `app/event/[id]` 355, `app/feedback` 239, `features/events` (`outing.ts` 160) | 594 routes/0; features 286/301 | mobile README | `outing(event, now)` rules pinned by `outing.test.ts` (154). Decision log 2026-11-02. Open #330 moves these rules to the API. |
| Crews: crew screen, leaving, offers | `app/crew/[id]` 428, `app/crew-leave` 71, `features/crews` (`crew-view.ts`) | 499 routes/0; features 104/197 | mobile README | `crewView()` plus its test (decision log 2026-11-04, #173). |
| Chat panel and room socket, typing dots | `app/chat` 40, `features/chat` (`room.ts` 314) | 672/618 | mobile README | Tests (`room.test` 381). Decision log 2026-11-02. |
| Together tab invites and sort | `features/together` | 151/79 | – | `sort.test.ts`. |
| Sign-in (OTP), auth provider, sign-out unregisters push | `app/sign-in` 141, `app/signup` 171, `features/auth` | 312 routes; 150/358 | mobile README, supabase/README | Tests (sign-in screen, sign-out push). Decision log 2026-10-20 and 2026-11-02. |
| Onboarding and profile editing (steps, draft, language picker, interest suggestions) | `app/onboarding` 96, `app/profile-edit` 92, `features/profile` (`steps.tsx` 448, `draft.ts` 247) | 188 routes; 1,137/165 | mobile README | `draft.test.ts` (149). `steps.tsx` and `suggest-activities.tsx` (127) untested. Decision log 2026-11-02 (settings save rule). |
| Report someone | `app/report` 101 | 101/0 | mobile README omits the route | Code only. |
| Account actions (delete account), appearance (Light/Dark/System) | `features/account` 66, `features/appearance` 90 | 156/0 | mobile README | Code only. Decision log 2026-10-20 (deletion order) and 2026-10-26. |
| App i18n: catalogue fetch and cache, bundled languages, layout direction, shown language | `src/i18n` | 641/537 (8 files) | mobile README; decision log (many rows) | Tests (ui-strings 251, strings 102, sentences lint). |
| Push registration, tap routing, refresh table | `lib/push.ts` 158, `notification-target.ts`, `push-kinds` | in `lib` 645/572 | mobile README push section (lists plan and group pushes only) | Tests (push-routing, push-kinds, push-refresh). Decision log 2026-11-04. |
| Query keys and invalidation, refetch on foreground | `lib/queries.ts` 113, `app-focus.ts` | in `lib` | – | Decision log 2026-11-03 (#171) and 2026-11-04. `queries.test.ts`. |
| Secure session storage | `lib/secure-storage.ts` 74 | in `lib` | – | Decision log 2026-10-20. `secure-storage.test.ts`. |
| Theme and lint rules (text sizes and colours only from the theme; no glued sentences) | `constants` 268, `components` (1,881/214) | | docs/design-system.md, AGENTS.md | `theme-lint.test.ts`, `glued-sentences-lint.test.ts`. Decision log 2026-11-03/04 (#201, #202, #204). |
| Shared frame and loading ladder | `components/query-screen`, `screen-frame` | in `components` | – | Decision log 2026-11-02, 2026-11-03 and 2026-11-04 (a failed refetch keeps the data). |
| Hooks (colour scheme, motion, now, screen active) | `src/hooks` | 117/0 | – | Code only. |
| Web build (`*.web.tsx` variants) | across `features/*` | – | mobile README | PR #21; decision log 2026-10-21; open #340. |
| Builds: EAS, expo-updates fingerprint channels, Sentry source maps | `app.config.js`, `eas.json` | – | mobile README (says iOS "deferred to M9") | PR #104; decision log 2026-10-26 and 2026-10-27; open #316. |
| First-run spotlight tour | **not built** | – | decision log 2026-10-29 rows describe it | open #355 |

## 5. Admin SPA (`apps/admin`, Vite + React)

| Page | Src | Tested | Issues/PRs |
|---|---:|---|---|
| Reports (moderation queue) | – | yes (`Reports.test`) | PR #51 |
| People, Person (suspend) | – | yes | PR #55 |
| PlanDetail, StopsEditor | – | yes | PRs #56, #59 |
| GroupReview | – | yes | PR #57 |
| Venues (curation) | – | yes | PR #58 |
| Cities, CityEditor (pack editor through PRs) | – | yes | PRs #62, #266; open #320 |
| Translations (editor, Gemini suggestions, translator-only view) | 304 | **no** | PRs #77, #78 |
| Metrics | 182 | **no** | PR #61 |
| Plans, NewPlan | 95, 99 | **no** (the admin README says this is deliberate) | PR #56 |
| Groups | 69 | **no** (deliberate per the README) | PR #57 |
| Flags (system signals, today the gender-lean pick flag) | 74 | **no** | PR #47 |
| CrewLeaves | 50 | **no** | PR #47 |
| App shell, router (`Object.hasOwn`), Confirm, format | 809 non-page | yes (`App.test`, `Confirm.test`, `format.test`) | PRs #218, #301, #313 |

**Where knowledge lives:** the admin README (83 lines; one row per page with its RPCs) and the decision log, with 2026-11-03 rows on testing against a typed fake `api` and confirming irreversible actions. Pages total 2,308 src / 1,166 test.

## 6. Contracts and shared client

- **`proto/`:** ConnectRPC services plus the `buf.validate` limits.
  - CI runs `buf breaking` in the `proto-breaking` job.
  - Docs: proto/README (22 lines) and decision log 2026-11-04 (enums, limits).
- **`packages/api-client`:** `index.ts` 87, `errors.ts` 13 (test 33), `limits.ts` 23 (test 24).
  - The README still calls the admin SPA "(planned)".

## 7. Data

| File | Size | Tests | Docs |
|---|---|---|---|
| `data/activities.json` (66 activities, 8 categories) | 30.5 KB | `cities/data_test.go` | data/README |
| `data/venue-categories.json` (48) | 8.5 KB | same | data/README |
| `data/citypacks/tokyo.json` | 6.9 KB | same | data/README |
| `data/citypacks/da-nang-hoi-an.json` (wards with OSM boundaries) | 13.9 KB | same | data/README; PR #130; soft-launch map #334, #346 |
| `data/citypacks/austin.json` (synthetic) | 2.6 KB | same | data/README; decision log 2026-10-20 |

## 8. Vendor integrations

| Vendor | Where | Interface | Tests | Docs |
|---|---|---|---|---|
| Supabase Auth and admin API | `platform/auth`, `platform/supabase` | yes | fake admin API | supabase/README, docs/supabase-hosted.md |
| Google Routes, Places (New), Place Details | `maps/google` | `maps.Router`, `PlaceSearcher` | `mapstest.Fake`, httptest | docs/google-maps.md; open #359 (caching terms) |
| Gemini on Agent Platform (tagging, walks, translator, editor suggestions) | `platform/gemini`, `*/gemini` | `tagging.Tagger`, `walks.Proposer`, `translate.Translator` | httptest plus env-gated live evals | backend README, docs/google-maps.md |
| Google Cloud Translation Basic | `translate/google` | `translate.Translator` (`TRANSLATOR=google`) | yes | docs/google-maps.md (still describes it as the server's translator) |
| Anthropic (SDK and the Claude Code CLI) | `walks/claude`, benchmark only | `walks.Proposer` | 82 lines | decision log 2026-10-22 |
| GitHub API (editor PRs) | `platform/github`, `repoedit` | yes | yes | backend README; open #351 (token renewal) |
| Expo Push | `push` | – | yes | backend and mobile READMEs |
| Firebase Cloud Messaging (Android push via Expo) | config only | – | – | decision log 2026-10-22 |
| OpenStreetMap (boundaries) | `cmd/tool/boundaries.go` | – | – | data/README |
| Sentry | `platform/sentry`, the app, `config/backup` cron monitor | – | yes | docs/deploy.md; open #239 |
| Cloudflare (Workers, R2, origin cert) | `.github/workflows/deploy-web.yml`, `config/backup` | – | – | docs/deploy.md |
| Resend SMTP | `supabase/config.toml` | – | – | supabase/README, decision log 2026-10-20 |
| PostHog | **not present in code** | – | – | decision log 2026-10-26 (deferred); open #342 |

## 9. Infra

- **CI (`.github/workflows/ci.yml`, 377 lines):**
  - Jobs: `changes`, `secrets`, `commits`, `codegen`, `proto-breaking`, `backend`, `typescript`, `image`.
  - `scripts/ci-changes.sh` (129, with a 98-line test) decides which jobs run.
  - Coverage is reported but never gated.
  - Knowledge: decision log (155 test/CI rows), docs/contributing.md.
- **Deploy:**
  - `deploy-api.yml` (159): Kamal, gated on CI for the same commit.
  - `deploy-web.yml` (175): the `site` job and the admin and web Workers.
  - `release-please.yml` (46).
  - `config/deploy.yml` (123) and `deploy.production.yml` (42).
  - `config/server/deploy.sh` (76) and `kamal-role-env.rb` (61).
  - Knowledge: docs/deploy.md (231), decision log (~27 Kamal/deploy rows).
  - PRs #63, #93, #94, #99, #100, #120, #215, #263, #285, #305.
- **Backup:**
  - `config/backup/backup.sh` (122) plus a Dockerfile: nightly to R2 with a Sentry cron monitor.
  - Docs: docs/deploy.md "Off-site backups". PRs #92, #102, #222; open #239.
  - No tests.
- **Supabase config:**
  - `supabase/config.toml` (472) and `supabase/templates/` (3).
  - Docs: supabase/README, docs/supabase-hosted.md, CLAUDE.md.
- **Scripts:**
  - `db-checks.sh` 37, `test-db.sh` 43, `deploy-web.sh` 56, coverage summaries 20 and 21.
  - `scripts/brand/` (logo build, Python), `scripts/site/` (README panels).
  - Only `ci-changes.sh` has a test.
- **Makefile (135 lines):** targets setup, secrets-scan, docs, gen (proto/sql/strings), lint, lint-go, test, test-db, migrate-up, migrate-status, api, worker, seed, matrix, venues, boundaries, sim, email-templates, ui-bundle, app-build, app-icons and readme-panels.
- **Marketing site:**
  - `site/index.html` (1,102 lines, a single page): animated outing story, waitlist.
  - Docs: site/README (23), decision log 2026-10-26 and 2026-10-27.
  - About 15 `fix(infra): … site …` PRs (#112–#128).
  - No tests.
- **Brand and README assets:** `assets/brand` (19 files), `assets/readme` (12 SVG panels generated from the site).
- **Agent tooling:**
  - `.claude/skills` (40 files, the Supabase skills).
  - `.claude/orchestration.md` (orchestrate plugin config).
  - Docs: `docs/agents/*`.

## 10. Flags

### No tests (or near none)

- **Mobile route screens:**
  - None of `app/**` has a test (~2,990 lines), apart from the Plans tab (`plans-tab.test`) and the sign-in screen.
  - Untested routes include `crew/[id]` 428, `event/[id]` 355, `feedback` 239, `plan` 219, `report` 101.
- **Mobile modules with no tests:**
  - `features/profile/steps.tsx` 448
  - `suggest-activities.tsx` 127
  - `features/account` 66
  - `features/appearance` 90
  - `src/hooks` 117
  - `src/constants` 268
- **Admin pages with no tests:**
  - Translations 304, Metrics 182, Flags 74 and CrewLeaves 50.
  - Plans, NewPlan and Groups are untested on purpose, according to the admin README.
- **`cmd/tool` subcommands with no tests:** walk-bench 299, ui-bundle 123, ui-strings, translate-compare and sentry-test.
- **Go packages with no tests:** `platform/googleapi` 54 and `platform/pgconv` 24.
- **Thin tests relative to source:**
  - `matching/sim` 539/57
  - `walks/bench` 1,766/342
  - `catalog` 261/100
  - `profile` 517/256
- **No tests at all:** `site/index.html`, `config/backup/backup.sh`, `config/server/deploy.sh` and every script except `ci-changes.sh`.
- **Live evals only:**
  - `tagging/gemini/eval_test.go` and `walks/gemini/eval_test.go` skip unless an env var is set.
  - The model behaviour of tagging and walks is not checked in CI.

### No or thin docs

- **Domain metrics:** 4 decision-log rows, 11 comment lines; the definitions live in `queries/metrics.sql`.
- **Mobile report screen:** missing from the mobile README's structure table. So is `crew-leave/[id]`.
- **Mobile hooks, query keys and the shared frame:** no README coverage; the knowledge is only in decision-log rows and code.
- **`walks/bench` method:** only in the decision log (2026-10-22) and code; the README has one subcommand row.
- **After 2026-11-04:**
  - The ADR directory has one, process-only ADR.
  - Decisions since then live in issue comments (e.g. the owner's calls on #323–#333 and #401–#407).

### Docs that contradict the code

| Doc | Statement | Code |
|---|---|---|
| docs/architecture.md:92, :109 | Core tables in `00001_init.sql` and `00002_geo.sql`; later migrations up to `00026` | Only `00001_baseline.sql`, `00002_river.go` and `00003_domain_rule_views.sql` exist (squashed, 2026-11-03 row) |
| docs/architecture.md:97 | An area is "a neighbourhood or station" | GLOSSARY and decision log 2026-11-04: areas are local government units |
| docs/architecture.md:70–71 | `cmd/tool` is "migrations and seeding"; `cmd/worker` "(M6)" | `cmd/tool` has 14 subcommands; milestone labels are retired |
| docs/architecture.md:127 | "Google Cloud Translation translates it now" | `TRANSLATOR` defaults to `gemini` (backend README:105; decision log 2026-10-26). architecture.md's own Known risks section names Gemini. |
| docs/architecture.md:88 | Vendor interfaces; "other packages still have their own fakes" | The 2026-11-04 row says no test file defines its own maps fake |
| docs/google-maps.md:88 | "The API server calls Cloud Translation Basic (v2) with `GOOGLE_TRANSLATE_API_KEY`" | Gemini is the default; Google is used only with `TRANSLATOR=google` |
| backend/README.md:278 | "Not done yet: chat, push notifications, the travel-matrix refresh and AI-written plan text." | Chat (`chat`, `chatws`), push (`notifications`, `push`) and the matrix refresh (`maintenance` queue) exist. "AI-written plan text" is the only part not confirmed in code. |
| packages/api-client/README.md:3 | The admin SPA is "(planned)" | `apps/admin` exists (16 pages) |
| apps/mobile/README.md:213, :230, :234 | iOS and EAS builds "deferred to M9" / "M9 is planned" | M9 closed 2026-10-26; Apple moved to Launch (2026-10-29 row) |
| apps/mobile/README.md push section | Lists plan and group pushes | `crews.NotificationRenderer` also sends crew pushes |
| apps/admin/README.md | Paging "not yet checked in a browser (#233)" | Shipped in PR #233; unverified state recorded in the README |

## 11. Knowledge that lives only in code (no README, glossary or decision-log entry found)

- **Mobile:**
  - The plan screen (`app/plan`), the report screen and account and appearance actions.
  - The hooks.
  - The internals of `features/profile/steps.tsx`, i.e. how onboarding steps sequence.
- **Backend helpers:** none. `platform/googleapi`, `pgconv`, `deadline` and the other helpers each have a row in the backend README's package table.
- **Admin:** none at page level. Every admin page has a row in the admin README's page table, including Flags and Crew leaves; the untested pages' behaviour is described there but not pinned by tests.
- **Site:** the marketing site's animation and waitlist mechanics. The site README is 23 lines; decision-log rows describe the story, not the code.
- **Migrations:** the specific SQL inside `00001_baseline.sql` (42 KB). Its comments describe tables, but no doc lists the full schema.
- **Matching scores:** the weight values (e.g. `balance` weight 2 in the decision log; others in the `matching` config and city-pack overlays). The README lists the rules, not every weight.
