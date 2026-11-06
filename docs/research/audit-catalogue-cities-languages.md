# Audit: catalogue, cities and languages

Research for [#425](https://github.com/shrujancheruku/issho/issues/425), a child of the map [#411](https://github.com/shrujancheruku/issho/issues/411). It scores each part of this area on the six signals from [#413](https://github.com/shrujancheruku/issho/issues/413), estimates renovate and rebuild costs to reach the hygiene bar in [#414](https://github.com/shrujancheruku/issho/issues/414), and recommends a verdict. The scope comes from [#417](https://github.com/shrujancheruku/issho/issues/417). This doc holds facts and recommendations only; it changes no product code.

**Snapshot:** `main` at `139d77e` (2026-11-06). CI on that commit is green (run 42159804293: backend, typescript, codegen, image). Locally, `go build ./...` passes, and `go vet` plus `go test` (unit tests, no database) pass for every Go package named below.

**How things were counted**
- "Src/test" are non-generated line counts from `wc -l`. "Test" means `*_test.go`, `__tests__/` and `*.test.*`.
- Defect history is `git log --since=2026-10-20` on each part's paths, plus open `bug` issues.
- Costs are in agent sessions: one session is one ticket worked end to end by one agent, PR included.

**Verdict rules (#413):**
- **Keep:** no reds and at most ~2 sessions of work.
- **Rebuild:** domain fit is red, or tests and structure are both red.
- **Renovate:** everything else.
- **Cost check:** the cheaper estimate wins. On a tie, renovate if tests are amber or green.
- Docs, defects and refactor load only move a part between keep and renovate.
- A part may be split only along a package or module boundary.

## Summary

| Part | Tests | Structure | Domain | Docs | Defects | Refactor load | Renovate / rebuild (sessions) | Verdict |
|---|---|---|---|---|---|---|---|---|
| `catalog` | amber | amber | green | amber | green | amber | 1.5 / 3 | **Keep** |
| `curation` + admin Venues page | green | green | green | green | green | green | 0.25 / 4 | **Keep** |
| `cities` | green | amber | green | amber | amber | amber | 2.5 / 6 | **Renovate** |
| `citypacks` + admin Cities page | green | amber | green | amber | green | amber | 1.5 / 3.5 | **Keep** (if the read side of repoedit stays; see Q1) |
| `data/` Da Nang–Hoi An and Austin packs | green | green | green | amber | green | amber | 0.5 / n.a. | **Keep** |
| `data/citypacks/tokyo.json` | n.a. | n.a. | n.a. | n.a. | n.a. | n.a. | 1 (removal) | **Cut: remove** (#417) |
| `translate` + `translate/gemini` + `translate/translators` | green | amber | green | amber | green | amber | 1 / 3 | **Keep** |
| `translate/google` (+ `tool translate-compare`) | n.a. | n.a. | n.a. | n.a. | n.a. | n.a. | 0.5 (removal) | **Cut: remove** (#417) |
| `uistrings` | green | amber | green | amber | green | amber | 1.5 / 4 | **Keep** (same condition as `citypacks`) |
| `emailtemplates` (+ `tool email-templates`) | green | green | green | amber | green | green | 0.25 / 1.5 | **Keep** |
| mobile `src/i18n` | green | green | amber | amber | amber | amber | 1.5 / 4 | **Keep** (the Vietnamese work is new Pilot work, not renovation) |

No part has a red domain fit, and no part has red tests and red structure together, so nothing qualifies for rebuild. The only part over the ~2-session keep line is `cities`.

## Cross-cutting findings

These findings affect several parts, and #417 doesn't settle them. Each one ends in an owner question.

1. **Two in-scope paths run through parked `repoedit` and `platform/github`.**
   - **Applying a pack from the Cities page.** `ApplyCityPack` reads the pack from the default branch with `h.Repo.Read` (`backend/internal/citypacks/handler.go:71-78`). `ListCityPacks` reports `Configured: h.Repo != nil` (`handler.go:88`). The Cities page shows "Apply from main" only when `configured && p.inRepo` (`apps/admin/src/pages/Cities.tsx:185-190`). Without `GITHUB_TOKEN`, the page shows an error and offers only "Find venues" (`Cities.tsx:29`, `:159`).
   - **Following human UI strings on main.** `uistrings/follow.go` imports `repoedit` (`follow.go:11`). The API starts the five-minute follow only when the repo is configured, and production stops on a refused token (`backend/cmd/api/main.go:217-222`). #351 tracks renewing `GITHUB_TOKEN` before 2027-01-22.
   - **The conflict.** #417 keeps both features ("open an area through a PR to the pack", "human UI-string versions fetched from main") and also parks `repoedit` and `platform/github`.
   - **The way out.** The two features need only the read side (`Repo.Read`, `Repo.List`, and the token check). The write side (`Change`, `Pending`, `Files`) serves only the parked CityEditor and translation editor.
   - **Recommendation:** count `repoedit` and `platform/github` as "in scope, read-only". The write side stays compiled and tested but unused in the Pilot.
   - **Alternative:** drop "Apply from main" for `tool seed` run through `kamal app exec` (the image ships `data/` at `/data`, `backend/Dockerfile:38`), and serve the human strings embedded at build time. This costs about 1.5 sessions more, and a strings fix then needs a deploy.
2. **The admin Cities page is not in #417's admin list,** yet it is the only UI for "Apply from main" and "Find venues", the venue discovery that #417 keeps. Recommendation: put it in scope. It already has tests (`Cities.test.tsx`, 206 lines).
3. **Parked editor code sits inside in-scope packages,** so it can't be split off as its own part under the package-boundary rule:
   - `cities/edit.go` (241 lines: `Propose`, `Changes`, `keepFileOnly`, `keepOverrideNames`). `CheckPack` (`edit.go:31`) is shared with apply.
   - `uistrings/editor.go` (314 lines).
   - `translate/review.go` (101 lines, `Reviewer`).

   They stay reachable as long as `CityPackService.ProposeCityPack` and `TranslationService` are registered. If parking means unregistering those handlers, the dead-code gate (#414) flags all three. Recommendation: keep them registered and admin-only, or move them into the parked packages (`citypacks`/`translations`) in the same change that unregisters them. `CheckPack` stays in `cities`.
4. **`catalog` depends on parked `tagging`.** `catalog.Handler` holds `Suggestions *tagging.Service` (`backend/internal/catalog/handler.go:22-23`) for `SuggestActivities` (`suggest.go:21`). If tagging is parked, `SuggestActivities` has to be parked with it: either left returning its "unavailable" reason (already tested in `suggest_test.go`) or removed from the proto. Decide with the matching/tagging audit.
5. **Vietnamese isn't wired yet.** #417 makes the Pilot English and Vietnamese only, but:
   - The bundled offline strings are `{ es, ja }` (`apps/mobile/src/i18n/bundled/index.ts`).
   - There is no `human/vi.json`.
   - `tool ui-bundle` (`backend/cmd/tool/uibundle.go`) bundles the city packs' languages. Today those are Tokyo's `ja` and Austin's `es`; Da Nang's `vi` is missing because the bundle predates the pack.
   - `CatalogService.ListLanguages` returns all ISO 639-1 codes (`backend/internal/catalog/languages.go:16`; `backend/README.md:149`), and the mobile picker lists them all (`apps/mobile/src/features/profile/language-picker.tsx:13`). "Left out of the picker" isn't implemented.
   - #352 (font and RTL check) still asks for a Vietnamese font check.
6. **Removing Tokyo breaks tests and checks,** listed under Removed parts below. The live walk eval, which #417 keeps, reads only the Tokyo and Austin packs (`backend/internal/walks/gemini/eval_test.go:48-66`) and has no Da Nang cases.

## Per part

### `catalog`: Keep

`backend/internal/catalog`, 261 src / 100 test lines. RPCs: `ListActivities` (`handler.go:26`), `ListCities` (`cities.go:15`), `ListLanguages` (`languages.go:16`) and `SuggestActivities` (`suggest.go:21`).

| Signal | Rating | Evidence |
|---|---|---|
| Test quality | amber | `languages_test.go` tests the pure helpers (`Languages`, `stored`, `withTraditionalChinese`). `suggest_test.go` tests only the unavailable reason. There are no handler-level tests for `ListActivities`, `ListCities` or `ListLanguages`, which is a #414 gate. |
| Structure/coupling | amber | Depends on parked `tagging` through `Handler.Suggestions` (`handler.go:23`). Otherwise thin. |
| Domain fit | green | City overrides and area names "only as stored" match the glossary's Area and Display languages. |
| Docs match | amber | Comments use Tokyo examples (`cities.go:29`, "Tokyo → Tokio", 吉祥寺). The README says ListLanguages returns "every ISO 639-1 language" (`backend/README.md:149`), which the Pilot's en/vi picker will contradict. |
| Defect history | green | No fixes on these paths since 2026-10-20. |
| Refactor load | amber | #357 (activities with no venue category); #406 (citydata, L) renames the loaders around it. |

- **Renovate, about 1.5 sessions:** integration handler tests for the three RPCs (1); park or settle `SuggestActivities` with tagging, and fix the Tokyo comments (0.5).
- **Rebuild, about 3 sessions.**
- **Verdict: Keep.** No reds and at most 2 sessions. The list above is the work.

### `curation` and the admin Venues page: Keep

`backend/internal/curation`, 636 src / 604 test lines (`service.go` 438, `handler.go` 167, `flag.go` 31). `apps/admin/src/pages/Venues.tsx` 285 / `Venues.test.tsx` 169.

| Signal | Rating | Evidence |
|---|---|---|
| Test quality | green | `TestIntegrationHandler` covers all six RPCs (ListVenueCities, SetApprovedVenuesOnly, ListVenues, SetVenueStatus, SearchVenues, AddVenue). Also `TestNonAdminRefused`, `TestFits`, `TestNearest`, `TestIntegrationService`, `TestIntegrationUpstreamErrors`, `TestVenueStatusProtoIsStrict` and `TestFlag`. The Venues page has render and interaction tests. |
| Structure/coupling | green | The handler only checks the admin and maps protos; rules live in `Service`. Imports are geo, maps, paging, errs and auth only. |
| Domain fit | green | Curation is per place, matching the Pilot's "Venues curation page". |
| Docs match | green | `backend/README.md:50`, `:171` and `:395-396` match the code. |
| Defect history | green | One 1-line change in #134 (`9224ace`). |
| Refactor load | green | None frozen. |

- **Renovate, about 0.25 sessions:** Tokyo strings in `Venues.test.tsx` once the pack goes.
- **Rebuild, about 4 sessions.**
- **Verdict: Keep.**

### `cities`: Renovate

`backend/internal/cities`, 1,288 src / 846 test lines (`pack.go` 429, `validate.go` 273, `edit.go` 241, `apply.go` 201, `removals.go` 144).

| Signal | Rating | Evidence |
|---|---|---|
| Test quality | green | Integration tests cover apply, matching, the baseline schema, authoritative removal and plan areas (`TestIntegrationApply`, `TestIntegrationApplyMatching`, `TestIntegrationBaselineSchema`, `TestIntegrationAuthoritative`). Unit tests: `TestDiffAreas`, `TestDataFiles`, `TestValidatePack*`, `TestCategoryLabels`, `TestWKT`, `TestPlanAreas`, `TestChanges`, `TestKeep*`. |
| Structure/coupling | amber | `Apply` (`apply.go:73`) runs no checks, so each caller picks its own validators: `tool seed` (`cmd/tool/main.go:169`), the admin handler (`citypacks/handler.go:236`) and the worker (`adminjobs/jobs.go:179`), which checks nothing (#407). The parked editor logic `edit.go` lives in the same package. |
| Domain fit | green | Areas are local government units with official boundaries and names, enforced by validation and documented in `data/README.md:123`. |
| Docs match | amber | `pack.go:57-59` says "Tokyo's areas are named after stations", which is out of date (Tokyo moved to municipalities). `docs/architecture.md:97` says an area is a "neighbourhood or station", against GLOSSARY.md:16. |
| Defect history | amber | #407 is an open bug. |
| Refactor load | amber | #407 (S), #320 (preview an apply on the Cities page), #406 (citydata, L, touches the loaders next door). |

- **Renovate, about 2.5 sessions:**
  - #407: move the checks into `Apply` (1).
  - Tokyo-removal fallout: comments; `TestDataFiles` still has its two packs (Austin and Da Nang) (0.5).
  - Move `edit.go` with the editor, or document it as admin-only (0.5).
  - Docs (0.5).
- **Rebuild, about 6 sessions:** seeding, removals, matrix rebuild and PostGIS boundaries, plus the integration suite.
- **Verdict: Renovate.** No reds, so not a rebuild. Renovating is cheaper, but at about 2.5 sessions it is over the keep line. #407 is the deciding item.

### `citypacks` and the admin Cities page: Keep (conditional)

`backend/internal/citypacks/handler.go`, 340 src / 561 test lines. RPCs: ListCityPacks, GetCityPack, ProposeCityPack (CityEditor, parked), ApplyCityPack and DiscoverAreaVenues. `apps/admin/src/pages/Cities.tsx` 194 / `Cities.test.tsx` 206.

| Signal | Rating | Evidence |
|---|---|---|
| Test quality | green | `TestIntegrationCityPacks` covers all five RPCs with `mapstest` and a fake GitHub. `TestIntegrationApplyRefusesInUseArea` checks the in-use refusal. The Cities page has render and interaction tests. |
| Structure/coupling | amber | In-scope Apply needs parked `repoedit` (cross-cutting finding 1). Parked Propose and Get share the handler. The check is red if repoedit is parked whole, amber if its read side stays. |
| Domain fit | green | Owner opens an area by a PR to the pack, then applies from main: exactly #417's flow. |
| Docs match | amber | The package doc says "(M8)", a retired milestone (`handler.go:2`). `docs/architecture.md:143` describes the editor flow as the way cities change. |
| Defect history | green | One fix since 2026-10-20: `ca62ccd` (#81) keeps the admin's email out of city-pack PRs. |
| Refactor load | amber | #320 (preview an apply). |

- **Renovate, about 1.5 sessions:** hide or refuse ProposeCityPack and the Edit/New-city links in the Pilot build, and keep the read side (0.75); doc fixes and the Tokyo strings in tests (0.75).
- **Rebuild, about 3.5 sessions.**
- **Verdict: Keep,** given Q1 is answered "repoedit read side in scope".
- If the owner parks repoedit whole, structure is red and the verdict is **Renovate**, about 2.5 sessions: Apply reads the pack from the image's `/data` or from `tool seed` instead.

### `data/` packs: Keep Da Nang–Hoi An and Austin; remove Tokyo

- **`da-nang-hoi-an.json`**: 15 wards with OSM boundaries, languages `vi` and `en`, walk and drive. It passes `TestDataFiles`.
- **`austin.json`**: 2.6 KB, marked synthetic ("Not a launch city"). It **is** a test fixture, so per #417 it stays. Users:
  - `TestDataFiles` needs at least 2 packs (`backend/internal/cities/data_test.go:43`), and cross-pack `ValidatePacks` only means something with two.
  - `scripts/db-checks.sh:36` runs `discover-venues -dry-run austin`.
  - The live walk eval reads it (`walks/gemini/eval_test.go:48,66`).
  - `matching/sim/sim_test.go` reads it, but sim is parked.
- **Docs:** amber. `data/README.md:9` calls Tokyo the "launch city", `:121` says to copy `tokyo.json`, and `docs/architecture.md:16` says "The first city is Tokyo".
- **Renovate, about 0.5 sessions:** README and architecture text, plus #317 ("Rerun Tokyo venue discovery") closed as obsolete.
- **Verdict: Keep** the packs; the Tokyo pack is in Removed parts.

### `translate`, `translate/gemini` and `translate/translators`: Keep

- `translate`: 449 src / 400 test lines (`translate.go` 320, `review.go` 101, the guide).
- `translate/gemini`: 218 / 175.
- `translate/translators`: 32 / 31.

| Signal | Rating | Evidence |
|---|---|---|
| Test quality | green | `TestResolve*`, `TestBatches`, `TestPrefill`, `TestResolvePair*` (cache, de-duplication, background completion, failures) and `TestReviewerReview`. Gemini: `TestTranslate`, `TestTranslateMissingItem`, `TestGuide`, `TestDraft`, `TestDraftBudget`. |
| Structure/coupling | amber | `review.go` is translation-editor code (parked) in the core package. `translators` is a switch over `gemini`/`google` that shrinks to one case. |
| Domain fit | green | Gemini on-demand translation with a tone guide is what #417 keeps. |
| Docs match | amber | `docs/architecture.md:127` still says "Google Cloud Translation translates it now", already a contradiction in the inventory. `docs/google-maps.md:10,30,88` and `backend/README.md:19,406` describe the Google translator. |
| Defect history | green | No fixes since 2026-10-20. |
| Refactor load | amber | #350 (a livelier English tone guide). |

- **Renovate, about 1 session:** collapse `translators` once Google is removed (0.25); docs (0.5); move `review.go` with the editor if it's unregistered (0.25).
- **Rebuild, about 3 sessions.**
- **Verdict: Keep.**

### `uistrings`: Keep (conditional)

`backend/internal/uistrings`, 894 src / 426 test lines (`catalog.go` 360, `editor.go` 314, `human.go` 94, `follow.go` 89, `handler.go` 37).

| Signal | Rating | Evidence |
|---|---|---|
| Test quality | green | `TestGetStringsKnownVersion` is a handler-level test of `GetStrings`. Also `TestEmbeddedCatalog`, `TestIn`, `TestEmbeddedHuman`, `TestHumanVersions`, `TestStamp`, `TestSamePlaceholders`, `TestLoadHuman`, `TestStartRefusedToken` and `TestArguments`. |
| Structure/coupling | amber | `follow.go` depends on `repoedit` (cross-cutting finding 1). Parked `editor.go` sits in the same package. |
| Domain fit | green | Human versions win over machine text, as #417 keeps. |
| Docs match | amber | `backend/README.md:42` and `:145` are accurate for any language; the Pilot's en/vi limit isn't stated anywhere. |
| Defect history | green | No fixes on these paths since 2026-10-20. Mobile cache fixes are counted under mobile i18n. |
| Refactor load | amber | #325: `Catalog.Text` returns "" for an unknown key (`catalog.go:270-272`); add a test that the push keys exist. #315 (ja/es keys) is obsolete under en/vi. |

- **Renovate, about 1.5 sessions:** #325 (0.5); a `human/vi.json` path, or a decision that Vietnamese is machine text with a human review later (0.5); move `editor.go` with the translation editor if that is unregistered (0.5).
- **Rebuild, about 4 sessions.**
- **Verdict: Keep,** under the same Q1 condition as `citypacks`.

### `emailtemplates`: Keep

`backend/internal/emailtemplates` 150 src / 134 test lines; `backend/cmd/tool/emailtemplates.go` 93 / 138.

- **Tests: green.** `TestRender` and `TestRenderRejects` execute the generated templates with `text/template` and `html/template`, as Supabase Auth does. Also `TestSetSubjects` and `TestReadSource`.
- **Already includes Vietnamese.** `supabase/templates/source.json` has 20 languages including `vi`, and both templates branch on `"vi"` (`confirmation.html:58`).
- **Docs: amber.** `cmd/tool/emailtemplates.go:41,46` mention the Google translator.
- **Renovate, about 0.25 sessions:** drop that mention with the Google removal.
- **Rebuild, about 1.5 sessions.**
- **Verdict: Keep,** as #417 says ("stay as generated").

### Mobile `src/i18n`: Keep

`apps/mobile/src/i18n`, 641 src / 537 test lines (`i18n-provider.tsx` 226, `strings.ts` 203, `device.ts` 56, `cache.ts` 47, `direction.ts` 34, `shown.ts` 26, `bundle.ts` 15) plus the JSON catalogues.

| Signal | Rating | Evidence |
|---|---|---|
| Test quality | green | Tests for `ui-strings` (251 lines), `strings`, `sentences`, `i18n-provider`, `device`, `bundle`, `shown` and `direction`. |
| Structure/coupling | green | Pure helpers plus one provider. Names match the glossary's display languages and shown language (`shown.ts`). |
| Domain fit | amber | Built for any language: about 180 ISO codes in `device.ts:7-14`, RTL (`direction.ts:7`), zh-Hant and per-language fonts. The Pilot needs en/vi. That generality isn't wrong, but it is untested breadth the Pilot doesn't use. No Vietnamese bundle (cross-cutting finding 5). |
| Docs match | amber | `apps/mobile/README.md` names the Tokyo pack and ja/es bundles. |
| Defect history | amber | Four fixes since 2026-10-20: `9b93606` (stale UI text from the cache, 2026-10-22), `e0ca66d` (device languages, 2026-10-21), `3a37d0b` (#111, bundles) and `9224ace` (#134). The rest are copy-only `en.json` edits. |
| Refactor load | amber | #352 (Vietnamese font and RTL check); #315 obsolete. |

- **Renovate, about 1.5 sessions:**
  - Regenerate the bundles for `vi` (with Tokyo gone, `tool ui-bundle` follows Da Nang and Austin: `vi` and `es`).
  - Limit the picker to en/vi: a filter in the client or in `ListLanguages`.
  - The #352 font check.
  - Docs.
- **Rebuild, about 4 sessions.**
- **Verdict: Keep.** The en/vi work is Pilot work, not a hygiene failure.

## Parked parts: compile and test check

| Part | Check at `139d77e` |
|---|---|
| CityEditor (`apps/admin/src/pages/CityEditor.tsx` 220, test 65; `cities/edit.go`; `ProposeCityPack`) | Go builds, and `go test ./internal/cities/... ./internal/citypacks/...` passes, including `TestChanges`, `TestKeep*` and the propose path of `TestIntegrationCityPacks`. Admin tests pass in CI (`typescript` job green). |
| `repoedit` (341 / 430) | `go vet` and `go test` pass (`TestChange`, `TestBranchMoved`, `TestPending…`, `TestRead`, `TestTokenRefused`, `TestNotConfigured`, `TestGitHubTextNeverReachesTheCaller`). |
| `platform/github` (292 / 219) | `go vet` and `go test` pass. |
| Translation editor (`internal/translations` 267 / 660; `uistrings/editor.go`; `translate/review.go`; admin `Translations.tsx` 304) | `go test` passes (`TestPermissions`, `TestListAppLanguages`, `TestSaveAndRemove`, `TestEnglish`, `TestCatalog`, `TestDraftAppString`; integration tests in CI). `Translations.tsx` has **no test**, which is acceptable only while it stays parked. |
| Translator role (`tool set-translator`, `cmd/tool/settranslator.go` 64; `apps/admin/src/App.tsx:120-125` sends a translator to Translations only) | `go test ./cmd/tool/...` passes. |
| Other UI languages (bundled `es`/`ja`; `GetStrings` for any code) | `TestGetStringsKnownVersion` and the mobile `bundle`/`ui-strings` tests pass. Per #417 they stay in the catalogue and only leave the picker. |

## Removed parts

### Tokyo pack (`data/citypacks/tokyo.json`, 6.9 KB)

About 100 non-history grep hits name the pack's ids or file. Removing it needs about 1 session. What breaks or goes stale:

- **Tests and checks that break**
  - `profile` `TestIntegrationSave` uses the seeded area `tokyo-musashino` (`backend/internal/profile/validate_test.go:17`, `service_test.go:62,87`).
  - `scripts/db-checks.sh:25` (seed glob, harmless) and `:35` (`build-matrix -dry-run tokyo`).
  - The live walk eval (`walks/gemini/eval_test.go:48-66`) reads Tokyo and Austin and has no Da Nang cases. Re-point it at Da Nang–Hoi An before removing Tokyo.
  - Parked or removed code only: `matching/sim/sim_test.go:33,49` (sim parked) and `walks/bench/cases.json` (the bench is removed anyway).
- **Strings only, safe to keep or rename:** `platform/github/github_test.go` and admin `App.test.tsx`, `Cities.test.tsx`, `CityEditor.test.tsx`, `Venues.test.tsx`; mobile `draft.test.ts:26`; plain "tokyo" ids in the identity, maps, bound, catalog/suggest and tagging tests.
- **Independent:** many `plans` tests use `Asia/Tokyo` only as a time zone.
- **Code comments and usage text:**
  - `cities/pack.go:57-59`, `catalog/cities.go:29`, `repoedit/repoedit.go:3`, `cmd/tool/main.go:5,10`.
  - Comments in `db/migrations/00001_baseline.sql:26,27,54`. Leave them: goose migrations are history.
  - Examples in the proto files (`catalog`, `plans`, `profile`) and `docs/generated/api.md`.
- **Docs:**
  - `data/README.md:9,65-72,121,123`
  - `docs/architecture.md:16,135`
  - `docs/google-maps.md:43,50-54,60,68`
  - `docs/deploy.md:77,157,186-187`
  - `docs/onboarding.md`, `backend/README.md`, `apps/mobile/README.md`, the `Makefile`
  - `GLOSSARY.md:16` (the Area example)
  - `CHANGELOG.md` is history; leave it.
- **Issues:** #317 (rerun Tokyo venue discovery) becomes obsolete.
- **Hosted data:** if issho-dev already has Tokyo seeded, removing the file doesn't remove the rows. Removing a city is a separate apply (`removals.go` handles areas, not whole cities). Check before deleting.

### `translate/google` (TRANSLATOR=google)

106 src / 131 test lines, plus `cmd/tool/translatecompare.go` (85 lines, the `translate-compare` subcommand registered at `cmd/tool/main.go:15,64,73`). Removing it needs about 0.5 sessions. References:

- **Code:**
  - `translate/translators/translators.go:10,24-28`.
  - `platform/config/config.go:27-29,63,79,86` and `config_test.go:13,109-114`.
  - Warning text in `cmd/api/main.go:147,150` and `cmd/worker/main.go:61,64`.
  - `cmd/tool/emailtemplates.go:41,46`.
- **Config:** `Makefile:105`, `.env.example:13-16`, `.kamal/secrets.production.example:11`, `config/deploy.yml:87`.
- **Docs:** `backend/README.md:19,21,83,95,105-106,406`, `docs/google-maps.md:10,30,88`, `docs/secrets.md:30`, `docs/architecture.md:127`.

## Hygiene-bar Pilot-gate gaps in this area (#414)

1. **Handler-level RPC tests are missing** for `CatalogService.ListActivities`, `ListCities` and `ListLanguages` (and `SuggestActivities` if it isn't parked). Every other in-scope RPC here has one: `curation` (all six), `citypacks` (all five) and `uistrings.GetStrings`.
2. **Dead-code check.** Once the editors are unregistered, `cities/edit.go` (except `CheckPack`), `uistrings/editor.go` and `translate/review.go` become dead code in in-scope packages. So do `tagging`, through `SuggestActivities`, and the `repoedit` write side. Either keep them registered (admin-only) or move them with the parked packages. Removing `translate/google` and `translate-compare` also clears their entries.
3. **Admin pages the owner runs the Pilot with.** Venues is tested. Cities is tested but missing from #417's list (Q2). Parked `Translations.tsx` has no test.
4. **Doc contradictions:**
   - `docs/architecture.md:127` (Cloud Translation), `:97` (area as "neighbourhood or station"), `:143` (the editor flow as the way cities change; Austin "exists to prove this") and `:16` (first city Tokyo).
   - `docs/google-maps.md:10,30,88`.
   - `data/README.md:9,121`.
   - `backend/README.md:19,406` and the `citypacks` "(M8)" package doc.
5. **Code names against the glossary.** `ProposeCityPack`, `cities.Propose` and `Proposal` use "proposal", which GLOSSARY.md:61 lists under *Avoid* for a pending change. This is parked code, so it's a gap only if the editor comes back. GLOSSARY.md:16 cites Tokyo's wards as the Area example.
6. **Pilot readiness, not a hygiene gate:**
   - No Vietnamese offline bundle or human strings; the picker isn't limited to en/vi.
   - The walk eval has no Da Nang cases.
   - #352 Vietnamese font check.
   - #351 `GITHUB_TOKEN` renewal, if Q1 keeps the read side.

## Frozen and open tickets that target this area

| Ticket | Part | Status for the Pilot |
|---|---|---|
| #407 `cities.Apply` checks (bug, S) | `cities` | Do. It's the renovation's core. |
| #406 citydata (L) | loaders next to `cities`/`catalog` | Belongs to the platform audit; touches `geo.LoadCity`, not `cities` directly. |
| #320 preview an apply on the Cities page | `citypacks`, Cities page | Optional; nice for the owner's Pilot workflow. |
| #325 push keys exist | `uistrings` | Do (S). |
| #350 livelier English tone | `translate` guide | Optional. |
| #357 activities with no venue category | `catalog` | Decide with the catalogue content for Da Nang. |
| #347, #348 plans that combine activities | `catalog`/plans | Out of the Pilot (#417). |
| #315 ja/es keys | mobile i18n | Obsolete under en/vi. |
| #317 rerun Tokyo venue discovery | data | Obsolete once Tokyo is removed. |
| #352 fonts and RTL | mobile i18n | Narrow to the Vietnamese font check. |
| #351 renew `GITHUB_TOKEN` | repoedit/github | Needed if Q1 keeps the read side. |
| #290 discover-venues loses results | `tool`/maps, used by the Cities page | Relevant to venue discovery in the Pilot. |

## Open questions for the owner

1. **Do `repoedit` and `platform/github` stay in scope read-only** (recommended), so "Apply from main" and the human-strings follow keep working with `GITHUB_TOKEN`? The other option is to park them whole and switch to `tool seed` plus strings embedded at build time.
2. **Is the admin Cities page in scope?** It is the only UI for applying a pack and finding venues.
3. **Does "park" mean unregistering the CityEditor and translation-editor RPCs,** or keeping them registered and admin-only? This decides whether `cities/edit.go`, `uistrings/editor.go` and `translate/review.go` have to move.
4. **Is the Pilot's Vietnamese UI machine-translated** (Gemini, cached) with a human pass later, or does it need `human/vi.json` before the Pilot starts?
5. **Should the en/vi limit live in the server** (`ListLanguages`) **or only in the mobile picker?** Leaving the server alone keeps "other UI languages kept in the catalogue" literally true.
