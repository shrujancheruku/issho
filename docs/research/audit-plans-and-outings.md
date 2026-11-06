# Audit: plans and outings

Research for [#421](https://github.com/shrujancheruku/issho/issues/421), a child of the wayfinder map [#411](https://github.com/shrujancheruku/issho/issues/411). It scores this area against [Verdict criteria (#413)](https://github.com/shrujancheruku/issho/issues/413), measures it against [The hygiene bar (#414)](https://github.com/shrujancheruku/issho/issues/414) and judges only what [The Pilot's scope (#417)](https://github.com/shrujancheruku/issho/issues/417) keeps in. It gives facts and recommendations only. The owner settles verdicts in the verdict ADR.

Audited at `origin/main` 139d77e (2026-11-06). CI on that commit is green, including the database job. File:line references are to that commit.

## Summary

| Part | Tests | Structure | Domain fit | Docs | Defects | Refactor load | Renovate | Rebuild | Recommended verdict |
|---|---|---|---|---|---|---|---|---|---|
| A. Backend `internal/plans` | amber | amber | amber | amber | amber | **red** | ~8 sessions | ~16 sessions | **Renovate** |
| B. Mobile plans and outings (`features/plans`, `features/events`, the Plans tab, the plan, signup, event and feedback screens) | amber | amber | amber | amber | amber | amber | ~4 sessions | ~8 sessions | **Renovate** |
| C. Admin plan and group pages (Plans, NewPlan, PlanDetail, StopsEditor, Groups, GroupReview) | amber | green | green | green | green | green | ~1 session | ~4 sessions | **Keep** |

No part of this area is parked or removed by #417. The whole area is in scope: plans (rounds, feed, signups, plan screen), outings (invites, confirm, check-in, meeting point, routes), the opening-hours check, walks, feedback with "rather not" picks, and the admin pages Plans, NewPlan, PlanDetail, StopsEditor and Groups/GroupReview.

### How the split was made

#413 allows a split only along a package or module boundary. `internal/plans` is **one Go package** (59 files, ~6.2k source lines and ~4.6k test lines, per the [inventory](https://github.com/shrujancheruku/issho/blob/research/feature-and-knowledge-inventory/docs/research/feature-and-knowledge-inventory.md) §3.3). Its sub-areas (rounds, feed, outings, review, stops, feedback, walk orchestration) are files, not packages, so the backend gets one verdict. The mobile code and the admin pages are separate modules in separate apps (`apps/mobile`, `apps/admin`), with their own tests and builds, so each gets its own verdict.

## A. Backend `internal/plans`

**What it is.** Four handlers over one `Service` and one `Runner`:

- `Handler` (PlanService: `ListFeed`, `GetPlan`, `GetPlanCards`, `SignUp`, `Withdraw`), `backend/internal/plans/handler.go:32-117`
- `EventHandler` (EventService: `ListMyEvents`, `GetEvent`, `RespondToEvent`, `CheckIn`, `GetFeedback`, `SubmitFeedback`, `ReportPerson`), `events_handler.go:29-171`
- `ReviewHandler` (ReviewService: 21 admin RPCs), `review.go:62-300`, `review_events.go:30-223`, `stops.go:72-277`
- `Runner` (the worker's passes: `Settle`, `Schedule`, `Suggest`, meeting points, routes), `run.go:230-370`, `events.go:391`, `walks.go:172-409`

The largest files are `service.go` 648, `run.go` 517, `review.go` 480, `store.go` 478, `events.go` 472 and `walks.go` 452 lines.

### Signals

**1. Test quality: amber.**

- 51 test functions. 39 are integration tests against a real Postgres (`testfixture.New` skips them unless `TEST_DATABASE_URL` is set, `backend/internal/testfixture/testfixture.go:33-38`; CI sets it). Locally without a database, 12 pass and 39 skip.
- The rules are pinned well. `Membership` (`membership.go:12-194`) is pure, with table tests in `membership_test.go`. Rounds, hours windows, check-in, meeting points, routes, feedback, proposals, stops, races (`withdraw_race_test.go`), resilience (`resilience_test.go`, 5 tests), paging and caps each have a test file. These can act as a spec for a renovation.
- **Gap:** the admin `ReviewHandler` is tested at handler level (`review_test.go:35-155`, `proposals_test.go:37-194`, `stops_test.go:92-224`, `paging_test.go:59-160`), apart from `ListVenueOptions`. The 12 user-facing RPCs of `Handler` and `EventHandler` are tested only through `Service` (for example `SignUp` and `CheckIn` in `service_test.go` and `checkin_test.go`). So `connectError` (`handler.go:148`), `eventError` (`events_handler.go:234`), the proto mapping (`eventToProto` `events_handler.go:290`, `planToProto` `handler.go:182`, `signupFromProto` `handler.go:247`) and caller extraction have no direct test. This misses the hygiene-bar gate "every in-scope RPC has a handler-level test".

**2. Structure and coupling: amber.**

- One package holds four services' worth of code. File seams are clear (handler/service/store per concern), and it changes in place routinely: 49 commits since 2026-10-20.
- It is a hub. `crews` uses 17 exported `plans` symbols (`CheckReport`, `Membership`, `LoadCity`, `Runner`, `SaveEvent`, …), `moderation` uses 12 and `chatws` uses `ChatAccess`. Two concerns live here that belong elsewhere: report and kept-apart rules (`feedback.go:247-290`, `CheckReport`/`Report`, which `crews.go:229,329` also call; see #403), and the city snapshot loader for matching (`load.go:54-98`, `feedback.go:300`, `citycache.go`; see #406).
- Duplication kept equal only by comments: plan cards are loaded two ways, with a 19-field hand copy into `dbgen.ListOpenPlansRow` (`service.go:399-409` vs `closedPlan` `:553`; see #326 and #332). The worker failure policy is written as prose and re-implemented per loop (`run.go:481-494` vs `crews/pass.go:134`; see #405). The notification render loop is duplicated with crews (`notifications.go:42-77`; see #328).
- Dead code: `deadcode ./...` (golang.org/x/tools v0.28.0) reports 3 unreachable functions: `DefaultTiming` (`run.go:48`), `Create` (`store.go:115`) and `LoadSignups` (`store.go:248`). All three are used only by tests or by the parked simulator's tests.
- Not red: there is no tangle that prevents a change in place, and every coupling problem already has a scoped ticket.

**3. Domain fit: amber.**

- The model matches `GLOSSARY.md`. A plan with no date becomes an outing ("the code and the API call it an event", which the glossary sanctions). **Attended** and **Met** come from the `app.attendance` view (`backend/db/migrations/00003_domain_rule_views.sql:24-26`). **Kept apart** is written by `Report` and by "rather not" picks. **Stage** is a client concept (see B). The decision log's rules (opening-hours check, approval = scheduling, proposal rest) are implemented.
- The rules are duplicated, and the copies have drifted:
  - "Went ahead" has two meanings: `went_ahead` = `e.status = 'done' and g.status <> 'cancelled'` (`00003_domain_rule_views.sql:25`). The feedback rule is written three times: `Membership.FeedbackWindow` (`membership.go:146-156`), `ListUserEvents` (`events.sql:29-30`) and `QueueFeedbackRequests` (`feedback.sql:116-121`). A group can get stuck as `inviting` at a finished outing (#331).
  - `ListMetForPicks` uses its own "met" rule that ignores check-ins (#321).
  - `Membership` holds every outing rule, but `eventToProto` (`events_handler.go:290-324`) sends only raw statuses, so the app derives the rules again and gets them wrong (#329, #324).
- Not red: the model is right. The fixes are "use the one definition", not a different model.

**4. Docs match: amber.**

- The endpoint table (`backend/README.md:135-194`) matches the handlers, RPC by RPC.
- `backend/README.md:56` describes `internal/plans` as `PlanService`, `Runner`, `LoadCity` and the store. It leaves out `EventService`, `ReviewService`, feedback, proposals, stops and walks, and it names `LoadSignups`, which is now unreachable.
- `backend/README.md:278` says "Not done yet: chat, push notifications, the travel-matrix refresh and AI-written plan text". All but the last exist (inventory §10).
- The "Plans, signups and events" section (`backend/README.md:256-278`) is otherwise accurate.

**5. Defect history: amber.** Since 2026-10-20 there were 5 `fix` commits out of 49 commits touching `backend/internal/plans`:

- 58c2b37: venue opening window from the activity's hours (#282, issue #168)
- faf3184: withdraw and chat-send races (#276, issue #188)
- dcdde08: one item never stops a pass or a push batch (#223)
- 9224ace: review fixes (#134)
- 8e57310: bracketed examples in plan titles

There is one open bug (#321), and #331 describes a live defect (a stuck `inviting` group). No outing-losing bug is open.

**6. Pending refactor load: red.** Frozen tickets that target the package:

- #326: one card loader
- #331: one "goes ahead"/"feedback due"
- #332: `plan_cards` view (db)
- #329: `MyEvent` carries the action windows
- #328: one notification renderer with crews
- #321: a fix
- #401: `app.participants`
- #403: a reports module (M)
- #405: one stage runner (S)
- #406: `internal/citydata` (L)
- #402: Standing (L), which touches the gate that `plans` relies on
- #397: feature, group check-ins on the day

That is 10 refactors or fixes plus features, two of them size L.

### Cost to reach the hygiene bar

- **Renovate: ~8 agent sessions.**
  - 1: handler-level tests for the 12 user RPCs plus `ListVenueOptions`
  - 1: #326 with #332
  - 1: #331 with #321
  - 1: #329
  - 1: #403
  - 2: #406
  - 0.5: #405 with #328
  - 0.5: README rows `:56` and `:278`, plus the dead-code clean-up
  - #401 and #402 are counted under the identity and moderation audits.
- **Rebuild: ~16 agent sessions.** About 6.2k source lines, ~95 sqlc queries in `plans.sql`, `events.sql`, `feedback.sql` and `review.sql`, 33 RPCs, and the worker passes, rebuilt next to the old ones. It would then have to re-reach today's test depth (4.6k test lines) and switch `crews`, `moderation`, `chatws`, `worker` and `cmd/api` over.

### Rule applied

Domain fit is amber, not red, so "Rebuild if domain fit is red" doesn't apply. Tests and structure are both amber, so "Rebuild if both are red" doesn't apply. Refactor load is red, so the part isn't Keep (Keep needs no reds), and the cost is over 2 sessions anyway. Refactor load alone never forces a rebuild. **Renovate.** Cost check: renovate (~8) is cheaper than rebuild (~16).

## B. Mobile plans and outings

**What it is:**

- `apps/mobile/src/features/plans`: 31 files, ~1.76k source lines (feed data, cards, signup, maps native and web, route polylines, timeline)
- `features/events`: `outing.ts` 160, `data.ts` 76, `feedback.ts`, `share.ts`
- The routes `app/(tabs)/plans.tsx` 300, `app/plan/[id].tsx` 219, `app/signup/[id].tsx` 171, `app/event/[id].tsx` 355 and `app/feedback/[id].tsx` 239

### Signals

**1. Test quality: amber.**

- `pnpm exec jest src/features/plans src/features/events` runs 14 suites and 82 tests, all passing.
- The pure rules are pinned: `outing.test.ts` (154 lines, the outing stage and actions), `signup.test.ts`, `sections.test.ts`, `map-data.test.ts`, `route-path.test.ts`, `feedback.test.ts` and `share.test.ts`. `respond.test.tsx` pins the cache invalidation.
- Screen tests exist for the Plans tab (`plans-tab.test.tsx`) and the signup sheet (`signup-sheet.test.tsx`).
- **Gap:** the plan screen (`app/plan/[id].tsx`), the outing screen (`app/event/[id].tsx`) and the feedback screen (`app/feedback/[id].tsx`) have no render or interaction test. They are core-loop screens (#414 gate).

**2. Structure and coupling: amber.**

- Module boundaries are clean: routes are thin and import from `features/*`. `sh` import check: every file in `features/plans` and `features/events` is imported somewhere, so there are no orphan files. Knip was not run in this audit.
- The plan rules are spread over about 11 checks across screens and the crew-chat card, which is how the card drifted (#327, #323).
- `outing.ts` re-derives server rules (`eventState`, `outing.ts:25-61`; `stageOf`, `:78-87`; see #330).
- `event/[id].tsx` is the largest screen at 355 lines.

**3. Domain fit: amber.**

- **Stage** (invited, going, on the day, after, muted) and **Outing** are named as in the glossary.
- The client copy of the rules disagrees with the server:
  - `canShare` is true for a scheduled event even when the group is cancelled (`outing.ts:146`; see #324). The server treats a cancelled group as over (`membership.go:85-92`).
  - Cancel is hidden after check-in, which the server allows.
  - "On the day" is decided in the device's time zone (#329, #330). The owner has settled these in the 2026-11 architecture review: cancel after check-in, 02:00 on-the-day, time zone on tap (#333).

**4. Docs match: amber.**

- `apps/mobile/README.md:83` and `:88` describe `features/plans` and `features/events` accurately.
- `README.md:88` calls `outing.ts` "the one place the rules about an outing live". That is not true while `plans.Membership` holds the same rules on the server.
- The plan screen's behaviour is documented only in code (inventory §11).

**5. Defect history: amber.** Since 2026-10-20 there were 6 `fix` commits out of 39 commits touching these paths:

- 2aa1d13: keep loaded data when a refetch fails (#273)
- 3a37d0b: map crashes (#111)
- 6ed1ae2: Google attribution (#37)
- 8e57310: bracketed examples
- 9b93606: plan screen layout and stale text
- ec787b7: `toSorted` on Hermes

Two bugs are open: #324 (share link for a cancelled group) and #323 (a closed plan in crew chat says "Signed up"; the card lives in `features/chat`, but the fix is the plan rule from #327).

**6. Pending refactor load: amber.** #327 (`planView()`), #330 (`outing()` reads the server's actions, which depends on #329), #333 (the city's time zone on tap), #324 and #323. All are small or medium, and none is size L.

### Cost to reach the hygiene bar

- **Renovate: ~4 agent sessions.**
  - 1: #330, after #329 on the backend; this also fixes #324
  - 1: #327, with #323
  - 1.5: render-and-interaction tests for the plan, outing and feedback screens
  - 0.5: the README line, plus #333 if it is kept
- **Rebuild: ~8 agent sessions.** About 3k lines of routes and features, re-created next to the old ones, including maps on native and web, which took several fix rounds (#111, #203 map parity).

### Rule applied

Domain fit is amber, and tests and structure are amber. There are no reds, but the cost is over 2 sessions, so the part isn't Keep. **Renovate.** Cost check: renovate (~4) is cheaper than rebuild (~8).

## C. Admin plan and group pages

**What it is:** `apps/admin/src/pages/Plans.tsx` 95, `NewPlan.tsx` 99, `PlanDetail.tsx` 124, `StopsEditor.tsx` 172, `Groups.tsx` 69 and `GroupReview.tsx` 130 lines.

### Signals

**1. Test quality: amber.**

- `pnpm exec vitest run` on `PlanDetail.test.tsx`, `StopsEditor.test.tsx` and `GroupReview.test.tsx` runs 30 tests, all passing.
- They cover approve, reject and withdraw a plan, approve and reject a proposed round, the walk editor, the exact RPC requests, and the error, cancel and double-click cases (`apps/admin/README.md:81-83`).
- `Plans.tsx` and `Groups.tsx` are untested on purpose: they are list and navigation pages with no irreversible action (`README.md:81`). The router covers them only through stubs (`App.test.tsx:33-34`).
- **Gap:** `NewPlan.tsx` is untested. It calls `api.review.createPlan` (`NewPlan.tsx:28`), and it is how the owner seeds plans by hand during the Pilot (#412).

**2. Structure and coupling: green.** The pages are small, use the shared `ConfirmButton`, `paging.tsx` and `lib.ts`, and talk only to `ReviewService` through `@issho/api-client`.

**3. Domain fit: green.** "Proposal" for a group review is the term the glossary reserves (`GLOSSARY.md`, under **Pending change**: "Avoid: proposal (a plan's group review)"). Approval = scheduling, as in the decision log (2026-10-24).

**4. Docs match: green.** Every page has a row in the admin README. One open note: "Load more not yet checked in a browser" (`README.md:20`) covers Plans. It is an honest open check, not a contradiction.

**5. Defect history: green.** No `fix` commits out of 7 commits since 2026-10-20, and no open bugs.

**6. Pending refactor load: green.** No frozen ticket targets these pages.

### Cost to reach the hygiene bar

- **Keep or renovate: ~1 agent session.** It covers:
  - a `NewPlan` test (success, error, double-click)
  - optional smoke tests for Plans and Groups, if the owner reads the gate as covering them
  - the browser check of Load more
- **Rebuild: ~4 agent sessions.**

### Rule applied

There are no reds, and the cost is about 1 session (≤ 2). **Keep.**

## Hygiene-bar Pilot-gate gaps in this area

1. **Handler-level tests (gate: every in-scope RPC).** Thirteen RPCs have none:
   - PlanService: `ListFeed`, `GetPlan`, `GetPlanCards`, `SignUp`, `Withdraw`
   - EventService: `ListMyEvents`, `GetEvent`, `RespondToEvent`, `CheckIn`, `GetFeedback`, `SubmitFeedback`, `ReportPerson`
   - ReviewService: `ListVenueOptions`

   The other 20 ReviewService RPCs have handler-level tests.
2. **Core-loop mobile screens (gate: a render-and-interaction test each).** None exists for `app/plan/[id].tsx`, `app/event/[id].tsx` or `app/feedback/[id].tsx`. The Plans tab and the signup sheet have one.
3. **Admin pages the owner runs the Pilot with.** `NewPlan.tsx` (creating a plan by hand) is untested. Plans and Groups are untested by design; the owner should decide whether the gate covers list pages.
4. **End-to-end core-loop test.** There is none in the repo: no Playwright, Maestro or Detox, and no e2e directory. Every step of it (sign up for a plan, matched, outing dated, attend, check in, feedback) runs through this area.
5. **Dead-code check.** `deadcode` flags `DefaultTiming` (`run.go:48`), `Create` (`store.go:115`) and `LoadSignups` (`store.go:248`). They are test-only, so the check needs `-test` or these must move into test files. Knip was not run on the mobile or admin code.
6. **Docs contradictions:**
   - `backend/README.md:278` (in #415's list)
   - `backend/README.md:56` (package row out of date; not in #415's list)
   - `apps/mobile/README.md:88` ("the one place the rules live")
7. **Code names match glossary terms.** They do. "Event" in code and API is sanctioned by the glossary's **Outing** entry. No rename is needed for the gate.
8. **"No outing fails because of a bug" (success bar).** #331's stuck-`inviting` group and #324's share link for a cancelled group are live defects on the outing path. Fix them before the Pilot, whatever the verdict.

## Parked and removed parts touching this area

None of this area's parts is parked or removed. Two consumers of `plans` are, and they constrain a renovation:

- **Parked:** the matching simulator (`internal/matching/sim`) imports `plans`. `go vet` and `go test ./internal/matching/sim/` pass at 139d77e. A `plans` renovation must keep it compiling.
- **Removed:** `internal/walks/bench` imports `plans` (one call site). Its removal (scored in the matching and walks audit, #422) takes away one consumer. `go test ./internal/walks/bench/` passes today.
- Five `plans` test files mention Tokyo, but only as the `Asia/Tokyo` time zone of their own `zz-plans-*` fixture cities (for example `city_config_test.go:18` and `proposals_test.go:26`). They create those cities through `testfixture` and never load the Tokyo city pack, so removing the pack doesn't break them.

## Sources

- Code at `origin/main` 139d77e. `go vet`, `go test` (unit; integration via CI run on 139d77e, green), `deadcode` v0.28.0, Jest and Vitest runs as quoted.
- `git log --since=2026-10-20 --grep='^fix'` on the paths above.
- Issues #321, #323, #324, #326–#333, #397, #401–#406 (bodies quoted for line references).
- [Feature and knowledge inventory](https://github.com/shrujancheruku/issho/blob/research/feature-and-knowledge-inventory/docs/research/feature-and-knowledge-inventory.md) §3.3, §10, §11.
- `GLOSSARY.md`, `backend/README.md`, `apps/mobile/README.md`, `apps/admin/README.md`.
