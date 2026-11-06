# Audit: backend platform, schema and API contract

Research for [#426](https://github.com/shrujancheruku/issho/issues/426), a child of the map [#411](https://github.com/shrujancheruku/issho/issues/411). It scores each part of this area against Verdict criteria ([#413](https://github.com/shrujancheruku/issho/issues/413)), measures it against The hygiene bar ([#414](https://github.com/shrujancheruku/issho/issues/414)) and judges only what The Pilot's scope ([#417](https://github.com/shrujancheruku/issho/issues/417)) keeps in. It gives facts and recommendations only. The owner decides in the verdict ADR ([#429](https://github.com/shrujancheruku/issho/issues/429)).

**Snapshot:** `origin/main` at `139d77e` (2026-11-06). Line counts are non-generated `src/test` lines. Paths are relative to the repo root.

**Sources**
- The code at the snapshot.
- `git log --since=2026-10-20 -i --grep='^fix'` on this area's paths, with `--stat` to see which files each fix touched.
- `gh issue list` (open issues and the `bug` label).
- The Feature and knowledge inventory (#415, branch `research/feature-and-knowledge-inventory`).
- Tool runs at the snapshot:
  - `go build ./...` and `go test` (no database) on every package in this area and on the parked and removed packages.
  - `go run golang.org/x/tools/cmd/deadcode@latest`, run both with and without `-test`.
  - A script that checks, for each of the 84 RPCs in `proto/`, whether a Go test calls it with a `connect.Request`.
  - A script that counts, for each of the 242 sqlc queries, its non-generated callers.

**How the signal colours were set.** #413 names the signals but sets no thresholds, so this audit used these:
- Defect history: green for 0–1 fix commits and no open bug; amber for 2–4, or one open bug; red for 5 or more, or the same defect coming back.
- Refactor load: green for no frozen refactor ticket; amber for 1–2; red for 3 or more, or one that fixes a live defect.

## Summary

| Part | Tests | Structure | Domain fit | Docs | Defects | Refactor load | Renovate / rebuild (sessions) | Recommended verdict |
|---|---|---|---|---|---|---|---|---|
| A. Platform core (`internal/platform/*` except `github`, plus `health`, `testfixture`) | green | green | green | amber | amber | green | 0.5 / 5 | **Keep** |
| B. `platform/github` + `repoedit` | green | green | green | green | green | green | 0.5 / 2 | **Keep, not parked** (scope conflict, see B) |
| C. Worker (`internal/worker`, `cmd/worker`) | green | amber | amber | green | amber | amber | 1 / 3 | **Keep** |
| D. API server (`cmd/api`) | amber | amber | green | amber | amber | amber | 1 / 2 | **Keep** |
| E. `cmd/tool` (in-scope subcommands) | amber | amber | green | amber | amber | green | 1 / 2 | **Keep** (remove 2 subcommands, park 2) |
| F. `adminjobs` | green | green | green | green | green | green | 0.25 / 1.5 | **Keep** |
| G. `metrics` | green | green | amber | amber | green | amber | 0.5 / 1 | **Keep** (the success bar is gate work, not hygiene) |
| H. Schema: migrations, domain views, `db/queries`, sqlc | green | green | amber | amber | amber | **red** | 3 / 10 | **Renovate** |
| I. `proto/` | green | amber | amber | amber | green | amber | 1 / 4 | **Keep** |
| J. `packages/api-client` | amber | green | green | amber | green | green | 0.25 / 0.5 | **Keep** |

How the rules apply:
- **Rebuild:** no part has red domain fit, and no part has red tests together with red structure, so no part is a rebuild.
- **Keep:** a part keeps when it has no reds and costs about 2 sessions or fewer to reach the bar.
- **H, the only renovate:** its one red is refactor load, which may only move a part between keep and renovate. Its cost check also favours renovating (3 sessions against 10).

## Scope cuts in this area (#417)

| Item | #417 status | Evidence | Check or size |
|---|---|---|---|
| `tool simulate` (`cmd/tool/main.go:222`, `internal/matching/sim` 539/57) | **Parked** | Listed under Parked | Compiles; `go test ./internal/matching/sim ./cmd/tool` passes. |
| `tool walk-bench` (`cmd/tool/walkbench.go`, 299 lines, no tests) | **Removed** | Listed under Removed, with `walks/bench` and `walks/claude` | 299 lines in `cmd/tool`, plus `walks/bench` 1,766/342 and `walks/claude` 86/82 (owned by #422). The `case` at `cmd/tool/main.go:69`. The usage text at `main.go:9` and `:64`. README rows `backend/README.md:38` and `:268`. Removing `walks/claude` and `walks/bench/llm.go` also drops `github.com/anthropics/anthropic-sdk-go` (`backend/go.mod:22`). About 0.25 session. |
| `tool translate-compare` (`cmd/tool/translatecompare.go`, 85 lines, no tests) | **Removed** (follows from it) | It compares `translate/google` with Gemini (`translatecompare.go:15`, `:61`). #417 removes `translate/google` and `TRANSLATOR=google`. | 85 lines plus the `case`, the usage text and the README row (`backend/README.md:11`). |
| `tool set-translator` (`cmd/tool/settranslator.go`, 64 lines, tested in `setadmin_test.go`) | **Parked** | The translator role is parked | Compiles and passes its tests. |
| `TRANSLATOR` / `GOOGLE_TRANSLATE_API_KEY` in `platform/config` | **Removed** (follows from `translate/google`) | `backend/README.md:83`, `:95`, `:105`, `:106` | Small: the config fields, `translators.New`'s `google` branch, and the warning text at `cmd/api/main.go:150` and `cmd/worker/main.go:61`. Lands with #425's removal. |
| `TranslationService` (9 RPCs, `proto/issho/v1/translation.proto`), `internal/translations` | **Parked** | The translation editor is parked | Compiles; `go test ./internal/translations` passes. Still wired at `cmd/api/main.go:209-224`. |
| `CatalogService.SuggestActivities` | **Parked** (interest tagging) | Wired at `cmd/api/main.go:152-158` | Compiles; `go test ./internal/catalog` passes. |
| `CityPackService.GetCityPack` / `ProposeCityPack` | **Parked** (CityEditor) | `backend/README.md:173` | Compiles; `go test ./internal/citypacks` passes. **Open question:** `ListCityPacks`, `ApplyCityPack` and `DiscoverAreaVenues` back the admin *Cities* page, which #417's admin list leaves out. Yet venue discovery is in scope, and "the owner opens an area through a PR to the pack" still needs an apply (`ApplyCityPack` or `tool seed`). |
| `repoedit`, `platform/github` | **Parked** per #417 | But see part B: the in-scope UI-string follow runs through them | — |
| The Tokyo pack | **Removed** (#425) | `scripts/db-checks.sh:24` seeds `data/citypacks/*.json`, and `:31` runs `build-matrix -dry-run tokyo` | The removal must change `db-checks.sh:31` (to `da-nang-hoi-an` or `austin`). Austin stays: `db-checks.sh:32` uses it as a fixture. |

## A. Platform core

**Code:** `internal/platform/*` without `github`: auth 200/239, bound 55/38, config 266/198, db 78/75, deadline 51/77, errs 196/232, gemini 150/116, googleapi 54/**0**, httpjson 318/350, i18n 117/94, paging 193/88, pgconv 24/**0**, sentry 218/130, supabase 96/140, validate 57/89. Plus `internal/health` 57/63 and `internal/testfixture` 147/241.

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | green | Every package has tests except `googleapi` (54 lines) and `pgconv` (24 lines). The critical ones pin behaviour:<br>• `errs_test.go`/`kinds_test.go`: Internal is sanitised, Upstream is reported.<br>• `auth_test.go`: JWKS, AdminOnly, TranslatorsOnly.<br>• `httpjson_test.go` (350): retry and backoff.<br>• `deadline_test.go`.<br>`testfixture/rules_test.go:70` (`TestIntegrationDomainRuleViews`) pins the domain views. |
| Structure | green | `platform` imports no domain package; its only internal import outside `platform` is `testfixture`, in a test. Packages are small and single-purpose. `deadcode -test ./...` reports nothing. Without `-test`, deadcode flags only test helpers here (`testfixture.go:31-131`), which is expected. |
| Domain fit | green | No domain terms, apart from `i18n`'s display languages, which match GLOSSARY "Display languages". |
| Docs match | amber | `docs/architecture.md:62` says "city-pack apply opts out" of the deadline. `cmd/api/limits.go:39-42` shows it no longer does (it's a River job). The config rows for `TRANSLATOR=google` (`backend/README.md:83-106`) go away with the removal. |
| Defect history | amber | Four fixes, all from 2026-10-20 to 10-26, none open:<br>• `f561bcb` (errs leaked internal details)<br>• `974f1ae` (db: goose bookkeeping in `public`)<br>• `3a37d0b` (sentry tracing, config)<br>• `3eae8f6` (gemini endpoint)<br>Nothing since. |
| Refactor load | green | No frozen refactor ticket targets `platform`. |

**Cost to the bar:**
- Renovate: 0.5 session (tests for `pgconv` and `googleapi`, the architecture.md line).
- Rebuild: about 5 sessions (2,600 lines of tested plumbing).

**Verdict:** Keep. There are no reds, and it costs 2 sessions or fewer.

## B. `platform/github` and `repoedit`

**Code:** `platform/github` 292/219, `repoedit` 341/430.

**A scope conflict.** #417 parks `repoedit` and `platform/github`. It also keeps "UI-string versions written by people, fetched from `main`". That fetch runs through these packages:
- `cmd/api/main.go:171-176` builds `repoedit.New(github.New(...))`.
- `main.go:220` passes it to `uiCatalog.Start`.
- The worker does the same at `cmd/worker/main.go:68-74`.
- `internal/uistrings/follow.go:50-52` calls `repoedit.TokenRefused` to fail production on a refused token.

The in-scope path uses only `Repo.Read` and `Repo.List` (`repoedit.go:110`, `:127`), through the `uistrings.HumanSource` interface. The write path is what the parked editors use:
- `Change` (`:264`), `Pending` (`:185`), `Files` (`:219`)
- the per-key locks

Both paths sit in one package, so #413's split rule (package boundaries only) doesn't allow a split.

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | green | 430 test lines for 341 in `repoedit` (rolling PRs, rate-limit vs refused token at `repoedit_test.go:393`); 219/292 in `github`. |
| Structure | green | Behind the `repoedit.Host` interface (`repoedit.go:35`). deadcode is clean. |
| Domain fit | green | GLOSSARY "Pending change" matches `Repo.Change`'s one rolling PR per key. |
| Docs match | green | backend README package rows; architecture.md:143. |
| Defect history | green | No fix commits since 2026-10-20. |
| Refactor load | green | None. #351 (renew `GITHUB_TOKEN` by 2027-01-22) is ops, and it matters for the in-scope follow. |

**Cost:**
- Keeping both packages in scope as they are: 0.5 session.
- Moving the read path into `uistrings` on top of `platform/github` so `repoedit` can be parked: about 1 session.
- Rebuild: 2.

**Verdict:**
- Recommended: **keep both in scope.** Park only their callers (the translation editor and CityEditor), not the packages. Parking them as #417 lists is not possible while the follow path depends on them.
- If the owner prefers a clean park: move `Read`, `List` and `TokenRefused` into `uistrings`, then park `repoedit` (1 session).

## C. Worker (`internal/worker` 524/524, `cmd/worker` 109/167)

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | green | `worker_test.go:75` (every kind sets its queue, timeout and attempts), `:115` (every kind is registered and unique), `:178` (notify runs while passes are busy), `:228`/`:272` (refresh_maps fan-out and per-city failure). `admin_jobs_test.go:51` (the admin-job lifecycle). `cmd/worker/main_test.go:118`, `:150`, `:160` (every defined kind is registered; fails without config or a database). The job bodies delegate to domain runners, which are tested in their own packages (e.g. plans' resilience tests). |
| Structure | amber | One 524-line file wiring 11 kinds. It imports 8 domain packages (`worker.go:20-29`), which is its job as the composition layer. The item-failure policy is copied between `plans/run.go:481` and `crews/pass.go:134`, and the copies have drifted (#405). |
| Domain fit | amber | `crews_city` runs `crews.Runner.Pass`, which still has the automatic offer stage: `worker.go:180-182` says "offer new crews", calling `crews/pass.go:126` → `offer` at `:152`. #417 says the server never offers a crew by itself. The fix belongs to crews (#424); the worker only stops calling that stage. |
| Docs match | green | `backend/README.md:263` matches `queues()` (`worker.go:61-66`) and each kind's timeout and attempts. |
| Defect history | amber | `dcdde08` (#223, one item never stops a pass, 2026-11-03) and `9224ace` (#134, shutdown drain and crews_city retries, 2026-11-02). No open bug. |
| Refactor load | amber | #405 (one stage runner, size S). |

**Sentry:** `worker.go:506` sets `ErrorHandler: sentry.JobErrors{}`. Item failures report with tags (`plans/run.go:483`, `crews/pass.go:137`).

**Cost:**
- Renovate: 1 session (#405, plus removing the offer call with #424).
- Rebuild: 3.

**Verdict:** Keep. There are no reds, and it costs 2 sessions or fewer.

## D. API server (`cmd/api` 347/181)

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | CORS (`cors_test.go:9`), request size, deadlines, timeouts, and the WebSocket outliving server timeouts (`limits_test.go:27-97`); validation after auth (`validate_order_test.go:20`). **Not pinned:** which services are public (`main.go:123`), admin-only (`:125`) or behind the gate (`:131`). `auth_test.go` tests the interceptors in isolation, and CI's "Binaries start" step (`ci.yml:374`) only checks start-up. |
| Structure | amber | `run()` (`main.go:92-262`) wires 19 services inline, including the parked ones: tagging (`:152-157`), the translation editor (`:209-224`), and `repoedit` for the editors (`:171-179`). There's no rate-limit layer: no `ratelimit`, `x/time/rate`, Turnstile or captcha anywhere under `backend/`. |
| Domain fit | green | Wiring only. |
| Docs match | amber | The warning at `main.go:150` names `GOOGLE_TRANSLATE_API_KEY` first, though the default is Gemini. `docs/architecture.md:62` (see A). |
| Defect history | amber | `6d64b3d` (CORS allowed every origin when none was set), `f561bcb`, `9224ace`, `3a37d0b`, all touching `main.go`. No open bug. |
| Refactor load | amber | #402 (Standing) rewires the gate (`main.go:131-132`) and chatws's `Suspended` (`:231`). |

**Waitlist RPC (#417, new).** This is feature work, not hygiene, but it lands here:
- The public, unauthenticated RPC has to join the public list at `main.go:123`.
- It needs rate limiting and a bot check, and the repo has neither today.
- It needs a table in `app` (a goose migration, part H) and an admin list RPC (`auth.AdminOnly`, `main.go:125`).

**Cost:**
- Renovate: 1 session (unwire the parked services or put them behind config; a table test of the procedure → interceptor mapping; the log text).
- Rebuild: 2.

**Verdict:** Keep.

## E. `cmd/tool` (1,361/794)

**Subcommands:**
- In scope: `migrate`, `seed`, `build-matrix`, `discover-venues`, `area-boundaries`, `email-templates`, `set-admin`, `ui-strings`, `ui-bundle`, `sentry-test`.
- Parked: `simulate`, `set-translator`.
- Removed: `walk-bench` (#417) and `translate-compare` (it follows from removing `translate/google`).

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | The writes are tested as functions against fakes and the throwaway database (`setadmin_test.go`, `boundaries_test.go`, `emailtemplates_test.go`, `geo_test.go`, `seed_test.go`; `backend/README.md:407`). **Untested** in-scope subcommands: `ui-strings` (83 lines), `ui-bundle` (123) and `sentry-test` (29). |
| Structure | amber | `main.go` (360 lines) holds the dispatcher plus the `seedPacks`, `buildMatrix`, `discoverVenues` and `simulate` bodies (`main.go:155-301`). Removed and parked code sits in the same `main` package. |
| Domain fit | green | — |
| Docs match | amber | `docs/architecture.md:70` says "migrations and seeding" (the tool has 14 subcommands). The package comment (`main.go:1-2`) says "simulating matching". `backend/README.md:11` says `email-templates` "needs `GOOGLE_TRANSLATE_API_KEY`", but it uses `translators.New` (`emailtemplates.go:25`), which defaults to Gemini. |
| Defect history | amber | One open bug: #290 (`discover-venues` keeps what it found when a search fails; ready-for-human). No fix commits under `cmd/tool` since 2026-10-20 other than `3a37d0b`, a one-line `sentrytest.go` change. |
| Refactor load | green | None (#406 touches `sim`, which is parked). |

**Cost:**
- Renovate: 1 session (the two removals, `db-checks.sh:31` after Tokyo goes, the docs, small tests for `ui-strings`/`ui-bundle`).
- Rebuild: 2.

**Verdict:** Keep, with the removals.

## F. `adminjobs` (342/221)

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | green | `handler_test.go:54-213` (Describe, GetAdminJob, classify, discover without a key, apply hash, insert opts). `worker/admin_jobs_test.go:51` (lifecycle against River). |
| Structure | green | No table of its own; uses River's job output (`jobs.go:8-11`). |
| Domain fit | green | — |
| Docs match | green | `backend/README.md:48`. |
| Defect history | green | None. |
| Refactor load | green | None (#320, preview an apply, is a feature). |

**Cost:** renovate 0.25 session, rebuild 1.5. **Verdict:** Keep. It's in scope as long as `ApplyCityPack` or `DiscoverAreaVenues` stay (see the open question under Scope cuts).

## G. `metrics` (99/151)

#413 lists metrics in this shared layer. #426's parts list doesn't name it, but it's included here for completeness.

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | green | `handler_test.go:18` (Monday), `:32` (non-admin refused), `:43` (`TestIntegrationMetrics`). |
| Structure | green | The definitions are SQL in `db/queries/metrics.sql`. |
| Domain fit | amber | It reads `app.attendance.went_ahead` (`metrics.sql:20`), which #331 shows counts an `inviting` group at a finished outing as having gone ahead. People counting re-derives city membership by hand (`metrics.sql:54-56`, `:67-69`), the pattern #401 removes. |
| Docs match | amber | Four decision-log rows and 11 comment lines (inventory §10). The admin README describes the page. |
| Defect history | green | No fix commits. |
| Refactor load | amber | #331, #401 (via the queries). |

**Pilot gate gap:** Metrics doesn't show the success bar (#417):
- the share of matched groups that go ahead: `groups` counts only those that went ahead (`metrics.sql:33`);
- the share of attendees who attend again: `repeat_attendees` exists, but there's no share over the Pilot;
- outings lost to a bug: no data source.

That's gate work, not part of the hygiene cost.

**Cost:** renovate 0.5 session (after #331/#401 land), rebuild 1. **Verdict:** Keep.

## H. Schema: migrations, domain views, `db/queries`, sqlc

**Code:**
- `backend/db/migrations/00001_baseline.sql` (769 lines, 36 tables, all in `app`)
- `00002_river.go`, registered from `backend/db/river.go:28` (there is no file of that name in `migrations/`, by design)
- `00003_domain_rule_views.sql` (56 lines: `app.attendance`, `app.suspended_users`, `app.kept_apart`)
- `db/queries`: 21 files, 2,059 lines, 242 queries; generated `gen/dbgen`, about 8,700 lines

This is one part: every frozen ticket touches both migrations and queries, so there's no boundary that splits it into independently doable verdicts.

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | green | CI runs up → down → up and seeds every pack (`scripts/db-checks.sh:21-24`). It also checks that migration bookkeeping stays out of `public` (`ci.yml:258`). `TestIntegrationDomainRuleViews` (`testfixture/rules_test.go:70`) pins the three views. The queries are exercised by each domain's `TestIntegration*` tests (`db-checks.sh:29`, `-run Integration` over `./...`). sqlc vet runs in CI lint. |
| Structure | green | A squashed history; the `app` and `goose` schemas; views are `security_invoker` with no grants (`00003:8-10`). Queries are filed by domain. Two queries have no production caller: `UpsertVenue` (`db/queries/geo.sql:41`, test only; `UpsertVenues` at `:76` replaced it) and `GetPlan` (`db/queries/plans.sql:73`, tests only). deadcode skips generated code, so it doesn't report them. |
| Domain fit | amber | `app.attendance.went_ahead` is "done and not cancelled" (`00003:25`), while the feedback rule is "group confirmed" in three places (`plans/membership.go:146-156`, `events.sql:29-30`, `feedback.sql:116-121`). A group still `inviting` at a finished outing counts as gone ahead (#331). City membership plus "not suspended" is re-derived in about 8 queries, and the copies disagree (#401). Table names follow GLOSSARY: "Outing" is `events` in code, as the glossary allows; `mate_picks` holds the "rather not" picks. There's no waitlist table yet (new work under #417). |
| Docs match | amber | `backend/README.md:209-222` is accurate. `docs/architecture.md` is not:<br>• `:92` names `00001_init.sql` and `00002_geo.sql`<br>• `:97` says an area is "a neighbourhood or station"<br>• `:109` cites `00026`<br>Only the baseline, `00002_river.go` and `00003` exist. No doc lists the full schema (inventory §11). |
| Defect history | amber | `974f1ae` (goose bookkeeping in `public`, 2026-10-20). The open bug #321 (`ListMetForPicks` uses its own attendance rule). #331 describes a live defect (stuck `inviting` groups), filed as a refactor. |
| Refactor load | **red** | Four frozen tickets, one fixing a live defect:<br>• #331 (`goes_ahead`, feedback due, stuck groups)<br>• #401 (M: `app.participants`)<br>• #332 (`app.plan_cards`; may close as wontfix after #326)<br>• #321 (the attendance rule in `feedback.sql`)<br>#402 also moves the `ListPushTokens` suspended filter (`notifications.sql`) onto Standing. |

**Cost to the bar:**
- The bar alone is about 1 session: delete the two unused queries, fix architecture.md's data-model section.
- With the frozen tickets, renovating is about 3 sessions: #321 0.5, #331 1, #401 1, #332 0.5 or wontfix.
- Rebuilding is about 10: re-derive the 769-line baseline, 242 queries and their callers in every domain.

**Verdict:** Renovate.
- Keep is out: refactor load is red.
- Rebuild is out: domain fit is amber (the model is right; its rules are copied), and the tests are green.
- Cost check: renovating (3) is cheaper than rebuilding (10).
- Breaking changes are free before the Pilot, so #331's behaviour change and #402's view changes cost only the work.

## I. `proto/` (18 files, 2,864 lines, 84 RPCs)

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | green | `buf lint` (`ci.yml:154`), `buf breaking` (`ci.yml:187`, skipped with `breaking-ok` per #414), and the codegen drift check (`ci.yml:156`). `buf.validate` limits are enforced and tested (`platform/validate/validate_test.go`; `api-client/src/limits.test.ts`). Behaviour behind the RPCs is the slices' (see the handler-level gap below). |
| Structure | amber | One file per service. `review.proto` (498 lines, 21 RPCs) is the largest. Parked surface stays in the contract: `TranslationService` (9 RPCs), `SuggestActivities`, `GetCityPack`, `ProposeCityPack`. Retired fields are `reserved` (e.g. `moderation.proto:309`, `review.proto:341`). |
| Domain fit | amber | Event naming is allowed by GLOSSARY "Outing". GetMe's `onboarded`/`suspended` booleans are to become one `standing` enum (#402, breaking). `MyEvent` lacks the caller's allowed actions (#329). Crew offers will be admin-initiated (#417), which may need an admin RPC (#424). |
| Docs match | amber | `proto/README.md` "Rules" says "No breaking changes once a field ships in an app build. CI runs `buf breaking` against `main` on every PR". It doesn't mention the `breaking-ok` label (`docs/contributing.md:74`) or #414's rule that breaks are free until the Pilot and the label goes after it. The rest matches (paging, limits, "authenticated by default"). |
| Defect history | green | Two fixes touched `.proto` files (`3a37d0b` identity, `3eae8f6` catalog, 2026-10-21/26). Neither was a contract defect. |
| Refactor load | amber | #402 (breaking), #329. Feature tickets #396–#399 add fields. |

**Cost:**
- Renovate: 1 session (the README rule, remove or mark the parked RPCs' comments, keep breaks within the `breaking-ok` window).
- Rebuild: about 4 (re-spec 84 RPCs, regenerate, update both clients).

**Verdict:** Keep.

## J. `packages/api-client` (123/57)

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | `errors.test.ts` and `limits.test.ts` cover `errorReason` and `maxLen`. Nothing tests `createApi`'s auth interceptor (`src/index.ts:58-62`: the Bearer header only when there is a token). |
| Structure | green | 87-line `index.ts`; the rest is generated (`src/gen`). |
| Domain fit | green | — |
| Docs match | amber | `README.md:3` calls the admin SPA "(planned)"; it exists (inventory §10). |
| Defect history | green | None. |
| Refactor load | green | None. #319 (pnpm lockfile alerts) is repo-wide. |

**Cost:** renovate 0.25 session (one interceptor test, the README line), rebuild 0.5. **Verdict:** Keep.

## Hygiene-bar Pilot-gate gaps in this area

1. **Dead-code check (gate: passes on every unparked part).** No CI job runs Go `deadcode` or `knip`. `.golangci.yml` has only `unused`, which catches unexported code only. Results at the snapshot:
   - `deadcode -test ./...`: **0 findings.**
   - `deadcode ./...` (no tests): 29 findings.
     - 25 are test helpers (`maps/mapstest`, `testfixture`), as expected.
     - 4 are production code reachable only from tests, all outside this area: `maps/google/placetypes.go:26` `IsPlaceType`, `plans/run.go:48` `DefaultTiming`, `plans/store.go:115` `Create`, `plans/store.go:248` `LoadSignups`.
   - Parked and removed code is reachable from the three `main`s, so deadcode won't flag it. The check needs either the parked wiring removed or a documented exemption list.
   - deadcode skips generated code, so the two unused sqlc queries (`UpsertVenue`, `GetPlan`, part H) need their own check, e.g. a script like this audit's caller count.
   - `knip` was not run: there's no knip config and no `node_modules` in this worktree. `api-client` re-exports every generated module (`src/index.ts:25-43`); knip would need those `export *`s marked as the public API.
2. **Every in-scope RPC has a handler-level test.** 84 RPCs. Leaving out the 12 parked ones (`TranslationService` 9, `SuggestActivities`, `GetCityPack`, `ProposeCityPack`) leaves 72 in scope. **29 of those have no handler-level test** (a test calling the real handler with a `connect.Request`). Most are the app's core-loop RPCs, tested only at service level today:
   - `CatalogService`: ListActivities, ListCities, ListLanguages
   - `ChatService`: ListMessages, SendMessage (`cmd/api/validate_order_test.go` hits an `Unimplemented` handler)
   - `CrewService`: all 5
   - `EventService`: all 7 (CheckIn and SubmitFeedback are tested only through `plans.Service`)
   - `IdentityService`: GetMe (gate tests use an `Unimplemented` handler, `moderation/suspensions_test.go:62-75`), DeleteAccount (service-level, `identity/service_test.go:100`)
   - `PlanService`: all 5
   - `ProfileService`: both
   - `PushService`: both (RegisterPushToken only through `Unimplemented` handlers in the gate and auth tests)
   - `ReviewService`: ListVenueOptions

   The admin services (Moderation, Review apart from one RPC, Curation, CityPack, Metrics, Admin) are covered. These tests belong to the slices' audits (#421, #423, #424, #425). In this area, the gap is that there's no shared handler-test harness: tests set the caller with `auth.WithCaller` (`platform/auth/auth.go:109`) package by package.
3. **Errors reach Sentry, with an alert to the owner.**
   - Wired:
     - API unary errors: Internal and Upstream (`platform/errs/errs.go:113-121`).
     - API panics (`platform/sentry/sentry.go:65`).
     - Worker jobs (`worker.go:506`).
     - Pass items (`plans/run.go:483`, `crews/pass.go:137`).
     - Notification renders (`notifications.go:183`).
     - List caps (`platform/bound`).
   - **Gaps:**
     - `/ws/chat` internal errors (`chatws/chatws.go:261`) and the LISTEN hub's failures (`chatws.go:145`) are only logged.
     - The alert rule is Sentry-side, not in the repo. `docs/deploy.md:75` notes that rules filtering on environment `production` don't fire while `APP_ENV` is `preview`.
     - The backup alert is still #239.
4. **The site's waitlist RPC (#417 gates before it goes live).** None of this exists yet:
   - the public procedure (`cmd/api/main.go:123`)
   - rate limiting or a bot check (nothing under `backend/`)
   - an `app` table and migration
   - an admin list RPC
5. **Docs contradictions to fix (gate: inventory contradictions).**
   - In this area:
     - `docs/architecture.md:62`, `:70-71`, `:92`, `:97`, `:109`, `:127`
     - `backend/README.md:11` (the email-templates key), `:278` ("Not done yet: chat, push…")
     - `packages/api-client/README.md:3`
     - `proto/README.md` (the `breaking-ok` window)
     - the warning texts at `cmd/api/main.go:150` and `cmd/worker/main.go:61`
   - Removals make more rows stale: `backend/README.md:38`, `:268` (walk-bench) and `:83-106` (TRANSLATOR).
6. **`buf breaking` after the Pilot.** The `breaking-ok` skip exists (`ci.yml:169`). The Pilot-start step (remove the label, make `proto-breaking` required) has no ticket yet. #402's breaking GetMe change should land before then.
7. **Parked parts compile and pass.** `go build ./...` passes. `go test` (no database) passes for every package in this area and for `matching/sim`, `walks/bench`, `walks/claude`, `translations` and `translate/google`. Integration tests weren't run here; CI runs them.

## Recommendations

- **Keep:** A, B (in scope, not parked), C, D, E, F, G, I, J.
- **Renovate:** H. Do #321, then #331, then #401, and decide #332 (wontfix if #326 covers it). Fix architecture.md's data-model section in the same work.
- **Remove:** `tool walk-bench` and `tool translate-compare`; `TRANSLATOR`/`GOOGLE_TRANSLATE_API_KEY` from config, with #425's removal of `translate/google`.
- **Park:** `tool simulate` and `tool set-translator`. Unwire or config-gate the parked services in `cmd/api/main.go` (tagging, the translation editor), so the dead-code check can tell parked from dead.
- **Owner questions for the verdict ADR:**
  1. Do `repoedit` and `platform/github` stay in scope, since the UI-string follow needs them? Or should their read path move first?
  2. Is the admin Cities page (`ListCityPacks`, `ApplyCityPack`, `DiscoverAreaVenues`) in scope? Or does the owner apply packs with `tool seed`?
  3. Should the dead-code check run `deadcode -test`, or without `-test` plus an exemption list?
