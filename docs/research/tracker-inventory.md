# Tracker inventory (2026-11-05)

Research for [#416](https://github.com/shrujancheruku/issho/issues/416), a child of the map [#411](https://github.com/shrujancheruku/issho/issues/411). Facts only: no keep or close decisions.

**Sources.** Snapshotted from GitHub on 2026-11-05:
- `gh issue list --state open --limit 200` and the closed list;
- for each open issue, the GraphQL `subIssues`, `parent`, `blockedBy` and `blocking` fields;
- the comments on each open issue that has any;
- each closed issue's closing event;
- `gh label list` and `gh pr list --state open`;
- the Dependabot alerts API, counts only.

Code and docs are read at `origin/main` 4d13143, and each finding that rests on a file names that file.

**The freeze.** On 2026-11-05, #411 deliberately blocked every open `ready-for-agent` issue and six soft-launch tickets (see #411's Notes, under "Freeze"). This inventory records those blocks but doesn't count them as defects.

## 1. Summary

**64 open issues:**
- 2 wayfinder maps (#334, #411) and their 17 open children (10 under #334, 7 under #411);
- 32 issues labelled `ready-for-agent`;
- 11 issues labelled `ready-for-human`;
- the deferred list #360;
- Renovate's Dependency Dashboard, #269.

| Class | Count | Issues |
|---|---|---|
| live | 26 | 19 wayfinder (#334, #339–#346, #362, #408, #411–#418); 6 bugs and fixes (#290, #319, #321, #323, #357, #407); 1 bot (#269) |
| feature | 14 | #320, #329, #333, #347, #348, #350, #355, #356, #358, #396, #397, #398, #399, #404 |
| refactor | 12 | #325, #326, #327, #328, #330, #331, #332, #401, #402, #403, #405, #406 |
| owner chore | 9 | #239, #306, #316, #317, #349, #351, #352, #359, #400 |
| stale | 1 | #315 (its counts and target languages no longer match the code or the launch) |
| duplicate | 1 | #324 (in part; its own body says #329 and #330 cover it too) |
| fog | 1 | #360 |
| already done | 0 | None is fully done. Parts of #340 and #352 are reported done in their comments or bodies. |

**Facts that weigh most on a decision:**

- **Dependencies.** Every blocking edge is native except two that exist only as text: #306 on #204 (closed) and #317 on #290. #411 blocks 38 issues. No open issue is assigned.
- **Drift: ROADMAP references.** 18 open issues still require a ROADMAP update, an M11 line ticked, or a ROADMAP Decision-log row in their acceptance criteria: #306, #315–#317, #319–#321 and #323–#333.
  - `docs/ROADMAP.md` was retired on 2026-11-04 (ADR 0001, `CLAUDE.md`).
  - No GitHub milestones exist.
- **Drift: terms and an ADR that aren't on `main`.**
  - #411 and #412 cite **Pilot** in `GLOSSARY.md`.
  - The agent briefs on #347 and #356 cite ADR 0002 and glossary terms "from #395".
  - These texts exist only in open, unmerged PRs (#419 and #395). `docs/adr/` on `main` holds only 0001.
- **Closed issues.** 88 are closed (#140–#394): 61 by a merged PR and 27 by hand. Every hand-closed issue has at least one comment giving its resolution.

## 2. Labels

`gh label list` returns these labels:
- the five triage labels from `docs/agents/triage-labels.md`;
- the five `wayfinder:` labels;
- `size:S`, `size:M` and `size:L`;
- `breaking-ok`;
- GitHub's defaults (`bug`, `enhancement`, `documentation`, `duplicate`, `wontfix`, …);
- release-please's `autorelease:` labels;
- labels for dependency PRs (`dependencies`, `go`, `javascript`).

Measured against the conventions:

- **Triage labels.** Every open issue outside the maps, the wayfinder children, #269 and #360 carries exactly one of `ready-for-agent` (32) or `ready-for-human` (11).
  - `needs-triage`, `needs-info` and `wontfix` are on no open issue.
  - #269 and #360 have no labels at all. #360's body says it is untriaged on purpose.
- **`size:S|M|L`.** In `.claude/orchestration.md` (Tiers), a size label on a ticket overrides the computed review tier.
  - It is on 7 of the 32 `ready-for-agent` issues, all from the 2026-11-05 architecture review: #401 M, #402 L, #403 M, #404 L, #405 S, #406 L, #407 S.
  - The other 25 have no size label.
- **`go`.** Its description reads "Pull requests that update go code".
  - It is on issues #401, #402, #403, #405, #406 and #407.
  - #404, from the same review, doesn't have it.
- **`breaking-ok`** is defined for PRs (the label's description and orchestration.md's merge gates), but it is on issue #402.
- **Type labels.**
  - Refactor and test tickets carry `enhancement`; no refactor label exists.
  - `build(deps)` #319 carries `bug`.
  - #290 is code work but is labelled `ready-for-human`. Its comment of 2026-11-04 says this parks it at the owner's request, so that orchestrated runs skip it.
- **Closed issues keep `ready-for-agent`.** 63 of the 88 closed issues still have it.
- **Scopes in titles.** `.claude/orchestration.md` derives a branch's lane from the scope in its ticket's title.
  - Scopes in use: `matching` (#347, #348, #356), `data` (#357), `notifications` (#325, #328), `deps` (#319), and `api`, `db`, `backend`, `mobile`, `admin`.
  - #404 has no scope (`feat:`).
  - `notifications` isn't in the `scope-enum` of `commitlint.config.mjs` (a warning-level rule). orchestration.md itself uses `notifications` as its example of a scope that maps to the closest lane.

## 3. Every open issue

Each table gives the class, the labels and any open native blockers ("c" marks a closed blocker), with a one-line reason that notes where the body has drifted from the code or docs. Line numbers are at `origin/main` 4d13143.

### 3.1 Map #411: keep, renovate or rebuild before the Pilot

| # | Title | Class | Labels | Blocked by | Reason and drift |
|---|---|---|---|---|---|
| 411 | Wayfinder: keep, renovate or rebuild before the Pilot | live | `wayfinder:map` | – | Opened 2026-11-05, with sub-issues #412–#418, all open. **Decisions so far** is empty. Blocks 38 issues (the freeze). It cites **Pilot** in `GLOSSARY.md`, but the glossary on `main` has no Pilot entry; the entry is in open PR #419. |
| 412 | What the Pilot is | live | `wayfinder:grilling` | – | Open question. It overlaps #345 (running alone) and #346 (recruiting) on the soft-launch map, for the Pilot rather than the soft launch. Blocks #417. |
| 413 | Verdict criteria | live | `wayfinder:grilling` | – | Open question. |
| 414 | The hygiene bar | live | `wayfinder:grilling` | – | Open question. Blocks #418. |
| 415 | Feature and knowledge inventory | live | `wayfinder:research` | – | Open research. Blocks #417. |
| 416 | Tracker inventory | live | `wayfinder:research` | – | This ticket. Unassigned (in `docs/agents/issue-tracker.md:47` a claim is an assignment). Blocks #418. |
| 417 | The Pilot's scope | live | `wayfinder:grilling` | #412, #415 | Open question. |
| 418 | How work gets planned from here | live | `wayfinder:grilling` | #414, #416 | Open question. |

The frontier (open, unblocked and unassigned, in map order) is #412, #413, #414, #415, #416.

### 3.2 Map #334: soft launch in Da Nang and Hoi An

| # | Title | Class | Labels | Blocked by | Reason and drift |
|---|---|---|---|---|---|
| 334 | Wayfinder: soft launch in Da Nang and Hoi An | live | `wayfinder:map` | – | 16 sub-issues. The 6 closed ones (#335–#338, #393, #394) each have a line in **Decisions so far**; 10 are open. **Drift:** its Notes still list "App redesign #353" and "Web release #354" as tracks in progress. Both were closed on 2026-11-05: #353 as not planned (split into #396–#400) and #354 as a duplicate of #340. |
| 339 | Waitlist and unlock | live | `wayfinder:grilling` | #411 | Frozen. A comment recommends where unlock state should live and refers to #401 and #402. Blocks #342 and #346. |
| 340 | Web for launch | live | `wayfinder:grilling` | #411 | Frozen. A comment carried over from closed #354 records the web status: what is done and what is open. |
| 341 | Production stack | live | `wayfinder:grilling` | #411 (#335 c) | Frozen. |
| 342 | Analytics and consent | live | `wayfinder:grilling` | #362, #339 (#335 c) | Not on #411's freeze list, but blocked through #339, which is. |
| 343 | Distribution for each platform | live | `wayfinder:grilling` | #411, #362 (#335 c, #336 c) | Frozen. |
| 344 | Product gates for the soft launch | live | `wayfinder:grilling` | #411, #408 (#338 c, #393 c) | Frozen. Comments add the result of the walk eval and make this issue the place that decides whether #396–#400 gate the launch. |
| 345 | Running the soft launch alone | live | `wayfinder:grilling` | – | On #334's frontier. Overlaps #412. |
| 346 | Seeding plan for Da Nang–Hoi An | live | `wayfinder:grilling` | #339 (#337 c) | Not on the freeze list; blocked through #339. |
| 362 | Legal operator and Vietnam filings | live | `wayfinder:grilling` | – | On #334's frontier. Blocks #342 and #343. |
| 408 | Plans that combine activities | live | `wayfinder:grilling` | #411 | Frozen. Overlaps #348 (a curated list of activities that go well together): both decide which pairs of activities a plan may combine. Blocks #344. |

The frontier of #334 (open, unblocked and unassigned) is #345 and #362.

### 3.3 `ready-for-agent` (all 32 blocked by #411)

| # | Title | Class | Size | Other blockers | Reason and drift |
|---|---|---|---|---|---|
| 319 | build(deps): fix the open security alerts in the pnpm lockfile | live | – | – | Still accurate: the alerts API shows 28 open alerts, all in `pnpm-lock.yaml`. 11 are high (10 fixable), 11 medium (all fixable) and 6 low (5 fixable), so 26 have a patched version and 2 have none. No open Renovate PR is a security update. Its acceptance asks for a ROADMAP update. |
| 320 | feat(admin): preview a city-pack apply before running it | feature | – | – | Not built: `PreviewCityPackApply` doesn't exist. Asks for a ROADMAP update. |
| 321 | fix(backend): ListMetForPicks uses the shared attendance rule; crews.sql comment | live | – | – | Both still true. `ListMetForPicks` (`backend/db/queries/feedback.sql:40-47`) reads `f.attended`. The comment at `crews.sql:158` names `Service.RemoveFromUpcoming`, but the caller is `RemoveFromAll` (`crews.go:277-278`). Blocks #401. Asks for a ROADMAP update. |
| 323 | fix(mobile): a closed plan suggested in a crew chat says why, not "Signed up" | live | – | – | Still true: `SuggestedPlan` (`apps/mobile/src/features/chat/chat-panel.tsx:36-64`) doesn't read `closedReason`, and the stale "GetPlan only" comment is still in `proto/issho/v1/plans.proto` (~185). Blocks #327. Asks for a ROADMAP update. |
| 324 | fix(mobile): no share link for an outing whose group is cancelled | duplicate (in part) | – | – | Still true: `canShare` (`apps/mobile/src/features/events/outing.ts:146`) ignores the group's status. Its own Out of scope says #329 and #330 also cover this. Asks for a ROADMAP update. |
| 325 | test(notifications): pin that every push kind's catalogue key exists | refactor | – | – | Still true: `Text` (`backend/internal/uistrings/catalog.go:266`) returns an empty string for an unknown key, and `crews_test.go:163` still uses `strings.Contains(m.Body, "crew")`. There is no test that every key exists. Blocks #328. Asks for a ROADMAP update. |
| 326 | refactor(backend): one loader for plan cards, single and batch | refactor | – | – | Its line references still match (`service.go` `Cards` :363, `openPlan` :529, `closedPlan` :553, and the `plans.sql` queries). Blocks #332. Asks for a ROADMAP update. |
| 327 | refactor(mobile): planView() holds the plan rules | refactor | – | #323 | Not built: no `planView`. Asks for a ROADMAP update. |
| 328 | refactor(notifications): one titled renderer for plans and crews | refactor | – | #325 | Still true: `NotificationKinds` exists in both the plans and the crews package. Asks for a ROADMAP update. |
| 329 | feat(api): MyEvent carries what the caller can do when, from plans.Membership | feature | – | – | Not built: there is no `ActionWindow`, `OutingAction` or `OnTheDayFrom`, and `MyEvent` has no `time_zone`. Asks for a ROADMAP Decision-log row. Blocks #330 and #333. |
| 330 | refactor(mobile): outing() reads allowed actions and on-the-day from the API | refactor | – | #329 | Not built. Asks for a ROADMAP update. |
| 331 | refactor(db): one definition of "goes ahead" and "feedback due", in app.attendance | refactor | – | – | Still true: `went_ahead` is at `backend/db/migrations/00003_domain_rule_views.sql:25`, and `ListGroupsToSettle` filters to `scheduled` outings (`events.sql:89`). Its Ordering note says #321 and #332 also add goose migrations, but #321's body describes only a query change. "Goes ahead" isn't in `GLOSSARY.md`. Asks for a ROADMAP Decision-log row. Blocks #332. |
| 332 | refactor(db): plan_cards view for the card columns | refactor | – | #331, #326 | Its body says to close it as wontfix if the view proves more awkward than the duplication, since #326 removes most of that. Asks for a ROADMAP update. |
| 333 | feat(mobile): an outing's time in the city's time zone on tap | feature | – | #329 | Not built. Asks for a ROADMAP update. |
| 347 | feat(matching): meals and beverages become an optional wind-down | feature | – | – | Moved from the ROADMAP; the spec is in an Agent Brief comment dated 2026-11-05. `brunch` and `dinner-meetup` are still activities in `data/activities.json`. The brief says to read ADR 0002 and the glossary terms "from #395; land that first". #395 is an open PR. Blocks #348. |
| 348 | feat(matching): a curated list of activities that go well together | feature | – | #347 | The spec is in an Agent Brief comment. Overlaps #408. |
| 350 | feat(backend): machine translations take the livelier English as a tone guide | feature | – | – | The spec is in an Agent Brief comment. `backend/internal/uistrings/human/en.json` still has 2 entries, as the issue says. |
| 355 | feat(mobile): a first-run spotlight tour | feature | – | – | Not built: no `TourProvider`. Its body leaves whether it gates the launch to #344. |
| 356 | feat(matching): join offers let someone new into an existing crew | feature | – | – | The spec is in an Agent Brief comment, which uses glossary terms from #395. Those terms aren't on `main`. |
| 357 | fix(data): activities with no venue type name a place, via text-search categories | live | – | – | Still true. 8 activities have neither `venue_categories` nor a walk theme: cycling, bouldering, martial arts, archery, skiing, cooking class, anime and escape room. Photography has an empty list plus a walk theme, which makes the nine the issue names. A comment on #317 makes #317 wait for this one. |
| 358 | feat(api): re-accept a new code of conduct | feature | – | – | Still true: `TestConductVersionIsOne` is at `backend/internal/profile/validate_test.go:152`, and there is no `AcceptConduct`. |
| 396 | feat(api): chat rooms report their unread count and last message | feature | – | – | Split from #353. Not built: no `MarkRead` or `ListRoomSummaries`. |
| 397 | feat(api): on the day, the group sees who has checked in | feature | – | – | Split from #353. Not built: no `Attendee` message. |
| 398 | feat(api): fellow crew members see who of them signed up | feature | – | – | Split from #353. Its comment records an owner decision of 2026-11-05, a narrow exception to the decision-log row "Interest counts are shown, never who" (`docs/decisions-2026.md:79`). Not built. |
| 399 | feat(api): the crew screen lists your past outings together | feature | – | – | Split from #353. Not built. |
| 401 | refactor(db): app.participants, one definition of who matching may place in a city | refactor | M | #321 | `app.participants` doesn't exist. Blocks #406. |
| 402 | refactor(backend)!: one Standing in identity for the gate, chat, push and GetMe | refactor | L | – | Carries `breaking-ok`, which is a PR label. "Standing" isn't in `GLOSSARY.md`. |
| 403 | refactor(backend): a reports module owns reasons, validation and kept apart | refactor | M | – | `backend/internal/reports` doesn't exist. Blocks #404. |
| 404 | feat: block someone, a kept-apart pair without a moderator queue | feature | L | #403 | No `rpc Block` yet. Unlike its siblings, its title has no scope and the issue has no `go` label. |
| 405 | refactor(backend): one stage runner for the worker passes' failure policy | refactor | S | – | `backend/internal/stage` doesn't exist. |
| 406 | refactor(backend): internal/citydata loads a city once for plans, crews, metrics and sim | refactor | L | #401 | `backend/internal/citydata` doesn't exist. |
| 407 | fix(backend): cities.Apply runs every pack check on every path | live | S | – | Still true: `Apply` (`backend/internal/cities/apply.go:73`) runs no checks, and the callers at `citypacks/handler.go:236` and `cmd/tool/main.go:169` match its table. **Drift:** `ValidatePack` is also called from `backend/cmd/tool/boundaries.go:80` and inside `cities/edit.go:43,79` (via `CheckPack`), and the issue's table lists neither. |

### 3.4 `ready-for-human` (none frozen)

| # | Title | Class | Blocked by | Reason and drift |
|---|---|---|---|---|
| 239 | Ops: turn on the backup alert (push the image, Sentry monitor and rule) | owner chore | – | The steps are still in `docs/deploy.md:108`. The repo can't show whether any step has been done. |
| 290 | discover-venues keeps what it found when a search fails, and waits out per-minute quota | live (parked) | – | Code work, parked. Branch `backend/290-venues-keep-results` (head cbaae88, 2026-11-04) is 2 commits ahead of `main` and 28 behind. On `main`, `discoverVenues` (`backend/cmd/tool/main.go:244`) still saves only at the end (`:291`). The comment's remaining steps include a ROADMAP Decision-log row. |
| 306 | Design: revisit the app's type scale after #204's snapping | owner chore | – | A design decision. Its "Blocked by #204" is text only, and #204 is closed (PR #304). `docs/design-system.md:71` gives the scale as it is now: caption 12, small 14, default 16, heading 22, subtitle 26, title 44. Asks for an entry in the ROADMAP decision log. |
| 315 | Translate the 16 catalogue keys from #202 and refresh the UI bundle | stale | – | **Drift:** the bundled `es.json` and `ja.json` each lack 28 of the 380 keys in `apps/mobile/src/i18n/en.json`, not 16. It treats Japanese and Spanish as the target languages, while the soft launch is in Vietnam, which has no bundle. Asks for the ROADMAP's M11 line to be ticked. |
| 316 | Rebuild dev clients built before react-native-maps | owner chore | – | An EAS rebuild. Asks for the M11 line to be ticked. Blocks #400. |
| 317 | Rerun Tokyo venue discovery after #290 | owner chore | – (text: #290) | Its "Blocked by #290" has no native edge. A comment adds #357 as a prerequisite and asks for a Da Nang–Hoi An run as well. Asks for the M11 line to be ticked. |
| 349 | Decide how many plans a city gets (users_per_plan) | owner chore | – | An owner decision. The default is still 10 (`backend/internal/matching/config.go:210`), and no city pack sets `users_per_plan`. |
| 351 | Renew GITHUB_TOKEN before it expires on 2027-01-22 | owner chore | – | A dated reminder. |
| 352 | Check per-language fonts and right-to-left layout on phones | owner chore | – | Phone checks. The code they cover exists: `BodyFontLanguages` and `DisplayFontLanguages` (`apps/mobile/constants/theme.ts:126,129`) and `src/i18n/direction.ts`. |
| 359 | Re-check Google Maps caching terms before launch | owner chore | – | `docs/architecture.md:176`, under Known risks, links to it, and the two texts match. |
| 400 | Check the redesign on phones | owner chore | #316 | Split from #353. A comment adds a check of #355's tour on Android. |

### 3.5 Neither ready label

| # | Title | Class | Reason |
|---|---|---|---|
| 269 | Dependency Dashboard | live (bot) | Renovate's issue, with no labels. Its sections are Pending Status Checks (2), Open (29) and Detected Dependencies. There is no Rate-Limited section and no vulnerability section. Section 5 compares it with `docs/onboarding.md`. |
| 360 | Post-MVP ideas (deferred) | fog | A list of ideas, untriaged on purpose and deferred until after the public launch. Its M11 reference has no milestone behind it. Two of its items (Stripe Identity and the crew-offer walkthrough) also appear in #334's Out of scope. |

## 4. Structure: sub-issues and dependencies

**Sub-issues** are used only by the two maps: #334 has 16 children (6 closed) and #411 has 7 (none closed). No other open issue has a parent or children. The closed map #140 had 15 children (#141–#155), all closed.

**Native "blocked by" edges**, open blockers only:
- **#411** blocks 38 issues: all 32 `ready-for-agent` issues, plus #339, #340, #341, #343, #344 and #408. That matches the freeze list in #411's Notes exactly.
  - #342 and #346 are blocked only through #339; #342 is also blocked by #362.
- **Inside the frozen set:**
  - #323 → #327
  - #325 → #328
  - #329 → #330 and #333
  - #326 and #331 → #332
  - #347 → #348
  - #321 → #401 → #406
  - #403 → #404
- **Elsewhere:**
  - #316 → #400
  - #339 → #342 and #346
  - #362 → #342 and #343
  - #408 → #344
  - #412 and #415 → #417
  - #414 and #416 → #418

**Text-only dependencies**, with no native edge:
- #306 "Blocked by #204" (#204 is closed).
- #317 "Blocked by #290" (#290 is open). #317's comment also names #357.
- #331's Ordering note covers #321 and #332. Only #331 → #332 exists natively.

**Pending docs in open PRs.** The tracker points at docs that exist only in four open PRs, none of which comes from Renovate:
- #395: ADR 0002 and the glossary terms used by #347 and #356;
- #409: glossary terms for walk and ride;
- #410: the walk eval in Da Nang;
- #419: the Pilot glossary entry, used by #411 and #412.

**Assignment.** No open issue is assigned. 78 of the 88 closed issues were assigned.

## 5. Where docs and tracker disagree

| Doc | What it says | What the tracker shows |
|---|---|---|
| `CLAUDE.md`, ADR 0001, `docs/decisions-2026.md` | `docs/ROADMAP.md` was retired on 2026-11-04. Decisions go in resolution comments or ADRs. The decision log is frozen. | 18 open issues (#306, #315–#317, #319–#321, #323–#333) still ask for a ROADMAP update, an M11 line ticked, or a ROADMAP Decision-log row, and so does #290's comment. "M11" appears in #315, #316, #317, #334, #344 and #360, but no GitHub milestones exist. |
| `GLOSSARY.md`, `docs/adr/` on `main` | The glossary has no Pilot, Standing or Goes ahead entry. Only ADR 0001 exists. | #411 and #412 cite Pilot in `GLOSSARY.md`. #347 and #356 cite ADR 0002 and terms "from #395". #331 and #402 use Goes ahead and Standing. The missing text is in open PRs #395 and #419 (and #409 for walk and ride). |
| `apps/mobile/README.md:169` | Links #354 "Web release" as the place to follow web work. | #354 is closed as a duplicate of #340, which holds the web status. |
| `docs/onboarding.md:124-129` (Dependency updates) | Renovate opens PRs on Monday mornings. Most pending updates sit under "Rate-Limited" because of `prConcurrentLimit: 5`. | `renovate.json5` still sets `schedule: ["before 7am on monday"]` and `prConcurrentLimit: 5`. Yet 28 Renovate PRs are open, all created on Thursday 2026-11-05 between 03:52 and 03:57 UTC. One of them is lock-file maintenance, which is scheduled for the 1st of the month. #269 has no Rate-Limited section. |
| `.claude/orchestration.md`, Backlog | A ticket is takeable when it is open, `ready-for-agent`, unassigned and unblocked, with no open sub-issues and no `wayfinder:` label. | No ticket is takeable: all 32 are blocked by #411, as the freeze intends. |
| `.claude/orchestration.md`, Tiers | A `size:` label overrides the computed tier. | Only 7 of the 32 `ready-for-agent` issues have one. |
| `.claude/orchestration.md`, lanes; `commitlint.config.mjs` | The lane is derived from the title's scope, with `notifications` as the example. | `notifications` (#325, #328) isn't in commitlint's `scope-enum`, and #404's title has no scope. |
| `docs/agents/issue-tracker.md:44-48` (Wayfinding) | A claimed ticket is assigned. Resolving a ticket appends a context pointer to the map's Decisions so far. | #334 has a pointer for each of its 6 closed children. #411 has no closed children yet, and none of its children is assigned. |
| `docs/agents/triage-labels.md` | Defines five triage roles. | Only `ready-for-agent` and `ready-for-human` are on open issues. |
| `docs/decisions-2026.md:79` | "Interest counts are shown … never who." | #398's comment records an owner exception. Because the log is frozen, the exception is recorded only on #398. |
| `docs/decisions-2026.md`, final row | The owner's leftovers are tracked as M11 issues (#290, #306, #315–#317). | There is no M11 milestone. All five issues are open and labelled `ready-for-human`. |
| `docs/architecture.md:176`, `docs/contributing.md`, `docs/onboarding.md` §7 | Known risks links #359. Contributors are pointed at Issues, the triage labels and the ADRs. | Consistent. |

## 6. Closed issues

- **88 closed issues**, numbered #140–#394; the missing numbers in that range are PRs.
  - By outcome: 84 completed, 3 not planned, 1 duplicate.
  - By date: 5 on 2026-11-02, 70 on 11-03, 8 on 11-04, 5 on 11-05.
- **61 were closed by a merged PR**, all as completed. None was closed by an unmerged PR.
- **27 were closed by hand**, each with a comment:
  - **22 wayfinder issues:**
    - the map #140, whose comment says its backlog went into the ROADMAP's M10 section;
    - its 15 children, #141–#155;
    - #334's six closed children: #335–#338, #393 and #394.

    Each has a resolution comment.
  - **#242**, not planned: moved off Dependabot.
  - **#244**, closed with reason "not planned", although its comment says it duplicates #229.
  - **#245**, completed: "Done in #247", a PR that didn't close it automatically.
  - **#353**, not planned: split into #396–#400.
  - **#354**, closed as a duplicate of #340.
- **None was closed without a resolution.** Every issue closed by hand has a comment saying why.
- 63 of the closed issues still carry `ready-for-agent`.

## 7. Other context

- **Open PRs: 32.**
  - 28 are from Renovate, and none of them is a security update.
  - 4 are the owner's docs PRs: #395, #409, #410 and #419.
  - No release-please PR is open, although its branch exists.
- **Parked work.** `backend/290-venues-keep-results` holds #290's unmerged commits.
- **Earlier research.** Branches from earlier wayfinder tickets keep their findings in `docs/research/<name>.md` on their own branches, for example `research/vietnam-social-app-rules` and `research/two-wheeler-routing-vietnam`.
- **Provenance.**
  - 30 open issue bodies carry the "generated by AI" disclaimer.
  - 12 were moved from the ROADMAP on 2026-11-04: #347–#352 and #355–#360.
  - Seven of those (#347, #348, #350, #355–#358) have an Agent Brief comment dated 2026-11-05 that holds the spec, and so do #396–#399.
- **Where tickets came from:**
  - #319–#321 were filed at the close of M10;
  - #323–#333 came from the 2026-11-04 architecture review;
  - #401–#407 came from the 2026-11-05 review.
