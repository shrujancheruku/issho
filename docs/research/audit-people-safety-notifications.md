# Audit: people, safety and notifications

Research for [#423](https://github.com/shrujancheruku/issho/issues/423), a child of the wayfinder map [#411](https://github.com/shrujancheruku/issho/issues/411). It scores each part of this area against [Verdict criteria (#413)](https://github.com/shrujancheruku/issho/issues/413), measured against [The hygiene bar (#414)](https://github.com/shrujancheruku/issho/issues/414), for what [The Pilot's scope (#417)](https://github.com/shrujancheruku/issho/issues/417) keeps in. It gives facts and recommendations only. The owner decides in the verdict ADR (#429).

- **Code read at:** `origin/main` 139d77e (2026-11-06). File:line references are to that commit.
- **Other sources:** the [Feature and knowledge inventory](https://github.com/shrujancheruku/issho/blob/research/feature-and-knowledge-inventory/docs/research/feature-and-knowledge-inventory.md) (#415), `GLOSSARY.md`, the backend, mobile and admin READMEs, and `gh` issue and PR history.
- **Defect history:** `git log --since=2026-10-20 -i --grep=^fix -- <paths>`, plus bug issues filed since 2026-10-20.
- **Checks run locally:** `go test` on `identity`, `profile`, `moderation`, `notifications`, `push`, `metrics`, `tagging/...` and `catalog -run Suggest`. All pass. Integration tests skip without `TEST_DATABASE_URL`. CI runs them, and main's last runs at 139d77e are green. `deadcode ./...` (golang.org/x/tools v0.38.0) reports nothing in these packages. JS tests and `knip` were not run, because the worktree has no `node_modules`.

## Summary

| Part (sub-part) | Tests | Structure | Domain fit | Docs | Defects | Refactor load | Renovate / rebuild (sessions) | Recommended verdict |
|---|---|---|---|---|---|---|---|---|
| Identity, backend (`internal/identity`) + sign-in screen | amber | green | amber | green | green | amber (#402) | 0.5 / 2 | **Keep** |
| Profile, backend (`internal/profile`) | amber | green | green | amber | green | green | 0.5 / 2 | **Keep** |
| Profile, mobile (`app/onboarding.tsx`, `app/profile-edit`, `features/profile`) | **red** | amber | green | green | amber | green | 1.5 / 3.5 | **Renovate** |
| Moderation, backend (`internal/moderation`, plus report recording in `plans`/`crews`) | green | amber | amber | amber | amber | **red** (#402, #403, #404) | 2.5 / 5 | **Renovate** |
| Safety and account screens, mobile (`app/report`, You tab account section, suspended screen) | **red** | amber | green | amber | green | amber (#404) | 1 / 1.5 | **Renovate** |
| Admin moderation pages (Reports, Flags, People, Person) | amber | green | green | amber | green | green | 1 / 2 | **Keep** |
| Notifications (`internal/notifications`) | amber | green | green | amber | amber | amber (#325, #328) | 1 / 3 | **Keep** |
| Push (`internal/push` + mobile `lib/push.ts`, `notification-target.ts`, `pushKinds`) | amber | green | green | amber | green | green | 0.5 / 2 | **Keep** |
| Metrics (`internal/metrics`, `queries/metrics.sql`, admin Metrics) | amber | green | amber | amber | green | amber (#331, #406) | 2.5 / 3 | **Renovate** |
| Interest tagging (`tagging`, `tagging/gemini`, `catalog/suggest.go`, mobile `suggest-activities.tsx`) | parked | | | | | | | **Park**: compiles, tests pass |

The rules applied (#413): **Keep** needs no reds and about 2 sessions or fewer to reach the bar. **Rebuild** needs red domain fit, or red tests *and* red structure. Everything else is **Renovate**, and when both routes are possible the cheaper one wins. Docs, defects and refactor load only move a part between keep and renovate. No part here has red domain fit, and none has red tests and red structure together, so no rebuild is indicated anywhere.

The cost estimates count only work towards the hygiene bar and the gaps the scope added. A frozen refactor is counted only where the gap's work depends on it, for example the blocking button on #403. Otherwise the frozen refactors are listed with their cost, for the owner to keep or close.

## Part by part

### 1. Identity: backend `internal/identity`, plus the mobile sign-in screen

This covers `GetMe` (lazy account creation, onboarded, display languages, suspended) and `DeleteAccount` (auth user first, then the app row). Sign-in is Supabase email OTP (`app/sign-in.tsx`, 141 lines). `features/auth/auth-provider.tsx` and the RootNavigator in `app/_layout.tsx` belong to the mobile shell audit (#427). They are mentioned here only where they carry this area's rules.

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | `identity/service_test.go:71` `TestMe` (email kept, onboarded, languages, suspended) and `:100` `TestDeleteAccount` (deletion order: auth first, app data kept if that fails) pin the service. No handler-level test exists for either RPC. The `ErrAccountDeleted` → `unauthenticated` mapping (`handler.go:25`) and `ErrDeletionUnavailable` → `failed_precondition` (`handler.go:50`) are untested. `GetMe` is only called through the Gate test (`moderation/suspensions_test.go`). On mobile: `sign-in-screen.test.tsx`, `sign-in-error.test.ts`, `auth-provider.test.tsx` and `sign-out-push.test.tsx`. |
| Structure | green | 160 source lines in total. Imports only `platform/auth`, `platform/errs` and `dbgen`. No dead code. |
| Domain fit | amber | The model is right: account deletion follows the legal floor (`service.go:89-104`, deleting the auth user first). But a person's standing is decided in five places with three mechanisms (#402). `GetMe` reads `IsSuspended` from the database (`service.go:70`), while `moderation.Gate` reads a 15 s cache (`moderation/suspensions.go:18`), so the two can disagree for up to 15 s on another instance. |
| Docs match | green | backend README:143-144 and architecture.md:48 match the code. |
| Defect history | green | One fix commit touched it: 3a37d0b (#111, performance; touched `identity.sql`). No bug issues. |
| Refactor load | amber | #402 (`refactor(backend)!: one Standing`, size L, breaking) targets it. |

**Cost:** renovate 0.5 (handler tests for `GetMe` and `DeleteAccount`); rebuild 2. #402, if kept, is another 1.5 to 2 sessions. The bar doesn't require it.
**Verdict: Keep.** There are no reds, and the cost is at most 2 sessions.

### 2. Profile, backend: `internal/profile`

This covers `GetProfile` and `SaveProfile` (whole-replace in one transaction), validation (age 18+, a fluent language, at least 3 activities, the conduct version) and `ConductVersion`.

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | `validate_test.go` pins every rule (`:30` `TestValidate`, `:92` `TestProtoRoundTrip`, `:152` `TestConductVersionIsOne`). `service_test.go:17` `TestIntegrationSave` covers the transaction. There is no handler-level test of `GetProfile` or `SaveProfile`: the `not_found`, `invalid_argument` and `failed_precondition` mappings at `handler.go:26-53` are untested. **The tests depend on the Tokyo pack**, which #417 removes: `service_test.go:15,62,87` and `validate_test.go:17,97,102` use `tokyo` and `tokyo-musashino`. They must move to Da Nang–Hoi An or a testfixture city when the pack goes. |
| Structure | green | 517 source lines. Imports only platform packages and `dbgen`. It is a clean handler → service → validate split. |
| Domain fit | green | It matches the decision log (age rule, whole-replace, conduct version). Re-accepting a new conduct version isn't built (#358), but that is a feature, guarded by `TestConductVersionIsOne`. |
| Docs match | amber | backend README:33, :146-147 and :337-338 match. `validate.go:20` still cites the retired milestone "(M9)". |
| Defect history | green | 94f40c4 (`fix(api)`, test data only). #276 touched `validate_test.go` (the conduct-version guard, from #188). No bug issues. |
| Refactor load | green | No frozen refactor targets it (#358 is a feature). |

**Cost:** renovate 0.5 (handler tests, moving the Tokyo fixtures, the stale comment); rebuild 2.
**Verdict: Keep.**

### 3. Profile, mobile: `app/onboarding.tsx` (96), `app/profile-edit/[step].tsx` (92), `features/profile` (1,137)

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | **red** | Only `features/profile/__tests__/draft.test.ts` (149 lines) and `save-error.test.ts` exist. `steps.tsx` (448 lines, seven steps), `onboarding.tsx`, `profile-edit/[step].tsx`, `language-picker.tsx` and `suggest-activities.tsx` have no tests. Onboarding is the first step of the core loop ("sign up"), so the screen gate in #414 applies. |
| Structure | amber | `steps.tsx` holds all seven steps in one 448-line file (`:83` Languages … `:408` Conduct), split into functions, so it can change in place. The parked suggestions are wired in at `steps.tsx:32,305` and `draft.ts:87`. |
| Domain fit | green | It follows the backend rules (`draft.ts` mirrors validation; the decision log's 2026-11-02 rule that settings save only when complete, from #133). |
| Docs match | green | mobile README:52 and :80 describe onboarding and the Profile tab's per-section editing. |
| Defect history | amber | Six fix commits since 2026-10-20: 48258d9 (#133, save from settings only when complete), 3a37d0b (#111, catalogue cache), e0ca66d (device languages), 78d0bf8, 448db9e and d069469 (`steps.tsx` layout). |
| Refactor load | green | None. #358 (re-accept the conduct) would add a screen here if kept. |

**Cost:** renovate 1.5 (render-and-interaction tests for onboarding and profile edit across the seven steps; hiding the parked suggestions); rebuild 3.5.
**Verdict: Renovate.** Tests are red, so it can't be a keep. Structure isn't red, so it isn't a rebuild, and renovating is cheaper anyway.

### 4. Moderation, backend: `internal/moderation` (826 source / 1,111 test lines), plus report recording in `plans` and `crews`

`ModerationService` serves the reports queue, flags, crew leaves, people, suspend and lift. The package also holds `Suspensions` and `Gate` (the API's suspended check) and `Remover` (the `remove_suspended` job). Reports are *recorded* outside the package: `plans/feedback.go:247` (`CheckReport`), `:262` (`Report`), `plans/events_handler.go` (`ReportPerson`) and `crews` (`ReportCrewMember`). Kept apart is the SQL view `app.kept_apart` (`db/migrations/00003_domain_rule_views.sql:40-51`).

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | green | Every `ModerationService` RPC has a handler test (`handler_test.go:18` refuses non-admins; `:32` `TestIntegrationQueue` covers reports, flags, crew leaves, people, suspend and lift). `paging_test.go` covers each list. `service_test.go:133-261` covers atomic suspend, a repeatable cleanup, lifted-before-run and resuspend. `suspensions_test.go:28,50` covers the cache and the Gate. `suspended_error_test.go` covers the error on the wire. `testfixture/rules_test.go` pins `app.kept_apart`. **Gap:** the user-side `ReportPerson` and `ReportCrewMember` handlers have no handler test. The rules are pinned at service level in `plans/feedback_test.go:145-166`. |
| Structure | amber | The package itself is tidy, but the report rules are spread out. `moderation/handler.go:58-65` re-lists the reasons as string literals mirroring `plans.Report*`. `crews` calls `plans.CheckReport`, even for leave text. There are two upserts (`UpsertReport`, `crews.sql:127` `UpsertCrewReport`). `people.go:126-132` and `handler.go:150` borrow `plans` proto mappers. `jobs.go:50-51` depends on `plans.Service` and `crews.Service`, which the remover needs. |
| Domain fit | amber | Suspended and kept apart match `GLOSSARY.md`:47-53, and the view is the single definition. The owning module is wrong (#403: kept apart is a glossary rule implemented in `plans`). Standing is spread across five places (#402). Matching calls kept-apart pairs `Blocked` (`matching/matching.go:293`), which the glossary lists under _Avoid_. That one belongs to the matching audit, but blocking will make it confusing. |
| Docs match | amber | backend README:28, :176, :203 and :322-326 are accurate and detailed. But README:169 says "(M8)" and names `Relations.Blocked`, and `moderation/handler.go:2` says "(M8)". Both cite retired milestones. |
| Defect history | amber | faf3184 (#276, closing #188: suspension checked on every chat send, plus races) and 9224ace (#134, review fixes) touched it. No open bug issues. |
| Refactor load | **red** | Three frozen tickets target it: #402 (L, breaking), #403 (M, the reports module) and #404 (L, blocking, which is built on #403). |

**Cost:** renovate 2.5 (#403 at 1, then blocking from #404 at 1.5 on top; handler tests for the two report RPCs inside that work); rebuild 5 (suspend, the remover, the queue and paging have to be re-proven).
**Verdict: Renovate.** The red is refactor load, which only moves a part between keep and renovate. Renovate is the cheaper route. Doing #403 first is recommended because it is what the blocking button builds on.

### 5. Safety and account screens, mobile: `app/report/[id].tsx` (101), You tab account section (`app/(tabs)/profile.tsx` 177), suspended screen (`app/_layout.tsx:97-110`), `features/account/account-actions.tsx` (66)

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | **red** | None of these has a test. The inventory confirms `app/report` and `features/account` are code-only. Reporting and account deletion are the legal floor (#412), and the report screen is the user end of "no safety incident goes unhandled". |
| Structure | amber | `report/[id].tsx:31-40` serves two contexts by a `crew=1` flag. It mounts all four hooks (`useFeedback`, `useCrew`, `useReport`, `useReportCrewMember`) and disables one of each pair with an empty id. It works, but blocking would be the third context. |
| Domain fit | green | Its comment (`:24-29`) states the 30-day window and kept apart, and it matches the backend. |
| Docs match | amber | The mobile README's structure table (`apps/mobile/README.md:80`) omits `app/report` (and `crew-leave`). `features/account` is described at the `src/features/account/` row. |
| Defect history | green | No fix commits since 2026-10-20. |
| Refactor load | amber | #404 adds the block action here and in chat and crew contexts. |

**Cost:** renovate 1 (render-and-interaction tests for report, account deletion and the suspended screen; README rows); rebuild 1.5.
**Verdict: Renovate.** Tests are red, structure isn't. The block button UI is counted under the gaps below.

### 6. Admin moderation pages: Reports (111), Flags (74), People (70), Person (129)

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | `Person.test.tsx` has 9 cases (suspend and lift with confirm, cancel, error, double-click). `People.test.tsx` has 4. `Reports.test.tsx` has 3, covering paging and display only: **resolving a report (`Reports.tsx:64`) is untested.** **Flags has no test** (`Flags.tsx:39` `resolveFlag`). #414 makes moderation pages a Pilot gate. |
| Structure | green | Small page files over shared `paging.tsx`. No reach-ins. |
| Domain fit | green | Matches admin README:14-17 and :22 (report decisions don't change matching; a suspension does). |
| Docs match | amber | admin README:20 still carries "Not yet checked in a browser (#233)" for Load more on Reports, Flags and People. Otherwise accurate. |
| Defect history | green | No fix commits. |
| Refactor load | green | None. |

**Cost:** renovate 1 (Reports resolve and Flags tests, the browser check, the README note); rebuild 2.
**Verdict: Keep.**

### 7. Notifications: `internal/notifications` (470 source / 288 test lines)

This is the outbox (`Enqueue` in the caller's transaction), the send pass (`Send`), receipts, token cleanup and retention. `plans` and `crews` register renderers.

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | Integration tests cover the hard parts: `notifications_test.go:101` (partial send, nobody pushed twice), `:146` (no renderer), `render_item_test.go:37` (one item never stops a batch), `retention_test.go:15`. Receipts are tested from `plans/receipts_test.go`. **Not pinned:** that every push kind's catalogue key exists. `uistrings.Catalog.Text` returns `""` for an unknown key (`uistrings/catalog.go:270-272`), so a renamed key silently sends an empty body (#325). There is little non-database unit coverage. |
| Structure | green | Imports only `push`, `dbgen` and platform. It imports neither `plans` nor `crews`, as backend README:57 says. `Send` is long (`notifications.go:201-343`) but commented. The duplicated render loop lives in `plans` and `crews` (#328), not here. |
| Domain fit | green | "Suspended people aren't sent pushes" (`GLOSSARY.md`:48) is enforced in `ListPushTokens` (backend README:203). |
| Docs match | amber | backend README:57 and :271 are accurate, but README:278 still says "Not done yet: chat, push notifications, …". |
| Defect history | amber | dcdde08 (#223: one item never stops a pass or a push batch) and faf3184 (#276) touched it. No bug issues. |
| Refactor load | amber | #325 (S, test) and #328 (M, one titled renderer). |

**Cost:** renovate 1 (#325's key-existence test and the README line; #328 is optional, another 1); rebuild 3.
**Verdict: Keep.**

### 8. Push: backend `internal/push` (229 / 130), mobile `lib/push.ts` (158), `lib/notification-target.ts`, `pushKinds` in `lib/queries.ts`

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | The Expo client is well pinned (`expo_test.go:12,54,66`, `retry_test.go:14`). Mobile is green: `push-routing.test.tsx`, `push-kinds.test.ts` (reads the backend's `Notify*` constants, so a kind with no row fails), `push-refresh.test.tsx` and `sign-out-push.test.tsx`. **`RegisterPushToken` and `UnregisterPushToken` have no behaviour test.** They are called only with an empty request in the auth and Gate tests (`platform/auth/auth_test.go:160`, `moderation/suspensions_test.go:75`). Token format, platform, moving a token to its last registrant, and unregistering someone else's token are not pinned (`push/handler.go:34-66`). |
| Structure | green | Small. The Expo sender sits behind the `Sender` interface (`expo.go:62`). |
| Domain fit | green | Per-account registration, best-effort unregister on sign-out (mobile README:188-189). |
| Docs match | amber | `lib/push.ts:15-16` and mobile README:186 list only plan and group pushes. `crews` also sends crew offered, formed and plan-suggested pushes (backend README:271). |
| Defect history | green | 8f5f7f1 (development logging only). |
| Refactor load | green | None directly. #402 would route the push sender's suspended filter through Standing. |

**Cost:** renovate 0.5 (push handler tests, the two doc lines); rebuild 2.
**Verdict: Keep.**

### 9. Metrics: `internal/metrics` (99 / 151), `db/queries/metrics.sql` (73), admin `Metrics.tsx` (182)

| Signal | Colour | Evidence |
|---|---|---|
| Test quality | amber | Backend: `metrics/handler_test.go:18` (`Monday`), `:32` (non-admin refused), `:43` `TestIntegrationMetrics`. **The admin Metrics page has no test.** It is the page the owner watches outcomes on, which #414 makes a Pilot gate. |
| Structure | green | A thin handler over two SQL queries. It loads its city by hand (`handler.go:52-62`), one of the five copies #406 targets. |
| Domain fit | amber | "Attended" comes from `app.attendance`, as the glossary says, and the north star is coherent. But the model is weekly, and **none of the Pilot's success-bar measures can be read off it** (see the gaps below). Two latent errors: `members` keeps only `t.went_ahead` groups (`metrics.sql:20`), so `groups` (`:34`) counts only groups that went ahead and has no denominator for "matched groups"; and `went_ahead` counts a group stuck `inviting` at a finished outing as having gone ahead (#331). The `reports` column (`metrics.sql:57`) joins `app.events`, so **crew reports (`event_id` null, `crews.sql:127-131`) are never counted**. |
| Docs match | amber | backend README:172 and admin README:12 match the code. `handler.go:2` and `metrics.sql:1` cite "(M8)". The inventory notes the metric definitions live only in SQL (4 decision-log rows). |
| Defect history | green | No fix commits and no bug issues. The latent errors above are new findings, not filed yet. |
| Refactor load | amber | #406 (L, `citydata`), and #331 (M, one definition of "goes ahead", which changes what `metrics.sql:20` counts). |

**Cost:** renovate 2.5 (success-bar SQL and RPC fields plus tiles, 1.5; the crew-reports fix and an admin page test, 0.5; tests on the bar's definitions, 0.5); rebuild 3 (a new success-bar page and query next to the old one, with the north star ported).
**Verdict: Renovate.** There are no reds, but the cost is above 2, so it can't be a keep. Renovate is cheaper than rebuild, though only just. If the owner judges the weekly north-star framing itself wrong for the Pilot, domain fit turns red and the rules give a rebuild of this small part at about 3 sessions.

### Parked: interest tagging

`tagging` (120), `tagging/gemini` (122), `catalog/suggest.go` (41) and mobile `features/profile/suggest-activities.tsx` (127): **`go vet` and `go test ./internal/tagging/... ./internal/catalog -run Suggest` pass.** The live eval `gemini/eval_test.go` is skipped without its environment variable. To park it, hide `<SuggestActivities>` at `features/profile/steps.tsx:305`. The RPC can stay, since without `GOOGLE_GEMINI_API_KEY` it already returns `failed_precondition` (backend README:104).

## Hygiene-bar Pilot-gate gaps in this area

**Scope gaps (#417)**

1. **The blocking button (legal floor, store requirement; #404, #336).** Nothing exists today: no RPC, no app action, no strings. `grep -i block` over proto, backend and the app finds only time-of-day blocks. #412 allows the minimum: "copy and a button on kept apart". Two routes:
   - **Minimal**, about 1 session: a `Block` RPC that writes a kept-apart pair with no moderation queue item, plus a block action on the report screen and on group and crew members. Kept apart today is a union of reports and `avoid` picks (`00003_domain_rule_views.sql:44-51`), and an `avoid` pick needs a `group_id`. So a block needs its own row source in the view, or `app.reports` with a status that skips the queue.
   - **Full #404**, about 1.5 sessions after #403: the minimal version, plus hiding the blocked person's messages from the blocker in shared rooms (history and live socket).

   Apple's App Store Review Guideline 1.2 expects user-generated-content apps to let people block abusive users. Chat is user-generated content, so hiding messages is the safer reading. Whether unblocking is needed is still open (#404).
2. **The success bar on Metrics.** None of the four measures in #412 is shown, and (3) has no data source:
   1. *share of matched groups that go ahead*: not computable. Groups that didn't go ahead are filtered out at `metrics.sql:20`, and "goes ahead" is ambiguous until #331 lands.
   2. *share of attendees who attend again during the Pilot*: `repeat_attendees` is per week (`metrics.sql:28-30`). The Pilot-wide share (people who attended twice ÷ people who attended at least once, since the Pilot started) isn't returned.
   3. *outings lost to a bug*: nothing records why a group or outing was cancelled, so this needs a field (or an admin mark) on cancellation, or a manual tally.
   4. *no safety incident unhandled*: not in #417's list for Metrics. Open reports and their age (Reports page, open oldest first) could be shown next to the bar.

   Also fix the crew-reports omission (`metrics.sql:57`).

**Tests (#414: every in-scope RPC has a handler-level test)**

3. Missing handler tests: `IdentityService.GetMe` and `DeleteAccount`, `ProfileService.GetProfile` and `SaveProfile`, `PushService.RegisterPushToken` and `UnregisterPushToken`, `EventService.ReportPerson` and `CrewService.ReportCrewMember`. The last two are in `plans` and `crews`, and would come with #403 if kept.
4. Mobile screens with no render-and-interaction test: onboarding, profile edit, report, You tab (account deletion, appearance) and the suspended screen.
5. Admin pages with no test (or no test of their action): Metrics (none), Flags (none) and Reports (resolve).
6. Profile tests use the Tokyo pack (`profile/service_test.go:15,62,87`, `validate_test.go:17,97,102`) and break when it's removed. The identity tests use `"tokyo"` as an opaque string (`identity/service_test.go:87`), which is harmless.

**Docs (#414: inventory contradictions fixed; code names match the glossary)**

7. backend README:278 ("Not done yet: … push notifications"); README:169 ("(M8)", `Relations.Blocked`).
8. mobile README:186 and `lib/push.ts:15-16` omit crew pushes; mobile README:80 omits `app/report`.
9. Retired-milestone references in code comments: `metrics/handler.go:2`, `db/queries/metrics.sql:1`, `moderation/handler.go:2` ("M8") and `profile/validate.go:20` ("M9").
10. Glossary drift: kept-apart pairs are called `Blocked` in matching (`matching/matching.go:293`, `assemble.go:394`, `crews.go:38,59`), and the glossary says to avoid "blocked". This matters more once a user-facing "block" exists. It belongs to the matching audit (#422), and blocking needs a glossary entry either way (the soft-launch store items).
11. admin README:20's unverified Load-more note (Reports, Flags, People).

**Code**

12. Dead code: `deadcode ./...` is clean for these Go packages. `knip` for the mobile and admin files was not run here.
13. #325: a missing push key sends an empty body silently (`uistrings/catalog.go:270-272`).

## Frozen tickets that target this area

| Ticket | Size | Target | Needed for the bar? |
|---|---|---|---|
| #402 one Standing (breaking) | L | identity, moderation Gate, chatws, push filter, mobile navigator | No. It closes the GetMe-vs-Gate disagreement and leaves room for waiting and needs-conduct (soft launch). |
| #403 a reports module | M | moderation, plans, crews | Yes, if blocking is built as #404 describes. |
| #404 block someone | L | moderation, chat, mobile | Yes (the scope gap). It can be cut down to the minimal route above. |
| #325 push keys exist | S | notifications | Recommended: it is cheap and guards a silent failure. |
| #328 one titled renderer | M | notifications, plans, crews | No. |
| #331 one "goes ahead" | M | attendance view, metrics | Yes, for success-bar measure (1). The plans audit (#421) owns the call. |
| #406 citydata | L | metrics, plans, crews, sim | No. |
| #358 re-accept the conduct | feature | profile (backend and mobile) | No, until the conduct text changes. |
