# Audit: crews and chat

Research for [#424](https://github.com/shrujancheruku/issho/issues/424), a child of [#411](https://github.com/shrujancheruku/issho/issues/411). Read at `origin/main` 139d77e (2026-11-06; CI green on that commit). Facts and recommendations only: the owner settles each verdict in the verdict ADR.

**Measured against:** Verdict criteria ([#413](https://github.com/shrujancheruku/issho/issues/413)), The hygiene bar ([#414](https://github.com/shrujancheruku/issho/issues/414)), The Pilot's scope ([#417](https://github.com/shrujancheruku/issho/issues/417)), What the Pilot is ([#412](https://github.com/shrujancheruku/issho/issues/412) and its amendment), the Feature and knowledge inventory (branch `research/feature-and-knowledge-inventory`), `GLOSSARY.md`.

## Summary

| Part (sub-part) | Tests | Structure | Domain fit | Docs | Defects | Refactor load | Renovate | Rebuild | Recommended verdict |
|---|---|---|---|---|---|---|---|---|---|
| Crews backend (`internal/crews`, `matching/crews.go`, `crews.sql`) | green | amber | amber | amber | amber | amber | ~2 (bar + owner proposes); ~3.5 with #403/#405/#406/#328 | ~4.5 | **Keep** (borderline; Renovate if the frozen refactors are taken in) |
| Chat backend (`internal/chat`) | green | green | green | amber | green | green | ~0.5 | ~2 | **Keep** |
| Chat socket (`internal/chatws`) | green | green | green | green | amber | amber | ~0.25 | ~2 | **Keep** |
| Mobile crew screens (`app/crew`, `app/crew-leave`, `features/crews`) | amber | amber | amber | amber | amber | green | ~1 | ~2 | **Keep** |
| Mobile chat (`app/chat`, `features/chat`) | green | green | green | green | amber | green | ~0.5 | ~2 | **Keep** |
| Together tab (`app/(tabs)/index.tsx`, `features/together`) | amber | amber | green | green | green | green | ~0.75 | ~1.5 | **Keep** |
| Admin CrewLeaves (page and `ListCrewLeaves`) | amber | green | green | green | green | green | ~0.25 | ~0.5 | **Keep** |
| *New:* admin crew candidates (owner proposes) | – | – | – | – | – | – | build ~0.75 | – | **Build** (scope addition, not a verdict) |

Costs are agent sessions to reach the hygiene bar. Nothing in this area is out of the Pilot's scope, so nothing is parked or removed. No part has a red signal, so no rule forces a rebuild anywhere.

## Scope check

[#417](https://github.com/shrujancheruku/issho/issues/417) keeps all of it in: crews (with one change, below), crew rooms, outing rooms, the room lock and retention, the CrewLeaves admin page. The Together tab is the home of the core loop (invites, outing chats). No cut forms apply.

**The owner-proposes change.** Today the worker offers crews by itself: `crews.Runner.Pass` ends with `r.offer(...)` (`backend/internal/crews/pass.go:126`), which runs `matching.FindCrews`, skips covered or overlapping sets, creates the crew and pushes `crew_offered` (`pass.go:147-240`). Under the scope the server only lists candidates and the owner sends the offer. What has to change:

1. **Backend.** Take the offer stage out of `Pass` (it keeps deciding offers and the weekly suggestion, both still automatic). Move the candidate search (`pass.go:153-205`: met pairs, aversions, `plans.LoadCity`, `FindCrews`, `CrewCoversMembers`/`CrewOverlaps`) into an admin read, e.g. `ListCrewCandidates(city)` returning each candidate's members (names), basis and why it qualifies. Add an admin write, e.g. `ProposeCrew(city, members, basis)`, that re-checks the members server-side (at least `matching.CrewMinSize` = 3, all in the city, none suspended, no kept-apart or averse pair, no covering crew) and then does what `pass.go:209-222` does in one transaction. Both RPCs get handler-level tests (a Pilot gate).
2. **Tests.** `TestIntegrationCrews` (`crews/crews_test.go:89`), `TestIntegrationUnansweredLapse` (`lapse_test.go:19`), `TestIntegrationListCap` and `resilience_test.go` create offers through `Pass` or set them up for it; they move to `ProposeCrew` (or a fixture). `FindCrews`' own tests (`matching/crews_test.go`) are unaffected.
3. **Admin SPA.** A crew-candidates page (pick a city, see candidates, adjust members, send). New, so counted as a build, not a renovation.
4. **Docs and words.** `crews.go:1-5` ("the worker offers a crew"), `docs/architecture.md:7` and `:76` ("the weekly pass that offers them"), backend README worker step **0b** (`backend/README.md:274`), and `GLOSSARY.md` **Crew** ("offered to people who got on well") should say who offers. The app's offer copy ("You got on well. Keep meeting as a crew?", `apps/mobile/src/i18n/en.json:66`) still reads correctly.

Estimate: about 1 session for (1), (2) and (4); about 0.75 for (3). The domain model (candidates, offer, answers, quorum of 3, lapse, active, closed) is unchanged; only the trigger moves, so this does not make domain fit red.

## Signal scale used

#413 names the signals but not their thresholds. This audit uses:

- **Test quality**: green = the behaviour is pinned (rules and integration paths) well enough to act as a spec; amber = rules pinned, but screens or entry points untested; red = behaviour mostly unpinned.
- **Structure**: green = clear boundary, no reach-ins; amber = changeable in place but with reach-ins or large mixed files; red = can't be changed in place.
- **Domain fit**: amber = names or copy drift from the glossary or settled decisions; red = the model itself is wrong.
- **Defect history** (`git log --since=2026-10-20 --grep='^fix'` on the part's paths, plus bug issues): green = at most one fix; amber = two or more fixes, or an open bug; red = repeated user-visible defects in the same behaviour.
- **Refactor load** (open, frozen `ready-for-agent` refactors that target it): green = none; amber = some, each doable in place; red = enough that the part's shape changes.

## Crews backend

`backend/internal/crews` (1,013 lines of source, 773 of tests), `backend/internal/matching/crews.go` (264 / 115), `backend/db/queries/crews.sql` (170). RPCs: `CrewService` `ListMyCrews`, `GetCrew`, `RespondToCrew`, `LeaveCrew`, `ReportCrewMember` (`proto/issho/v1/crews.proto:25-40`). Worker job `crews_city` (`internal/worker/worker.go:180-214`).

1. **Test quality: green.**
   - Every rule about one person's place is pure and table-tested: `Membership.Visible`, `Respond`, `Chat`, `Leave`, `Remove` (`crews/membership.go:20-71`; `membership_test.go:23-92`).
   - Integration tests pin the lifecycle: offer to the right people only, no second offer, form at 3, suggestion a day later, report, leave with reasons, close at 1 (`crews_test.go:89`); suspension removal (`:260`); city-timezone seasons (`:324`); unanswered members lapse (`lapse_test.go:19`); the list cap (`cap_test.go:18`); one failing crew doesn't stop the pass, setup failure retries (`resilience_test.go:53`, `:120`); a gone crew's push is skipped (`render_skip_test.go:18`).
   - `FindCrews` has scenario tests (`matching/crews_test.go`; listed in `backend/README.md:364`).
   - Gap: no handler-level test for the five `CrewService` RPCs (`crews/handler.go:33-119`); only `crewToProto` and `StatusProto` are tested (`enums_test.go:11`, `:19`). That is a Pilot gate (below), not a weakness of the spec.
2. **Structure and coupling: amber.**
   - Good seam: `Membership` holds the rules; the service only loads, locks and stores (`membership.go:5-8`). Chat access is a plug-in (`crews/chat.go:17-32`).
   - Reaches into `plans` for things that aren't plans: `plans.Invalid` and `plans.CheckReport` (also used for leave text, `crews.go:226`, `:229`), `plans.ReportReasons` and `plans.InvalidError` (`handler.go:111`, `:134`), and `plans.LoadCity` plus its own `GetCityMatching`/`LoadLocation` (`pass.go:169-179`).
   - The notification renderer queries through `r.Plans.DB` and uses `r.Plans.Translations` (`crews/notifications.go:57`, `:85`, `:120`), against `docs/architecture.md:86` ("domains talk to each other only through service types").
   - The pass hand-rolls the worker failure policy with its own `itemFailed`/`allFailed` and three different "all failed" counts (`pass.go:78`, `:122`, `:236`).
   - None of this stops change in place; all of it is already ticketed (refactor load).
3. **Domain fit: amber.**
   - Matches the glossary: **Crew**, **Room** (crew rooms), quorum 3 (`matching/crews.go:11`), leaving privately with a reason, **Kept apart** feeding `Relations.Blocked`.
   - "Offered by the server" no longer matches the scope (see the owner-proposes change): `crews.go:1-5`, `pass.go:34-35`.
   - **Lapsed** is used for two things. The glossary defines it as "a crew offer that someone never answered before it was decided" (the member's answer: `Unanswered = "lapsed"`, `crews.go:40`), but the crew status `Lapsed` is "too few accepted in time" (`crews.go:29`; `crews.sql:40`). Either the glossary gains the crew sense or the status is renamed (breaking changes are free before the Pilot).
   - Tokyo example in a doc comment: "Kichijoji crew" (`matching/crews.go:24`). Minor, but the Tokyo pack is being removed.
4. **Docs match: amber.**
   - The backend README rows for the five RPCs (`backend/README.md:164-168`) and worker step 0b (`:274`) match the code today.
   - `backend/README.md:278` says "Not done yet: chat, push notifications…", which the code contradicts (also flagged by the inventory).
   - The mobile README's push section lists plan and group pushes only, but `crews.NotificationRenderer` sends three crew kinds (`crews/notifications.go:22-26`) (inventory §10).
   - After the owner-proposes change, `architecture.md:7`, `:76`, README `:274` and GLOSSARY **Crew** need updating.
5. **Defect history: amber.** Three fix commits since 2026-10-20, all found by reviews, not by users, and each shipped with tests:
   - 9224ace `fix: shutdown drain, crews_city retries…` (#134): `pass.go`, `crews_test.go`.
   - dcdde08 `fix(backend): one item never stops a pass or a push batch` (#223): `pass.go`, `notifications.go`.
   - faf3184 `fix(backend): close withdraw and chat-send races…` (#276): `crews.go`.
   - Open bug #321: `ListMetForPicks` uses its own "met" rule, so picks and the crews "met" rule can disagree, plus a stale `crews.sql` comment.
6. **Pending refactor load: amber.** Frozen tickets that target it:
   - #403 a reports module (moves `CheckReport`/`ReportReasons`/`UpsertCrewReport` out of `plans`/`crews`).
   - #405 one stage runner for the pass failure policy (`pass.go:134`).
   - #406 `internal/citydata` (`pass.go:169-179`).
   - #328 one titled renderer for plans and crews (`crews/notifications.go`).
   - #325 a test that every push kind's catalogue key exists (crews' 3 kinds).
   - #321, above.
   - Frozen features on crews, not refactors: #356 join offers, #398 who in the crew signed up, #399 past outings together.

**Cost to the bar.**
- *Renovate/keep*: about 2 sessions. That covers the owner-proposes backend change and moved tests (~1), handler tests for the five RPCs (~0.5), the Lapsed naming and doc/glossary fixes (~0.25) and #321 (~0.25). Taking #403, #405, #406 and #328 in as well adds about 1.5, so ~3.5. Those tickets are shared with `plans` and mostly priced in its audit.
- *Rebuild*: about 4.5. That means re-writing the state machine, pass, chat access and notifications, re-pinning the integration tests, plus the owner-proposes change. `FindCrews` would likely be kept as is.

**Rule applied.** There are no reds, and the cost to the bar is about 2 sessions or fewer, so the result is **Keep**. Rebuild doesn't apply: domain fit is amber, and tests and structure are not both red. The cost check also favours staying in place (2 against 4.5). It is at the Keep threshold: if the owner adopts the frozen refactors that touch crews, the same rules give **Renovate** (~3.5, still cheaper than a rebuild).

## Chat backend (`internal/chat`)

482 lines of source, 549 of tests. RPCs `ChatService` `ListMessages`, `SendMessage` (`proto/issho/v1/chat.proto:28-32`). Retention: `DeleteOld`, `DeleteOldCrewChats` (`chat/chat.go:333-354`).

1. **Test quality: green.**
   - Both room kinds are tested through a fake `Access`: reading, writing, refusing, trimmed and limited messages, the cursor, and closed rooms that stay readable (`chat_test.go:120`). Draining (`:199`).
   - Posts wait for the room lock (`:254`). Access is re-checked under the lock (`:352`). Concurrent posts are all read (`:391`). NOTIFY is sent (`:448`).
   - Crew chat retention (`retention_test.go:15`). Event-room access is in `plans/chat_test.go` (`backend/README.md:369`).
   - Gap: no handler-level test for `ListMessages`/`SendMessage` (`chat/handler.go:30`, `:52`), which is a Pilot gate.
2. **Structure: green.** One module for both kinds. It imports neither `plans` nor `crews`; owners plug in an `Access` (`chat.go:1-5`, `:89-91`), and `cmd/api` wires them (`cmd/api/main.go:195`). The lock-ordering reason is commented (`chat.go:276-280`).
3. **Domain fit: green.** **Room** = an outing's or a crew's chat (GLOSSARY). `EventRoom` is allowed: the glossary says code and API call an outing an event. Writing windows (48 h before, 24 h after) and the 30-day retention match the decision log and the backend README (`chat.go:30-38`; `backend/README.md:160`, `:264`).
4. **Docs match: amber.** The README rows (`backend/README.md:59`, `:159-160`) match. `backend/README.md:278` ("Not done yet: chat…") contradicts the code and is one of the inventory's Pilot-gate doc fixes.
5. **Defect history: green.** One fix, faf3184 (#276: the chat-send race, suspension re-checked per send), shipped with `TestIntegrationSendChecksAccessUnderTheLock`. No open bugs.
6. **Refactor load: green.** No frozen refactor targets `internal/chat`. #396 (unread count and last message per room) is a frozen feature.

**Cost.** Renovate/keep about 0.5 (two handler tests, the README line, a dead-code pass); rebuild about 2. **Rule applied:** there are no reds and the cost is ≤ 2, so **Keep**.

## Chat socket (`internal/chatws`)

468 lines of source, 536 of tests. `GET /ws/chat`.

1. **Test quality: green.** The socket end to end (`chatws_test.go:35`), drain and revoke (`drain_test.go:32`), a suspension that ends the socket on the next send (`suspend_test.go:31`). The protocol is written out in the package doc (`chatws.go:1-43`).
2. **Structure: green.** One `Hub` listening per process (hijacked connection, reasons commented, `chatws.go:154-165`). The handler depends on `chat.Service` and an injected `Suspended` func (`chatws.go:198-201`; wired at `cmd/api/main.go:230-231`). One file, but a cohesive one.
3. **Domain fit: green.** It relays rooms; there are no domain rules of its own beyond access errors mapped to close codes (`chatws.go:30-35`).
4. **Docs match: green.** `backend/README.md:55`, `:177` agree with the package doc and the code.
5. **Defect history: amber.** Two fixes: 9224ace (#134, the shutdown drain) and faf3184 (#276, the suspension checked per send). Both were review-found, with tests (`drain_test.go`, `suspend_test.go`).
6. **Refactor load: amber.** #402 (one Standing in identity) replaces the injected `Suspended` func (`main.go:231`) with a Standing check. It is breaking and L-sized, but the chatws share is small.

**Cost.** Renovate/keep about 0.25 (dead-code pass; #402's chatws share is priced with identity); rebuild about 2. **Rule applied:** there are no reds and the cost is ≤ 2, so **Keep**.

## Mobile crew screens

`apps/mobile/src/app/crew/[id].tsx` (428, untested), `app/crew-leave/[id].tsx` (71, untested), `src/features/crews` (104 lines of source, 197 of tests: `crew-view.ts`, `data.ts`, `name.ts`).

1. **Test quality: amber.** The crew rules are pure and pinned (`features/crews/__tests__/crew-view.test.ts`). The offer-to-active transitions (`data.test.tsx`, 129 lines, from #296) and the name (`name.test.ts`) are pinned too. Neither route screen has a render or interaction test (inventory §10: "Untested routes include `crew/[id]` 428").
2. **Structure: amber.** `crew/[id].tsx` holds five components (`CrewScreen`, `People`, `Person`, `Offer`, `Active`, `:39-384`) and their animation in the route file, where the mobile README says `src/app/` is "Routes only" (`apps/mobile/README.md:80`). The rules are read from `crewView()` rather than re-derived (`:49`, `:58`, `:75`, `:158`), so it changes in place easily.
3. **Domain fit: amber.** The copy and a comment assume crews of four: "a group chat for the four of you" (`src/i18n/en.json:63`), "suits the four of you" (`:61`), "The four of you on a thread" (`crew/[id].tsx:70`). But `FindCrews` offers any clique of 3 or more (`matching/crews.go:11`, `:36-51`), and under owner proposals the owner picks the size. Otherwise it matches: quorum from the API, answer changeable until the deadline, leaving with a private reason.
4. **Docs match: amber.** `features/crews` is described (`apps/mobile/README.md:86`, `:161`). `crew-leave/[id]` is missing from the structure table, and the push section omits crew pushes (inventory §10).
5. **Defect history: amber.** Fixes touching it: 3a37d0b (#111, crash and load fixes, `crew/[id].tsx`), 9224ace (#134, `name.ts`), c2653bf (#48, naming the plan on suggestions). There are no open bugs on these screens; #323 is in `features/chat`.
6. **Refactor load: green.** No frozen refactor targets it. Frozen features #398 and #399 would add to the crew screen.

**Cost.** Renovate/keep about 1 session: render-and-interaction tests for the offer and active screens and crew-leave (~0.5), the "four of you" copy in English and Vietnamese (~0.1), moving the components into `features/crews` (~0.25), and the README rows (~0.1). Rebuild about 2. **Rule applied:** there are no reds and the cost is ≤ 2, so **Keep**.

## Mobile chat

`apps/mobile/src/app/chat/[id].tsx` (40, untested) and `src/features/chat`: `room.ts` 314, `chat-panel.tsx` 212, `messages.ts` 77, `typing-dots.tsx` 69, with 618 lines of tests.

1. **Test quality: green.** `room.test.tsx` (381) pins the shared socket per room, the cache, reconnects and typing. `chat-panel.test.tsx` (191) and `messages.test.ts` (46) cover the rest. Only the 40-line route and the typing dots are untested.
2. **Structure: green.** One hook for both room kinds (`useRoom(room)`), one socket per room shared by the crew screen's chat and plans tabs (`crew/[id].tsx:285-286`). Documented at `apps/mobile/README.md:89`.
3. **Domain fit: green.** Rooms, not channels or threads. Suggested plans are messages with a `planId`, as the backend sends them.
4. **Docs match: green.** The mobile README row matches.
5. **Defect history: amber.** One fix, 2aa1d13 (#273, keep loaded data when a refetch fails), plus an open bug, **#323**: a closed plan suggested in a crew chat still shows "Signed up" and a past "sign up by" date (`features/chat/chat-panel.tsx:37-63` never reads `closed_reason`). This is user-visible in a Pilot feature.
6. **Refactor load: green.** None. #396 (unread count and last message) is a frozen feature.

**Cost.** Renovate/keep about 0.5 (#323 ~0.25, a render test for `chat/[id]` ~0.1, dead-code pass); rebuild about 2. **Rule applied:** there are no reds and the cost is ≤ 2, so **Keep** (fix #323 before the Pilot).

## Together tab

`apps/mobile/src/app/(tabs)/index.tsx` (362, untested) and `src/features/together` (`sort.ts` 56 with `sort.test.ts` 79; `invite-card.tsx` 72 and `data.ts` 23 untested).

1. **Test quality: amber.** The section order (needs you, groups, waiting) is pinned as a pure function (`sort.test.ts`). The screen, the first-week state and the invite card have no render or interaction test. Only the Plans tab has one (`plans-tab.test`, inventory §3).
2. **Structure: amber.** The route file holds seven components (`ApiStatus`, `TogetherScreen`, `RowText`, `NeedsYouRow`, `CrewRow`, `OutingRow`, `WaitingRow`, `FirstWeek`: `index.tsx:30-259`), against "Routes only" (`apps/mobile/README.md:80`). That includes a development-only API status aid (`:29-42`). The data and sort live in `features/together`.
3. **Domain fit: green.** It reads `outing()` and `crewView()` flags rather than re-deciding (`features/together/sort.ts`). Copy uses outing, plan and crew.
4. **Docs match: green.** `apps/mobile/README.md:80`, `:87` describe it.
5. **Defect history: green.** One fix, 3a37d0b (#111, `invite-card.tsx`). No open bugs.
6. **Refactor load: green.** None. #396 (unread counts on rows) and #400 (check on phones) are frozen features or chores.

**Cost.** Renovate/keep about 0.75 (a render-and-interaction test for the three states ~0.5, moving the rows into `features/together` ~0.25); rebuild about 1.5. **Rule applied:** there are no reds and the cost is ≤ 2, so **Keep**.

## Admin CrewLeaves

`apps/admin/src/pages/CrewLeaves.tsx` (50, untested), served by `ModerationService.ListCrewLeaves` (`backend/internal/moderation/handler.go:262-310`).

1. **Test quality: amber.** The RPC is tested at handler level: crew leaves by reason with the crew's name (`moderation/handler_test.go`, `backend/README.md:326`), and paging (`moderation/paging_test.go`). The page itself has no test (inventory §10: "CrewLeaves 50").
2. **Structure: green.** A paged list with a reason filter, using the shared `usePaged`/`LoadMore` (`CrewLeaves.tsx:6`, `:12`).
3. **Domain fit: green.** "Why people left crews; the crew isn't told", with "made me uncomfortable" highlighted (`CrewLeaves.tsx:23-24`, `:35`). This matches `crews.Leave` (`crews.go:220-221`).
4. **Docs match: green.** `apps/admin/README.md:16`.
5. **Defect history: green.** None.
6. **Refactor load: green.** None.

**Cost.** Keep about 0.25 (a page test); rebuild about 0.5. **Rule applied:** there are no reds and the cost is ≤ 2, so **Keep**.

## Hygiene-bar Pilot-gate gaps in this area

1. **Handler-level tests missing for in-scope RPCs** (gate: "every in-scope RPC has a handler-level test"):
   - `CrewService`: `ListMyCrews`, `GetCrew`, `RespondToCrew`, `LeaveCrew`, `ReportCrewMember` (`crews/handler.go:33-119`).
   - `ChatService`: `ListMessages`, `SendMessage` (`chat/handler.go:30-62`).
   - The new admin `ListCrewCandidates`/`ProposeCrew` need them from the start.
   - The services under them are well tested; the gap is the handler layer (auth, id parsing, error-kind mapping, proto shaping).
2. **Core-loop mobile screens without a render-and-interaction test:**
   - The Together tab (`app/(tabs)/index.tsx`) and the outing chat route (`app/chat/[id].tsx`).
   - The crew screen and crew-leave (`app/crew/[id].tsx`, `app/crew-leave/[id].tsx`) if the owner counts crews as part of the loop. They feed the "attend again" success bar, so this audit recommends counting them.
3. **Admin pages the owner runs the Pilot with:** CrewLeaves has no page test. It's a moderation input ("may need a look at the crew's reports"), so it arguably falls under the moderation gate. The new crew-candidates page will be one the owner runs the Pilot with, so it needs a test.
4. **Docs contradictions from the inventory, in this area:**
   - `backend/README.md:278` ("Not done yet: chat, push notifications…").
   - The mobile README's push section missing crew pushes.
   - `crew-leave/[id]` missing from the mobile README's structure table.
5. **Code names match glossary terms:**
   - **Lapsed** has two senses in code (crew status `crews.go:29` against member response `:40`) and one in the glossary.
   - The crew docs and glossary say the server offers crews, which stops being true with the owner-proposes change.
6. **Dead-code check:** not run here (neither `deadcode` nor `knip` is installed locally, and the CI job doesn't exist yet). Nothing obviously dead was found reading these packages; `ListCrewOffersToDecide` and the decide stage stay in use after the offer stage moves.
7. **User-visible bug in a Pilot feature:** #323 (a closed suggested plan says "Signed up"). It isn't a hygiene-bar item, but the success bar counts "no outing fails because of a bug", and this misleads people about a crew's second outing.
8. **Copy:** "the four of you" (`en.json:61`, `:63`) is wrong for crews of 3 or 5+, and its Vietnamese strings need the same fix.

## Parked and removed

None in this area.
