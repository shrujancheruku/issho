# Audit: infra, CI, docs and tracker

Research for [#428](https://github.com/shrujancheruku/issho/issues/428), a child of the wayfinder map [#411](https://github.com/shrujancheruku/issho/issues/411). Facts and recommendations only; the owner settles each verdict in the verdict ADR ([#429](https://github.com/shrujancheruku/issho/issues/429)).

## Method

- **State audited:** `origin/main` at `139d77e` (2026-11-06). Line numbers refer to that commit.
- **Criteria:** Verdict criteria ([#413](https://github.com/shrujancheruku/issho/issues/413)): six signals per part, cost in agent sessions for renovating and for rebuilding, then the decision rules. The bar is The hygiene bar ([#414](https://github.com/shrujancheruku/issho/issues/414)). Scope is The Pilot's scope ([#417](https://github.com/shrujancheruku/issho/issues/417)) and What the Pilot is ([#412](https://github.com/shrujancheruku/issho/issues/412), as amended).
- **Shared layers, so signals are read this way** (as #428 asks):
  - *Test quality:* is the script or workflow tested or self-checking? For a doc: does anything check it against the code? For the tracker: does anything check its rules?
  - *Domain fit:* does the part match the settled decisions, the glossary and the Pilot (Da Nang–Hoi An, production stack, hygiene bar)?
- **Sources:** the files themselves; `gh api` for Dependabot alerts, rulesets and runs; `gh issue list` / `gh pr list` for the tracker; the Feature and knowledge inventory ([#415](https://github.com/shrujancheruku/issho/issues/415), `docs/research/feature-and-knowledge-inventory.md` on `research/feature-and-knowledge-inventory`); the Tracker inventory ([#416](https://github.com/shrujancheruku/issho/issues/416)).
- **Defect history:** `git log --since=2026-10-20` over `.github config scripts Makefile .githooks supabase renovate.json5`, `fix`/`revert` commits only, plus `bug` issues created since 2026-10-20.
- **Cost unit:** one agent session is one orchestrate worker ticket of size S–M. Owner-only steps (dashboards, secrets, servers) are listed separately and are not counted as agent sessions.
- **Scope filter:** every part in this area is in scope. Platform and delivery (web build, EAS, expo-updates, Sentry, the site) and the production stack are in the Pilot (#417, #412 "Environment"). Nothing here is cut. Two in-scope parts do touch cut items: the Makefile and `db-checks.sh` use the removed Tokyo pack and the parked simulator (see E).

## Summary

| Part | Tests | Structure | Domain fit | Docs | Defects | Refactor load | Renovate / rebuild cost | Recommended verdict |
|---|---|---|---|---|---|---|---|---|
| A. CI and hooks | green | green | amber | amber | green | green | 1.5 / 5 | **Keep** |
| A2. Dependency upkeep (Renovate config, alerts, backlog) | amber | green | amber | amber | red | amber | 2.5–3 / n.a. | **Renovate** |
| B. Deploy (Kamal, Workers, release-please) | amber | green | amber | amber | amber | amber | 2 / 5 | **Keep** (borderline) |
| C. Backups | red | green | amber | amber | green | amber | 1 / 1.5 | **Renovate** |
| D. Supabase config | green | green | amber | green | green | amber | 0.5 / 1.5 | **Keep** |
| E. Scripts and Makefile | amber | green | amber | green | green | green | 0.5 / 2 | **Keep** |
| F. Docs | amber | amber | amber | red | green | green | 1.5–2 / 3–4 | **Renovate** |
| G. Tracker | red | amber | amber | amber | green | amber | 1.5 / n.a. | **Renovate** |

No part has red domain fit, and no part has both tests and structure red, so **no rule forces a rebuild anywhere in this area**. In every case the renovate estimate is cheaper or equal.

A2 is split off from A along a file boundary: `renovate.json5`, the lockfiles and the alert and PR backlog. This lets CI's own verdict stand apart from a repo-wide backlog that CI doesn't own.

## Pilot gates in this area: measurements

### Dependencies and security alerts (#414 "Dependencies and security alerts")

Source: `gh api repos/shrujancheruku/issho/dependabot/alerts`, 2026-11-06.

- **Open Dependabot alerts: 28.** None critical; **11 high**, **11 medium**, **6 low**. All are npm alerts from `pnpm-lock.yaml`, and all are transitive.

  | Package | Alerts (H/M/L) | Patched in | Pulled in by (`pnpm why -r`) |
  |---|---|---|---|
  | node-forge 1.4.0 | 6 / 3 / 3 (#1, #16–#26) | 1.4.1 (#1: none) | `@expo/cli` 58.1.15 (expo) |
  | undici 6.29.1, 7.29.1 | 4 / 4 / 0 (#7–#14) | 7.30.1 | `@sentry/cli` (via `@sentry/react-native`), `miniflare` (via wrangler) |
  | zod 3.25.76 | 1 / 0 / 0 (#2) | 4.6.6 | `@expo/cli` |
  | fast-uri 3.1.8 | 0 / 2 / 2 (#3–#6, dev scope) | 3.1.9 | `ajv` via `@commitlint/config-validator` |
  | glob 7/10/13 | 0 / 1 / 0 (#27) | 14.0.2 | `test-exclude` (jest), `@expo/cli` |
  | minimatch 3.1.5 | 0 / 1 / 0 (#31, opened 2026-11-05) | 11.0.1 | eslint 9 |
  | deepmerge 4.3.1 | 0 / 0 / 1 (#15) | none | `@expo/config`, `jest-config` |

- **Fixed:** 2 (#29 xz, #30 rardecode, both Go, 2026-11-03). **Dismissed:** 0. So no medium or low alert has a written dismissal reason yet.
- **Alerts with no patched version:** #1 (node-forge) and #15 (deepmerge). These can only be dismissed with a written reason or fixed upstream by Expo.
- **The ticket for this:** #319 `build(deps): fix the open security alerts in the pnpm lockfile` (ready-for-agent, frozen by #411).
- **Gate state:** **not met.** There are 11 open high alerts, and 17 medium or low alerts are neither fixed nor dismissed.

### Open Renovate PRs

There are **25 open Renovate PRs**, #365–#392, all created 2026-11-05, plus the Dependency Dashboard #269.

- **Seven are majors:**
  - #386 node 26 and #387 TypeScript 7 (typescript fails on #387).
  - #388 eslint 10 (typescript fails).
  - #385 globals 17.
  - #389 postgres 18. This is the backup image, `config/backup/Dockerfile`; the database runs Postgres 17 (`ci.yml:215`, `docs/deploy.md:104`).
  - #390 connectrpc.com/connect v2 (codegen fails).
  - #391 anthropic-sdk-go v2. It is used only by `walks/claude`, which #417 removes.
- **Red checks:** #372 (buf 1.75.0, codegen fails), #387, #388 and #390. The other 21 are green and mergeable.
- **More are waiting on the dashboard:** 2 rate-limited and 6 pending status checks (#269 body).
- **Gate state:** **not met.** The backlog has to be merged or closed.

Two findings in the Renovate config, measured against the bar's ongoing rule ("Renovate patch and minor updates auto-merge when CI is green"):

- **Automerge covers only dev tooling.** `renovate.json5:43-51` automerges patch and minor updates of `devDependencies`, and `:52-66` the Go build tools. Runtime patch and minor updates (`fix(deps)`, e.g. #376–#384) wait for a person. To meet the bar, the config needs a runtime automerge rule. Expo stays excluded (`:85-105`).
- **The config and the docs don't match what Renovate does.**
  - `renovate.json5:14` schedules "before 7am on monday", but the 25 PRs were opened on Thursday 2026-11-05.
  - `renovate.json5:23` sets `prConcurrentLimit: 5`, but 25 are open.
  - `docs/onboarding.md:128` describes the Rate-Limited section as holding "most pending updates"; today it holds 2 (#416 noted this drift).

### Tracker rules (#414 "Tracker")

Counted on 2026-11-06, before this ticket closed.

| Rule | Measured | Detail |
|---|---|---|
| Every open issue has exactly one triage label | **24 of 67 open issues fail** (0 labels; none has 2 or more) | 22 `wayfinder:*` issues (#334, #339–#346, #362, #408, #411, #418, #421–#429), #360 *Post-MVP ideas*, #269 *Dependency Dashboard*. The rule as written has no exemption for maps, wayfinder tickets or the dashboard. |
| Closed issues carry no triage label | **64 of 94 closed issues fail** | 63 carry `ready-for-agent`, and #242 carries `wontfix` |
| No issue references the retired ROADMAP | **32 open issues mention `ROADMAP`; 68 closed ones do** | **18 open issues** require it in an unchecked acceptance box (#306, #315–#317, #319–#321, #323–#333). 12 say "Moved from `docs/ROADMAP.md`" (#347–#352, #355–#360). The maps #334 and #411 mention it in their notes. The repo itself is clean: `git grep -i roadmap` finds only CLAUDE.md:13 (the retirement rule), apart from the frozen log, ADR 0001 and CHANGELOG. |
| A PR closes its issue | **3 owner PRs merged since 2026-11-04 have no closing reference**: #420, #363, #361 | Renovate PRs and the release PR (#110) have none either. The PR template asks for `Closes #<n>` (`.github/pull_request_template.md:3`), but nothing checks it. |
| Checked by a scheduled script or Action | **No check exists** | `.github/workflows/` holds only `ci.yml`, `deploy-api.yml`, `deploy-web.yml` and `release-please.yml`, and none is scheduled (`git grep -E "schedule:|cron" .github` is empty) |

Other tracker drift that isn't a gate:
- PR-only labels are on issues: `go` on #401–#403, #405–#407, and `breaking-ok` on #402.
- Size labels are on 7 of 32 `ready-for-agent` issues.
- `commitlint.config.mjs:6` sets `scope-enum` at level 1, a warning. So scopes in use but not listed (`notifications`, `glossary`, `research`, `brand`) only warn. The issue-title scopes become PR titles, so the list and the issue titles can drift apart.

**Gate state: not met.** None of the four rules holds, and the checker doesn't exist.

### Backups: restore tested? (#414 "Operations")

- **Backups exist.** There is a nightly `pg_dump` of `app`, `goose`, `river` and `auth` to R2 (`config/backup/backup.sh:83-93`), described as live since 2026-10-26 with a 30-day lifecycle (`docs/deploy.md:117`).
- **A restore was tested once, on a format that no longer applies.**
  - The drill was 2026-10-26, on the first dump (`docs/deploy.md:125`).
  - That dump predates the baseline reset. `docs/deploy.md:225` says pre-reset dumps don't restore onto the baseline.
  - **No restore of a post-baseline dump is recorded.**
  - The restore rehearsal before the cutover sits in #341 *Production stack* (frozen).
- **The backup alert is not on.** #239 (ready-for-human, open) holds the owner steps: image push, `SENTRY_DSN` on the server, accessory reboot, alert rule, failure test.
- **The script has no test** (inventory §10), including its DSN parsing (`backup.sh:34-48`).
- **Gate state:** partly met. Backups exist; "a restore has been tested once" holds only literally, on a pre-baseline dump. The bar's intent (a restore of today's schema works) is unproven.

### Rollback path (#414 "Operations")

| Target | Documented path | Exercised? |
|---|---|---|
| API and worker (Kamal) | `kamal rollback -d production <version>` (`docs/deploy.md:81`). Migrations are not rolled back. `deploy.sh` can't deploy or roll back to commits that predate `config/kamal-image` (`docs/deploy.md:98`, `config/server/deploy.sh:26-30`). | No rehearsal is recorded in docs, the decision log or issues |
| Admin SPA, web build, site (Cloudflare Workers) | **None.** `docs/deploy.md`, `apps/admin/README.md:58-65` and `site/README.md:23` describe deploys only. A redeploy of an older commit through `workflow_dispatch` is possible but not written down. | No |
| Mobile JS (EAS Update) | **None.** `apps/mobile/README.md:236-238` describes publishing only | No |
| Store builds | Out of the server's control (tester phones); covered by `buf breaking` from the Pilot onward (#414) | n/a |

**Gate state:** partly met. There is a path for the API only, and it has never been rehearsed.

### Sentry alerts (#414 "Operations"; reported here because deploy config owns it)

- **API and worker alerts may not fire.** The API and worker tag events with `APP_ENV=preview` while they point at `issho-dev`. Any alert rule that filters on `production` no longer fires for them (`docs/deploy.md:75`, an owner to-do).
- **No backup monitor alert yet**, because #239 is still open.
- **Gate state:** unverified; owner steps are needed.

### Production secrets (#414 "Secrets")

- **The deployed stack runs against the dev project.**
  - The only Kamal destination, `production`, runs against `issho-dev` (`docs/deploy.md:10`, `config/deploy.production.yml:25`).
  - `supabase/config.toml` has only `[remotes.issho-dev]`.
  - No production Supabase project exists yet; that is #341 (frozen).
- **Gate state: not met.** "Production secrets live only in the deployment's secret store, and none have ever been used on dev" can only hold once the production project exists. Its secrets have to be created fresh, never copied from `issho-dev`.

### Docs contradictions from the inventory (#414 "Docs")

All 11 contradictions listed in the inventory (§10) are **still present at `139d77e`**:

1. `docs/architecture.md:92`, `:109`: names `00001_init.sql`/`00002_geo.sql` and "later migrations". The real files are `00001_baseline.sql` and `00003_domain_rule_views.sql`, with migration 2 registered in Go (`backend/db/river.go`).
2. `docs/architecture.md:97`: an area is "a neighbourhood or station". The glossary says it's a local government unit.
3. `docs/architecture.md:70-71`: `cmd/tool` is "migrations and seeding" (it has 14 subcommands), and `cmd/worker` carries "(M6)".
4. `docs/architecture.md:127`: says Google Cloud Translation translates; `TRANSLATOR` defaults to `gemini`.
5. `docs/architecture.md:88`: "other packages still have their own fakes".
6. `docs/google-maps.md:88`: says the server calls Cloud Translation, which #417 removes.
7. `backend/README.md:278`: "Not done yet: chat, push notifications, the travel-matrix refresh…"
8. `packages/api-client/README.md:3`: calls the admin SPA "(planned)".
9. `apps/mobile/README.md:213`, `:230`, `:234`: iOS and EAS "deferred to M9" / "M9 is planned".
10. `apps/mobile/README.md:186`: the push section lists plan and group pushes only; crew pushes are missing.
11. `apps/admin/README.md:20`: paging "not yet checked in a browser (#233)".

This audit found more in the same area:

12. **The two backup statements contradict.** `docs/deploy.md:10` says "the backup accessory is off until its R2 bucket exists", while `:117` says "Live since 2026-10-26".
13. **The Renovate docs don't match the config or the dashboard.** `docs/onboarding.md:128` and `renovate.json5:14`, `:23` disagree with the dashboard and PR dates (above).
14. **Retired milestone labels remain: 33 lines in 9 files.**
    - Files: `docs/architecture.md`, `docs/google-maps.md`, `docs/deploy.md` (`:31` "M11"), `docs/secrets.md` (`:32` "M6", `:42` "M2"), `backend/README.md`, `apps/mobile/README.md`, `apps/admin/README.md`, `data/README.md`, and `.github/workflows/ci.yml:315` ("shipped by Kamal in M9").
    - Milestones are retired (ADR 0001, #361).
15. **Tokyo framing in deploy config:**
    - `config/deploy.yml:6` (VM in Tokyo).
    - `config/deploy.yml:107` (`BACKUP_AT: "18" # 03:00 in Tokyo, the quietest time for the main city`).
    - `config/deploy.production.yml:25` ("a production project in Tokyo").
    - The Pilot's city is Da Nang–Hoi An, and the region is an open question in #341.

**Gate state: not met.** 0 of 11 fixed.

## Per-part findings

### A. CI and hooks

Files: `.github/workflows/ci.yml` (377), `scripts/ci-changes.sh` (129) and its test (98), `.githooks/{pre-commit,commit-msg}`, `commitlint.config.mjs`, `.gitleaks.toml`, ruleset `main` (required: secrets, commits, codegen, backend, typescript, image).

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | green | The only logic-bearing script, `ci-changes.sh`, is tested on every run (`ci.yml:56-57`). The rest is declarative and exercised on every PR. The last 10 `ci` runs on main all succeeded. Workflows have no `actionlint` or `shellcheck` (not required by the bar). |
| Structure | green | One definition per check: CI calls `make lint-go`, `make secrets-scan` and `scripts/db-checks.sh` (`ci.yml:83`, `:232`, `:257`). Every non-obvious choice is commented (caching `:129-146`, skip semantics `:29-39`). Required checks are skip-safe. |
| Domain fit | amber | The bar's new items are absent: there is **no dead-code check** (`knip` / `deadcode` appear nowhere), and the tracker check is in G. `breaking-ok` already works as the bar wants (`ci.yml:163-169`). `commitlint` scopes only warn (`commitlint.config.mjs:6`). |
| Docs match | amber | `docs/contributing.md:71` matches `ci-changes.sh`. `ci.yml:315` still says "M9". |
| Defect history | green | `75f7a20` and `8ff3dee` (`fix(ci)`, 2026-10-20 and 10-21); none since |
| Refactor load | green | No frozen ticket targets CI itself |

- **Cost to the bar:**
  - Renovate ≈ **1.5 sessions:** add `knip` and Go `deadcode` jobs (about 1; fixing their findings belongs to each part's audit), make the commitlint scope list an error and complete it (0.25), drop the M9 line (trivial).
  - Rebuild ≈ 5: a new `ci.yml` would have to relearn the path-filter, caching and skip-safety lessons recorded in the comments and in about 155 decision-log rows.
- **Rule applied:** no reds, and the cost is ≤ 2 sessions, so **Keep**.

### A2. Dependency upkeep (sub-part of A: `renovate.json5`, lockfiles, the alert and PR backlog)

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | Renovate PRs are gated by full CI. Four of the 25 are red (#372, #387, #388, #390), so CI does catch breaking bumps. |
| Structure | green | `renovate.json5` is well reasoned (rule order `:85-90`, the gitleaks grouping `:67-78`, custom managers `:107-125`) |
| Domain fit | amber | The automerge scope is narrower than the bar's ongoing rule (`renovate.json5:43-51`). #391 bumps a dependency of code #417 removes. #389 would take the backup image to PG 18 while the database runs 17. |
| Docs match | amber | `docs/onboarding.md:128`, and the Monday schedule against Thursday PRs |
| Defect history | red | 28 open security alerts, 11 high, all opened 2026-11-03 to 11-05. None is fixed within the bar's 7 days yet (the deadline is 2026-11-10). |
| Refactor load | amber | #319 (frozen) |

- **Cost to the bar:**
  - Renovate ≈ **2.5–3 sessions:**
    - Merge lock-file maintenance #392 and the green minors (0.5).
    - Clear the remaining alerts with `pnpm.overrides` for node-forge, undici, fast-uri, glob and minimatch, or the Expo SDK bump #368, which needs `expo install --fix` per `renovate.json5:102-104` (1).
    - Close or ticket the 7 majors, with a written dismissal for #1 and #15 (0.5).
    - Add the runtime patch and minor automerge rule and fix the docs (0.5).
  - Rebuild: n.a. (it's configuration).
- **Rule applied:** defect history is red, so not Keep. Defect history alone never forces a rebuild, so **Renovate**.

### B. Deploy (Kamal, the web Workers, release-please)

Files:
- `config/deploy.yml` (123), `config/deploy.production.yml` (42), `config/kamal-image`.
- `config/server/deploy.sh` (76), `config/server/kamal-role-env.rb` (61).
- `.github/workflows/deploy-api.yml` (159), `deploy-web.yml` (175), `release-please.yml` (46), `release-please-config.json`.
- `scripts/deploy-web.sh` (56).

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | `deploy.sh` and `kamal-role-env.rb` have no tests (inventory §10). The deploy checks itself: it asserts the API serves the deployed commit (`deploy-api.yml:153-159`). CI checks that the image's binaries start (`ci.yml:374-377`). The last 8 `deploy-api` runs succeeded. Rollback has never been exercised. |
| Structure | green | One Kamal pin (`config/kamal-image`). A forced-command deploy key, with production secrets only on the server (`deploy-api.yml:10-12`, `deploy.sh:1-9`). Deploys only commits on main (`deploy.sh:18-21`). Commit-accurate change detection (`deploy-api.yml:95-116`). |
| Domain fit | amber | The `production` destination points at `issho-dev` (`docs/deploy.md:10`, `config/deploy.production.yml:25`), while the Pilot runs on the production stack (#412 "Environment"). Tokyo hosting and backup timing (`config/deploy.yml:6`, `:107`); the region is open in #341. Only the API has a rollback path. |
| Docs match | amber | `docs/deploy.md:10` against `:117`. "M11" (`:31`). The Sentry environment note (`:75`). Workers and EAS Update have no rollback docs. |
| Defect history | amber | `09c74e3` (#120, run `kamal migrate` on the web role only, 2026-10-27) and `96fd3b2` (#285, write missing role env files before migrating, 2026-11-04) |
| Refactor load | amber | Frozen grillings #341 *Production stack*, #340 *Web for launch*, #343 *Distribution*: decisions that will reshape the config rather than refactors |

- **Cost to the bar:**
  - Renovate ≈ **2 sessions:**
    - Document and rehearse the rollback for the API, the Workers (`wrangler rollback` or a dispatch of an older commit) and EAS Update (republish the previous update) (1).
    - Fix the deploy.md contradictions and the Tokyo comments (0.25).
    - Point the destination at the production project once #341 decides (0.5).
  - Owner-only: create the production Supabase project and its secrets, rehearse a `kamal rollback`, fix the Sentry alert environment.
  - Rebuild ≈ 5, which throws away hard-won fixes (#120, #285, the change-detection logic).
- **Rule applied:** no reds, and the cost is about 2 sessions, so **Keep**.
- **Borderline:** if the production cutover from #341 is counted against this part, the cost passes 2, and the rule gives **Renovate**.

### C. Backups

Files: `config/backup/backup.sh` (122), `config/backup/Dockerfile` (9), the `backup` accessory in `config/deploy.yml`.

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | red | No automated test of the script (DSN parsing `:34-48`, check-ins `:52-72`). The one restore drill (`docs/deploy.md:125`) was on a pre-baseline dump that `docs/deploy.md:225` says no longer restores onto the baseline. Today's dumps have never been restored. |
| Structure | green | One small POSIX script, fail-soft check-ins, explicit table exclusions (`:77-88`) |
| Domain fit | amber | The right schemas, and sessions are excluded (`:77-87`). `BACKUP_AT: "18"` is chosen for Tokyo (`config/deploy.yml:107`). #389 would bump the image's Postgres major past the database's. |
| Docs match | amber | `docs/deploy.md:10` against `:117`. `:117` says "live", but #239 (alert setup) is open. |
| Defect history | green | No `fix` commits (PRs #92, #102, #222 are feature work) |
| Refactor load | amber | #239 (ready-for-human, not frozen). The restore rehearsal sits in #341 (frozen). |

- **Cost to the bar:**
  - Renovate ≈ **1 session:**
    - A `make restore-check` (or a scheduled job) that restores the latest dump into the Supabase Postgres image from `ci.yml:215` and compares table counts (0.75).
    - A unit test for the DSN parsing, and the doc fixes (0.25).
  - Owner-only: the #239 steps, and one restore of a current dump.
  - Rebuild ≈ 1.5: replace the accessory with Supabase Pro backups or PITR plus an R2 copy. That is a #341 decision, and it still needs a restore test.
- **Rule applied:** test quality is red, so not Keep. A rebuild needs domain fit red, or tests *and* structure red; structure is green, so the rule gives **Renovate**. The cost check agrees: 1 < 1.5.

### D. Supabase config

Files: `supabase/config.toml` (472), `supabase/templates/` (3), `supabase/README.md`, `docs/supabase-hosted.md`.

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | green | Repo rules are checked where they can be. The Data API is off (`config.toml:11`), and CLI migrations and seed are off (`:65`, `:72`). CI asserts goose bookkeeping is out of `public` (`ci.yml:258-261`). CI runs the same Postgres image as the CLI (`ci.yml:206-215`). |
| Structure | green | One file, per-remote overrides (`config.toml` `[remotes.issho-dev*]`) |
| Domain fit | amber | Only the `issho-dev` remote exists; the Pilot needs a production remote (#341) |
| Docs match | green | The inventory found no contradiction in supabase/README or supabase-hosted.md |
| Defect history | green | `6427414` (`fix(infra)`, disable the Data API locally, 2026-10-20); none since |
| Refactor load | amber | #341 (frozen) |

- **Cost to the bar:** renovate ≈ **0.5 session** (add `[remotes.<prod>]` after #341, run `config diff`). Rebuild ≈ 1.5.
- **Rule applied:** no reds, and the cost is ≤ 2, so **Keep**.

### E. Scripts and the Makefile

Files: `Makefile` (135); `scripts/db-checks.sh` (37), `test-db.sh` (43), `deploy-web.sh` (56), `go-coverage-summary.sh` (20), `ts-coverage-summary.mjs` (21); `scripts/brand/`, `scripts/site/`.

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | Only `ci-changes.sh` has a test. `db-checks.sh` runs in every backend CI job (`ci.yml:257`), so it's exercised. `deploy-web.sh` is exercised by `deploy-web.yml`. The coverage summaries are report-only (`ci.yml:245`, `:293`). |
| Structure | green | Scripts are shared between CI and make (`db-checks.sh:1-3`), with defensive guards (`db-checks.sh:10-18`) |
| Domain fit | amber | Coupled to cut items. `Makefile:89` (`CITY ?= tokyo`) and `scripts/db-checks.sh:35` (`build-matrix -dry-run tokyo`) depend on the Tokyo pack, which #417 removes. `Makefile:101-102` runs the simulator, which #417 parks. |
| Docs match | green | Targets are documented in the backend README (inventory §4) |
| Defect history | green | None |
| Refactor load | green | None frozen |

- **Cost to the bar:** renovate ≈ **0.5 session** (retarget to `da-nang-hoi-an` in the same PR that removes Tokyo, and keep `sim` compiling for the parked simulator). Rebuild ≈ 2.
- **Rule applied:** no reds, and the cost is ≤ 2, so **Keep**.

### F. Docs

Files: `docs/architecture.md` (178), the directory READMEs, `docs/onboarding.md` (147), `docs/contributing.md` (144), `docs/deploy.md` (231), `docs/adr/` (1 ADR), `GLOSSARY.md` (73), plus `docs/secrets.md`, `docs/google-maps.md`, `docs/supabase-hosted.md`, `docs/design-system.md`.

| Signal | Colour | Evidence |
|---|---|---|
| Test quality (is anything checking docs?) | amber | Only `docs/generated/api.md` is checked, through the codegen drift check (`make gen` → `docs`, `ci.yml:155-156`). Prose, links and glossary terms are checked only by the PR template checklist (`.github/pull_request_template.md:9`). There is no link check. |
| Structure | amber | Knowledge is spread across a 312-line frozen decision log, one process-only ADR (`docs/adr/0001-issues-replace-the-roadmap.md`), a 418-line backend README and issue comments (inventory §1). After 2026-11-04, decisions live only in issue comments. |
| Domain fit | amber | `GLOSSARY.md` is current: it defines Pilot (`:63`) and Soft launch (`:68`). The docs use retired milestone labels (33 lines, 9 files), the old area definition (`architecture.md:97`) and Tokyo framing in deploy config. These are stale words, not a wrong model. |
| Docs match | red | 11 inventory contradictions, all unfixed, plus 4 more found here (see the list above) |
| Defect history | green | n.a. (no bug issues against docs) |
| Refactor load | green | No frozen ticket targets the docs. Several frozen tickets require README updates as part of their own work. |

- **Cost to the bar:**
  - Renovate ≈ **1.5–2 sessions:**
    - Fix the 15 contradictions (1).
    - Sweep the milestone labels and the Tokyo framing (0.5).
    - Add a markdown link check to CI (0.25; optional, the bar doesn't require it).
  - Rebuild ≈ 3–4: rewrite architecture.md and the READMEs from code. Only architecture.md would be cheaper to rewrite than to patch, and that fits inside a renovate.
- **Rule applied:** docs match is red, so not Keep. Docs match alone never forces a rebuild, so **Renovate**.
- **The glossary-term gate** ("code names match glossary terms in every part that isn't parked") is measured by the per-slice audits, not here.

### G. Tracker

| Signal | Colour | Evidence |
|---|---|---|
| Test quality (is anything checking the rules?) | red | No scheduled script or Action exists (4 workflows, none scheduled). Every rule in #414 "Tracker" is unchecked. |
| Structure | amber | 24 open issues have no triage label, and 64 closed issues keep one. PR-only labels (`go`, `breaking-ok`) are on issues. Size labels are on 7 of 32 agent tickets. |
| Domain fit | amber | 18 acceptance criteria require the retired ROADMAP. The rule's reach is undefined for `wayfinder:*` issues, the Dependency Dashboard and #360. |
| Docs match | amber | `docs/agents/triage-labels.md` defines the five labels but no exemptions. `docs/agents/issue-tracker.md:39-48` defines `wayfinder:*` labels without saying how they relate to triage. |
| Defect history | green | n.a. |
| Refactor load | amber | #418 *How work gets planned from here* (open) may change the label set |

- **Cost to the bar:**
  - Renovate ≈ **1.5 sessions:**
    - A scheduled Action (or `scripts/tracker-check.sh` run weekly) that checks the four rules, with exemptions the owner names (1).
    - A one-off cleanup: remove the triage labels from 64 closed issues, edit the 18 acceptance criteria and decide the 12 "Moved from" lines (0.5). The edits to frozen tickets wait for the plan's keep or close decision.
  - Rebuild: n.a.
- **Rule applied:** test quality is red, so not Keep. Structure is amber, so no rebuild; the rule gives **Renovate**.

## Frozen tickets that target this area

- **#319** `build(deps)`: open alerts (frozen) → A2
- **#341** *Production stack* (frozen grilling): Supabase region and Pro, server location, restore rehearsal → B, C, D
- **#340** *Web for launch* (frozen grilling) → B (the Workers)
- **#343** *Distribution for each platform* (frozen grilling) → B (EAS, store tracks)

Not frozen, but in this area: #239 (backup alert, ready-for-human), #351 (renew `GITHUB_TOKEN` before 2027-01-22, used by `platform/github`, which #417 parks), #418 (tracker process).

## Questions for the owner

1. **Tracker rule scope:**
   - Are `wayfinder:*` issues, the Dependency Dashboard (#269) and #360 exempt from "exactly one triage label"?
   - Does "references the ROADMAP" cover "Moved from `docs/ROADMAP.md`" provenance lines and closed issues, or only open acceptance criteria?
   - Does `wontfix` on a closed issue (#242) count?
2. **The restore gate:** does "a restore has been tested once" count the 2026-10-26 pre-baseline drill, or does it need a restore of a current dump? This audit recommends a current dump.
3. **B's verdict:** should the production cutover (#341) count against B's cost? If it does, B becomes Renovate.
4. **Majors in the backlog:** close #391 (removed code) and #389 (PG 18 against a PG 17 database) outright, or ticket them?
