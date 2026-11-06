# Audit: matching and walks

Research for #422, a child of the wayfinder map #411. It applies the verdict criteria in #413 and the hygiene bar in #414 to the matching and walks parts that #417 put in scope, taking into account the walk decision in #412 (and its amendment). Everything here was checked at `origin/main` 139d77e (2026-11-06). The doc has facts and recommendations only. No product code was changed.

How the evidence was gathered:
- `go test -cover` on each package. Integration tests skip without `TEST_DATABASE_URL`; CI runs them.
- `deadcode` from golang.org/x/tools v0.52.0, run with `-test ./...`.
- Package imports from `go list`.
- `git log --grep '^fix'` since 2026-10-20, plus open and closed bug issues.

## Parts

The parts are split only along package boundaries, as #413 requires.

| Part | Packages (non-test lines) | Status in #417 |
|---|---|---|
| Matching | `internal/matching` (~2,100: matching, suggest, assemble, config, crews, feed, schedule, pool, relations) | In scope |
| Walks | `internal/walks` (~720) and `internal/walks/gemini` (76, plus the eval) | In scope, but walks run only in areas that pass the walk eval (#412) |
| Maps | `internal/maps` (~400), `internal/maps/google` (~610 + placetypes.txt), `internal/maps/mapstest` (275) | In scope (venue discovery, opening hours, routes) |
| Geo | `internal/geo` (~590: matrix, store, venues), plus the `build-matrix` and `discover-venues` subcommands in `cmd/tool/main.go` | In scope (travel matrix, venue discovery) |
| Simulator | `internal/matching/sim` (539), `tool simulate` | Parked |
| Walk bench | `internal/walks/bench`, `internal/walks/claude`, `cmd/tool/walkbench.go` | Removed |

How the parts depend on each other:
- `matching` imports no other internal package. It is used by `plans`, `crews`, `cities`, `moderation`, `matching/sim` and `cmd/tool`.
- `walks` imports only `maps`.
- `geo` imports only `maps`.
- `maps` is a leaf package. It is imported by 13 packages.

## Signals

Each signal is rated G (green), A (amber) or R (red).

| Part | Test quality | Structure / coupling | Domain fit | Docs match | Defect history | Refactor load |
|---|---|---|---|---|---|---|
| Matching | G | G | A | A | G | A |
| Walks | A | A | A (close to R) | A | R | G |
| Maps | G | G | G | A | G | G |
| Geo | A | G | G | A | A | A |

### Matching (`internal/matching`)

**Test quality: G.**
- Coverage is 92.7%, from 26 table-driven tests (~1,090 lines).
- The package is pure and deterministic, so the tests need no fakes.
- The parked simulator also checks invariants over the real city packs (`matching/sim/sim_test.go`).
- There is no handler here. The RPCs that call matching (`plans`, `crews`) belong to other audits.

**Structure / coupling: G.**
- It has no internal imports and no I/O.
- Its inputs come from `plans`, with `ListMatchingUsers` and related queries in `backend/db/queries/matching.sql`.
- Its config is a typed `Config` with `ParseConfig`, overlaid per city (`config.go`, `cities/validate.go:132`).

**Domain fit: A.**
- Two-activity pairing is on by default.
  - The pairing code is in `suggest.go`: `rankActivities` at :130, `pairs` at :247, `PlanInterest` at :267, and the `FollowUp` set at :190.
  - The defaults are in `config.go`: `PairBonus` 0.1, `PairCandidates` 10 and `PairShortOnly` true.
  - The da-nang-hoi-an pack has no `matching` block, so the Pilot city gets two-activity plans.
  - #417 lists "plans that combine activities" as out (#408 is open and #347/#348 are frozen).
  - Either the Pilot sets `pair_candidates: 0`, or the scope is read as excluding only new combination features. The owner decides.
  - `pair_candidates` isn't validated to be ≥ 1, so 0 already turns pairing off. That makes this a config switch, not code.
- The code drifts from the glossary.
  - `Relations.Blocked` (`matching.go:293–294`), the violation text "blocked each other" (`assemble.go:390–395`, with the comment at :381) and "(Blocked)" (`crews.go:38`) should all be **kept apart**.
  - The renames depend on #403 and #404.
- Milestone references such as "(M7)" (`matching.go:291`) point at the retired ROADMAP.

**Docs match: A.** The problems are in `backend/README.md`:
- Step 2 says "A non-conversational activity gets a food follow-up", but the code pairs by score and appeal (#89).
- The opening-hours bullet says "activities have no stored duration". That has been stale since #282 added `Activity.Hours`.
- It uses "blocked pairs" wording, and :52 and :282 cite M4, M6 and M7.
- :278 lists "the travel-matrix refresh" as not done.

**Defect history: G.**
- There has been one fix since 2026-10-20: 58c2b37 "fix(backend): venue opening window from the activity's hours (#282)", for #168.
- No bug issues are open against matching.

**Refactor load: A.**
- #406 (citydata, L) changes how the city's inputs reach matching and sim.
- #401 (participants view, M) rewrites the queries that feed matching.
- #403 and #404 rename kept apart.
- #349 (users_per_plan) is waiting on the owner.

### Walks (`internal/walks`, `internal/walks/gemini`)

**Test quality: A.**
- Coverage is 81.9% for `walks`, with good composer fakes (`walks_test.go`, `names_test.go`).
- The walk eval (`walks/gemini/eval_test.go`) is the 70% gate in #412.
  - Its cases are Tokyo and Austin only (:47–63), and it reads `tokyo.json` and `austin.json`.
  - The bar is at :127.
  - The Da Nang–Hoi An cases used in #338 were never committed.
  - The eval is skipped without `WALKS_EVAL`, so `walks/gemini` shows 0% coverage.
- The fixtures in `names_test.go` are Tokyo (Koenji, Kichijoji, Naka-Meguro).

**Structure / coupling: A.**
- The core propose → check → reject loop is clean and imports only `maps`.
- The core package also carries hooks for the bench and for Claude:
  - `Meta.CostUSD`, `OnAttempt` (`walks.go:189–191`) and the `Proposer.System` override;
  - `PromptVersion` "walk-v1", which only `walkbench.go:108` and the bench use;
  - the strict mode of `ProposalSchema(strict)`, which exists for Claude.

**Domain fit: A, close to R.** The model fits (an LLM proposes and the code checks against Places and Routes), but the parameters and the lexicon are tuned for Japan.
- `Kinds` (`walks.go:42–48`) include station, tea_house and shrine.
- `meetKinds` are {station, cafe, tea_house} (:298). `placeTypes` (:272) include `japanese_izakaya_restaurant`.
- The `SystemPrompt` (`prompt.go`) gives Tokyo examples ("Kayaba Coffee, Yanaka") and asks for "a station or a cafe right by a station". Da Nang has no stations.
- The `generic` word list in `names.go` is Japanese and English only.
  - NFKD doesn't decompose đ, so "Đình" never matches "Dinh" (#338).
- Result: 15 of 75 walks (20%) compose in Da Nang–Hoi An, against the 70% bar (#338).
  - Only 5 wards produce walks: Hoi An 5/5, Hai Chau 4, Hoi An West 3, Hoa Cuong 2 and Hoa Khanh 1. The other 10 produce none.
  - #338 found four causes: chain-café meeting places, the đ diacritic, a 3 km radius that is too small for the merged wards, and legs over 20 min.
- I rated it amber, not red, because rule changes and a lexicon fix can solve all four causes without changing the design. If the owner reads 20% as "doesn't fit the domain", the rules make this a rebuild. See the verdict.

**Docs match: A.**
- There are small inconsistencies inside the code.
  - The title message says "at most 60 characters", but the check is > 70 (`walks.go:317`).
  - `MaxLeg` is 20 min (`DefaultRules`), but the feedback text says "under 15 minutes".
  - Neither gap has a comment explaining why.
- The package doc describes Tokyo walks.

**Defect history: R.**
- No fix commits since 2026-10-20 touch walks.
- But the one field test (#338) failed the Pilot city by 50 points, and none of its causes is fixed yet.

**Refactor load: G.**
- No frozen ticket targets walks. #405 (stage runner) leaves walks hand-written.
- The walk retry and budget live in `plans/walks.go`: `WalkRetryAfter` is 7 days, and `maxWalksPerPass` is 4 (#275 and #183).

### Maps (`internal/maps`, `internal/maps/google`, `internal/maps/mapstest`)

**Test quality: G.**
- Coverage is 99.1% for `maps`, 85.6% for `google` and 84.0% for `mapstest`.
- `google_test.go` (427 lines) runs against a fake HTTP server.
- `mapstest.Fake` is the shared fake (#293).
- The opening-hours tests use an `Asia/Tokyo` location (`maps_test.go:9`). That is a time zone, not the Tokyo pack, so the test doesn't break when the pack goes.

**Structure / coupling: G.** `maps` is a leaf interface package, `google` is its only real implementation, and the provider details stay inside `google`.

**Domain fit: G.**
- Covers places, nearby search, routes, the opening-hours check, boundaries and polylines.
- TWO_WHEELER routing was researched in #394.

**Docs match: A.** The problems are in `docs/google-maps.md`:
- :3 says "two things".
- :12 says "Only the two tool commands need a real key", but the worker needs one too.
- It says tests use "httptest fakes"; they use `mapstest`.
- :43, :50–54 and :60 are Tokyo examples, and it cites "M6".

**Defect history: G.** No fixes and no open bugs in this part.

**Refactor load: G.** #359 (Maps caching terms) is waiting on the owner and is policy, not code.

### Geo (`internal/geo` and the `build-matrix` / `discover-venues` tool commands)

**Test quality: A.**
- Unit coverage is 53.9%. The `store.go` tests are integration-only (`TestIntegrationStore`, `SaveBatches`).
- `cmd/tool` has 15.0% unit coverage. `TestIntegrationBuildMatrix` (`cmd/tool/geo_test.go:69`) and `TestIntegrationDiscoverVenues` (:132) exist but run only in CI.
- The venue search's split-and-budget logic (`venues.go:169–220`) is unit-tested.

**Structure / coupling: G.**
- `geo` imports only `maps`.
- It is used by `adminjobs`, `cities`, `citypacks`, `curation`, `worker` and `cmd/tool`.

**Domain fit: G.** Travel matrices between areas, and venues placed in their area by boundary (`Locate`). This fits the glossary's Area.

**Docs match: A.**
- `backend/README.md:278` calls the matrix refresh not done.
- The README examples (:244–249) use `tokyo`.

**Defect history: A.**
- #290 (open, ready-for-human): discover-venues loses results on HTTP 429. It targets `discoverVenues` in `cmd/tool/main.go` and `geo.DiscoverVenues`.
- #357 (data: activities with no venue type) is data, not code.

**Refactor load: A.** #406 renames `geo.LoadCity` (`store.go:22`) into the citydata seam.

## Cost and verdicts

The #413 rules:
- **Keep** needs no reds and at most about 2 sessions to reach the bar.
- **Rebuild** follows when domain fit is red, or when tests and structure are both red.
- **Renovate** covers everything else.
- When the two estimates differ, the cheaper one wins. On a tie, renovate if tests are amber or green.
- Docs, defects and refactor load only move a part between keep and renovate.

| Part | Renovate (sessions) | Rebuild (sessions) | Verdict | How the rule applies |
|---|---|---|---|---|
| Matching | 2–3 | 6–8 | **Renovate** | No reds, but the cost is over 2: kept-apart renames, README fixes, the Pilot pairing switch, and absorbing #401 and #406. Without #406 it comes to about 2, which makes it Keep. |
| Walks | 3–4 | 5–6 | **Renovate** | Domain fit is amber (a red would mean rebuild), and the defect red stops Keep. Renovate is cheaper. The work: a pack-neutral lexicon and meeting kinds, a non-station prompt, đ folding, a per-area radius and leg rules, a committed Da Nang–Hoi An eval, removing the bench hooks, and a per-area walk switch. |
| Maps | ~1 | — | **Keep** | No reds and about 1 session (the `google-maps.md` fixes). |
| Geo | ~2 | — | **Renovate** (Keep if #290 stays with the owner) | No reds. Fixing #290, the README and the #406 rename brings it to about 2. That is on the line, so Renovate if #290 is in agent scope; otherwise Keep. |

Walks is the only verdict that could change. If the owner rates domain fit red because of the 20% Pilot result, the rules give **Rebuild**, at about 5–6 sessions, because a rebuild is still a propose-and-check composer. The renovation estimate is lower because each of the four causes found in #338 maps to a specific change.

## Parked: simulator

The simulator (`internal/matching/sim`, `tool simulate`):
- It compiles and its tests pass, at 97.3% coverage.
- `TestSimulate` runs tokyo, austin and da-nang-hoi-an, and `TestDeterministic` loads `tokyo.json`.
- Removing the Tokyo pack breaks these tests. Swap the fixture to da-nang-hoi-an (or austin) in the same change.
- `cmd/tool/main.go:5,10` and the README examples also name tokyo.

## Removed: walk bench

The removal covers `internal/walks/bench`, `internal/walks/claude` and `cmd/tool/walkbench.go`.

Size:
- About 2,151 source lines: bench 1,766, claude 86, walkbench.go 299.
- About 424 test lines: bench 342, claude 82.
- `bench/cases.json`, 443 lines.

What references them:
- `go.mod:22` `anthropic-sdk-go` v1.84.0 (and `go.sum:195–196`). Nothing else uses it, so it can go.
- `.golangci.yml:60`, a path exclusion that also lists `matching/sim`. Keep that half.
- `cmd/tool/main.go:11, 64, 69–70`: the subcommand and the docs for it.
- `platform/config/config.go:240–241, 256–257`: `AnthropicAPIKey` and `GoogleMapsEmbedAPIKey`. Both are used only by `walkbench.go:143, 218, 251`, so both can go. Also `config_test.go:16` and `.env.example:22` (`ANTHROPIC_API_KEY`).
- Docs: `docs/secrets.md:32`, `backend/README.md:37–38, 268, 345–347` and `docs/google-maps.md:65–66`.
- What the bench leaves behind in `walks`: `PromptVersion`, `Meta.CostUSD`, `OnAttempt`, the `Proposer.System` override, the strict mode of `ProposalSchema`, the comment at `prompt.go:10`, and the cross-references to `claude.go` and `claudecode.go`.

The walk eval (`walks/gemini/eval_test.go`) stays. Estimated cost is about 0.5 session.

## Pilot-gate gaps (#414) in this area

1. **The walk eval doesn't cover the Pilot city, which is at 20%.** #412 allows walks only where they pass 70%, but the eval has no Da Nang–Hoi An cases, and the measured rate is 20% (#338).
2. **There is no per-area walk switch.** City packs have only city-wide `activity_overrides`. "Walks only in areas that pass the eval" can't be expressed, and `plans/walks.go` composes walks for every area.
3. **The code doesn't match the glossary.**
   - `Relations.Blocked` and the "blocked each other" text in matching should say kept apart.
   - `docs/architecture.md:97` calls an area "a neighbourhood or station", and :5 gives "Kichijoji station" as an example.
4. **Docs contradict the code.**
   - `docs/architecture.md:16` says "The first city is Tokyo".
   - `docs/architecture.md:88` says "other packages still have their own fakes" and calls `translate.Translator` Google.
   - `docs/google-maps.md:3, :12`, and its testing wording.
   - `backend/README.md`: :278, the opening-hours note, the food follow-up step, and the M-milestone references.
5. **Deadcode passes, but only with the right setup.**
   - v0.52.0 with `-test ./...` finds nothing.
   - Without `-test` it flags only items that tests use: `google.IsPlaceType` (`placetypes.go:26`, used by `cities/data_test.go:31`) and the `mapstest.Fake` methods.
   - v0.38.0 crashes on go1.27.3.
   - The gate should pin a recent version and run with `-test`.
6. **Tokyo fixtures break when the pack is removed.** These are the sim tests, the walk eval and the walks name tests. The last use invented `Request{City: "Tokyo"}` values, so they don't read the pack, but they are the wrong domain for the Pilot.
7. **Handler tests.** This area is libraries only, so the handler-level test gate falls on `plans`, `crews` and `cities`.
8. **Pairing vs scope.** Two-activity plans are on by default in the Pilot city, while #417 puts combined plans out.

## Open questions for the verdict ADR

- Should the Pilot run with pairing off (`pair_candidates: 0` in the da-nang-hoi-an pack)? Or does #417's "out" cover only new combination features (#347, #348, #408)?
- Is walks' domain fit amber (renovate) or red (rebuild), given 20% in the Pilot city?
- Is #290 agent work or owner work? The answer decides Keep or Renovate for geo.
- Does #406 count toward matching's renovation cost? Without it, matching is Keep.
