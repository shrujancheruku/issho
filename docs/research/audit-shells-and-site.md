# Audit: mobile shell, admin shell and site

Research for #427, a child of the wayfinder map #411. It scores each part against Verdict criteria (#413) and The hygiene bar (#414), and judges only what The Pilot's scope (#417) keeps in. These are facts and recommendations only. The owner settles each verdict in the verdict ADR (#429).

**Method.** Code read at `origin/main` 48af2e8 (2026-11-06). Sources:

- the code itself, with `file:line` evidence;
- `git log origin/main --since=2026-10-20 -i --grep=^fix -- <paths>` for defect history;
- `gh issue list` for bug issues and frozen tickets;
- the Feature and knowledge inventory (`origin/research/feature-and-knowledge-inventory:docs/research/feature-and-knowledge-inventory.md`, "the inventory").

I also ran three commands in a fresh install (`pnpm install --frozen-lockfile`): `jest` in `apps/mobile` (48 suites, 320 tests, all pass), `vitest run` and `tsc --noEmit` in `apps/admin` (12 files, 114 tests, all pass; typecheck clean), and `npx knip@5` from the repo root with no config.

**Scope check.** Every part in #427 is in scope under #417: the web build (as the fallback), EAS builds and expo-updates channels, Sentry, dark mode, the admin pages the owner runs the Pilot with, and the site (privacy policy, terms, deletion page, and the waitlist signup). Nothing here is parked or removed, but two things the shells carry are parked:

- the admin **Translations** page and translator role;
- the admin **Cities**/**CityEditor** pages.

Those pages belong to #425. Their footprint in the shell is noted below. The mobile i18n module (`src/i18n`) is also left to #425 (UI strings). #427 doesn't list it among this audit's parts.

## Summary

| Part | Tests | Structure | Domain fit | Docs | Defects | Refactor load | Renovate / rebuild (sessions) | Recommended verdict |
|---|---|---|---|---|---|---|---|---|
| Mobile shell | amber | green | green | amber | green | green | ~1.5 / ~8 | **Keep** |
| Admin SPA shell | green | amber | green | amber | green | green | ~1 / ~3 | **Keep** |
| Site (`site/`) | red | amber | amber | green | amber | green | ~2.5 / ~3.5 (site only) | **Renovate** |

How the rules apply (#413):

- **Mobile shell and admin shell:** no red signal, and reaching the bar costs about 2 sessions or fewer, so **Keep**.
- **Site:** it has a red (tests), so it can't be kept. Rebuild needs domain fit red, or tests *and* structure red. Domain fit and structure are both amber, so rebuild isn't open, which leaves **Renovate**. The cost check doesn't apply, because only one of renovate and rebuild is open. Even if it did, renovating is cheaper.

No sub-part split is recommended. The mobile build config (`app.json`, `app.config.js`, `eas.json`, `wrangler.jsonc`, `public/_headers`) sits on a file boundary and could be split, but it would get the same verdict (Keep), so splitting gains nothing.

---

## 1. Mobile shell (`apps/mobile`)

**What's in the part:**

- **Navigation and tabs:** `src/app/_layout.tsx` (166 lines), `src/app/(tabs)/_layout.tsx` (30), `components/app-tabs.tsx` (44) and `app-tabs.web.tsx` (85).
- **Auth provider:** `features/auth/auth-provider.tsx` (132), `lib/supabase.ts`, `lib/api.ts`, `lib/secure-storage(.web).ts` and `app/sign-in.tsx` (141).
- **Theme and components:** `constants/theme.ts` (229), `constants/motion.ts` (39) and 26 files in `components/` (1,881 source lines).
- **Hooks:** the 6 files in `src/hooks` (117).
- **Query keys:** `lib/queries.ts` (113) and `lib/app-focus.ts`.
- **Appearance and account actions:** `features/appearance` and `features/account`.
- **Monitoring:** `lib/monitoring.ts`.
- **The web build:** 13 `*.web.ts(x)` variants, `wrangler.jsonc` and `public/_headers`.
- **EAS and expo-updates:** `app.json`, `app.config.js` and `eas.json`.

### Signals

**1. Test quality: amber.**

What is pinned:

- the auth provider's cache clearing on a user switch (`features/auth/__tests__/auth-provider.test.tsx`);
- the sign-in screen (`sign-in-screen.test.tsx`, 153 lines);
- push release on sign-out (`sign-out-push.test.tsx`);
- the invalidators (`lib/__tests__/queries.test.ts`) and focus refetch (`app-focus.test.ts`);
- session storage (`secure-storage.test.ts`);
- push routing, kinds and refresh, including tests that read the Go `Notify*` constants;
- the loading ladder and frame (`components/__tests__/query-screen.test.tsx`, `screen-frame.test.tsx`, `use-leave.test.tsx`);
- the theme and sentence lint bans (`lib/__tests__/theme-lint.test.ts`, `glued-sentences-lint.test.ts`).

Not pinned:

- the root navigator's guards (`src/app/_layout.tsx:96-157`): the suspended screen at `:103-111`, the account-loading and account-error screen at `:114-129`, and the three `Stack.Protected` groups at `:133-154`;
- the tab layout's push registration and tap wiring (`src/app/(tabs)/_layout.tsx:13-28`). Only the pure `notificationTarget` is tested;
- the tab badge (`components/app-tabs.tsx:16,30`);
- account deletion, which is part of the legal floor (`features/account/account-actions.tsx:15-33`);
- the appearance preference (`features/appearance/*`);
- every hook in `src/hooks`;
- 23 of the 26 components, among them `thread.tsx` (185), `map-sheet.tsx` (142) and `availability-grid.tsx` (87).

The logic users notice first is pinned. The routing that decides which screen anyone sees is not, which makes this amber rather than red.

**2. Structure and coupling: green.**

- Folders follow the rule in `apps/mobile/AGENTS.md`: routes only in `src/app`.
- Query keys exist only in `lib/queries.ts:15-40`. A search for inline `queryKey: [` or `invalidateQueries` outside `queries.ts` finds nothing in non-test code.
- The largest component is 185 lines.
- Two small reaches go from a shared layer into a feature:
  - `components/app-tabs.tsx:4` (and `.web.tsx:8`) imports `features/together/data` for the badge count;
  - `hooks/use-color-scheme.ts:3` (and `.web.ts:4`) imports `features/appearance/preference`.

  Both are shell-owned concerns (the badge and the theme choice), so they don't hinder change in place.
- Knip found no real dead files in the shell. Its unused-files findings are false positives, as the knip results below explain.

**3. Domain fit: green.**

- The tabs (Together, Plans, You) follow the settled redesign (decision log 2026-10-26; `app-tabs.tsx:8-12`).
- The guards follow the identity rules: suspension (`_layout.tsx:101-111`; GLOSSARY "Suspended") and deletion that doesn't depend on a moderator.
- Typography already handles the Pilot's Vietnamese. The body face (Zen Kaku Gothic New) lacks ă, đ, ơ, ư and every tone-marked vowel: I checked its cmap with fontTools, and 49 of the sampled Vietnamese letters are missing. So `vi` is left out of `BodyFontLanguages` (`constants/theme.ts:126-127`), and Vietnamese body text is set in the system font for the whole string, by design. `vi` is in `DisplayFontLanguages` (`:129-131`, Dela Gothic One covers it) and in `TallScriptLanguages` (`:139-141`).
- The Vietnamese screenshot check is still open in #352.

**4. Docs match: amber.**

The README covers the shell well:

- auth (`apps/mobile/README.md:54-75`);
- the structure table, including the frame and loading-ladder rules and the hooks (`:80`, `:92-93`);
- query keys (`:91`, `:142`);
- the web build (`:167-182`), OTA updates (`:238`) and Sentry (`:242`).

Contradictions, also listed in the inventory's "Docs that contradict the code":

- `README.md:213`: "iOS: deferred to M9". `:230`: "waits for M9". `:234`: "M9 is planned to use EAS Build/Submit". M9 closed on 2026-10-26, and iOS through TestFlight is a Pilot platform (#412).
- `app.config.js:21`: "iOS waits for M9", the same stale milestone, in a code comment.
- The structure table (`README.md:80`) omits the `report/[id]` and `crew-leave/[id]` routes, which are registered at `_layout.tsx:151,153`.

**5. Defect history: green.**

Five `fix(...)` commits touched shell paths since 2026-10-20, and no open bug issue targets the shell:

- b807646 (2026-10-20): the sign-in code field kept above the keyboard;
- 78d0bf8 (2026-10-21): chip labels wrap;
- 6ed1ae2 (#37, 2026-10-22): Google attribution;
- 3a37d0b (#111, 2026-10-26): map crashes, load speed, tracing;
- 2aa1d13 (#273, 2026-11-04): keep loaded data when a refetch fails, with tests.

For the build config, d16d642 (#115, 2026-10-26) unblocked Android builds. Most are UI polish, and the one behavioural fix (#273) shipped with a test.

**6. Pending refactor load: green.** No frozen *refactor* ticket targets the shell:

- #330 and #327 target the outings and plans slices.

Frozen or human tickets that touch it, which are work but not refactors:

- #355 (first-run tour): a tour provider and an overlay above the native tab bar. In scope and a Pilot gate, ordered late.
- #340 (Web for launch): the web session-security decision.
- #306 (type scale), #400 (check the redesign on phones), #316 (rebuild dev clients) and #352 (fonts and right-to-left on phones). All four are for the owner.

### Cost to reach the hygiene bar

**Keep, with small fixes: about 1.5 sessions.**

- About 1 session for render-and-interaction tests:
  - the root navigator's three guards and the suspended and account-error screens;
  - account deletion (confirm, then the API call, then sign-out; and a failure);
  - the tab badge;
  - the tab layout's tap routing.
- About 0.25 sessions to fix the M9 lines in the README and `app.config.js` and add the two missing routes.
- About 0.25 sessions for the shell's share of a knip config (Expo platform extensions; see below).

**Rebuild: about 8 sessions.** Providers, theme, 26 components, 13 web variants and build config. Every screen in every slice imports it, so a switch-over touches all of them.

### Verdict: Keep

No red signal, and the cost to the bar is about 1.5 sessions, so the Keep rule holds (no reds, about 2 sessions or fewer).

---

## 2. Admin SPA shell (`apps/admin`)

**What's in the part:**

- `src/App.tsx` (145 lines): the hash router and the console;
- `Confirm.tsx` (36);
- `format.ts` (169);
- `lib.ts` (22) and `main.tsx` (25);
- `SignIn.tsx` (63);
- `paging.tsx` (38) and `city.ts` (17);
- `styles.css`, `public/_headers`, `vite.config.ts` and `wrangler.jsonc`;
- the test kit in `src/test/` (`fakeApi.ts`, `render.tsx`, `deferred.ts`).

Pages are judged in their product slices.

### Signals

**1. Test quality: green.**

- `App.test.tsx` (126 lines, 6 cases, some table-driven) covers every route and the id forms. It also covers malformed and prototype-key hashes falling back to Reports, `hashchange`, a deep link kept through sign-in, the translator-only view and the no-access screen.
- `Confirm.test.tsx` covers fold-back, `keepOpen` and focus movement.
- `format.test.ts` has 7 cases.
- Paging is exercised through `pages/Reports.test.tsx`.
- The fake API fails any RPC a test didn't set up (`apps/admin/README.md:77`).
- Untested: `SignIn.tsx` (63 lines, two Supabase calls).

**2. Structure and coupling: amber.** The shell is small and readable, but it is coupled to two parked features:

- It learns the caller's role from **`TranslationService.GetTranslator`** (`App.tsx:83`, documented at `apps/admin/README.md:44`). Parking the translator role and the Translations page (#417) leaves the console's admin check riding on a parked service.
- The page table and the translator branch hard-wire the parked pages: `pages` lists `translations` and `cities` (`App.tsx:30,33`), and a non-admin is routed to Translations (`:119-121`). The `#/city/<id>` route goes to `CityEditor` (`:48-49`, `:125`).

Adding the Pilot's new admin pages (the waitlist list, the crew candidates for by-hand offers, and the Metrics success bar, all from #417) means editing `pages` and the ternary chain in `App.tsx:121-127`. That's fine at this size.

**3. Domain fit: green.**

- The labels follow the decided admin model: Groups for proposed rounds and events, Crew leaves, Reports and People.
- The `#/event/<id>` route matches the glossary's note that the code calls an Outing an "event". The admin is internal, so the glossary's "avoid event in the app's copy" doesn't bite.

**4. Docs match: amber.**

- `apps/admin/README.md:3`: "The admin SPA (M8)" is a retired milestone label.
- `:20`: the paging note says "Not yet checked in a browser (#233)". The inventory flags this unverified state.
- The rest is accurate, including the test description at `:75-83`.

**5. Defect history: green.** Two fixes, both in the shell, both small, both with tests, and no open bug issue:

- 7da69df (#301, 2026-11-04): a hash naming an `Object.prototype` key crashed the router. Fixed with `Object.hasOwn` (`App.tsx:58-59`).
- 8b76f2d (#313, 2026-11-04): font subsets were inlined as `data:` URLs, which the CSP blocks.

**6. Pending refactor load: green.** No frozen ticket targets the shell. #320 targets the parked CityEditor.

### Cost to reach the hygiene bar

**Keep, with small fixes: about 1 session.**

- Hide the parked pages (Translations, Cities and CityEditor) from the nav and the routes.
- Give the console a role check that doesn't depend on the parked translation service, for example a caller RPC on `AdminService`. That touches `proto/` and the backend: small, but across parts (#426).
- Fix README lines 3 and 20, which needs the browser check of Load more that `:20` asks for.
- Add a `SignIn` test.

**Rebuild: about 3 sessions.** A router library, the shell, the test kit, and re-pointing the 16 pages.

### Verdict: Keep

No red signal, and the cost is about 1 session, so the Keep rule holds.

---

## 3. Site (`site/`)

**What's in the part:**

- `site/index.html` (1,102 lines). It is one self-contained page with inline CSS and an inline script (`:871-1100`), and no build step.
- `logo.svg`, `favicon.svg`, `apple-touch-icon.png` and `og-image.png`, generated by `scripts/brand/`.
- `wrangler.jsonc` and the 23-line `README.md`.
- The generators `scripts/site/build_map.py` (the plan card's map) and `build_readme_panels.py` (`assets/readme/`).
- The `site` job in `.github/workflows/deploy-web.yml:118-140`.

### Signals

**1. Test quality: red.**

- No tests of any kind (the inventory's "No tests at all"). The only check is the deploy job's `curl -fsS https://joinissho.com/` (`deploy-web.yml:140`).
- The README's design notes (`site/README.md:13-21`) are a detailed prose spec of the animation, but nothing enforces them.
- The waitlist form will add behaviour that does need pinning: submit, consent, error and success states.

**2. Structure and coupling: amber.**

- It is one 1,102-line file mixing markup, CSS and script, plus two generated regions. One is the map between the `map:start` and `map:end` markers. The other is the README panels, generated *from* this page (`site/README.md:18-19`).
- It is coupled to the app's copy and tokens by hand: `apps/mobile/src/i18n/en.json` wording and `constants/motion.ts` timings (`site/README.md:17,20`).
- It can change in place: 12 small PRs did exactly that. But every content change has to rerun two generators.

**3. Domain fit: amber.** The story's model follows the glossary: a plan, then a date (invited), going, on the day, how it went, then a crew. But its place and copy contradict #417, which says the site leads with Da Nang–Hoi An:

- `index.html:842`: "Tokyo first, then your city." (#417 says to remove it).
- The plan card, map, invite and chat are all a Kichijoji walk: `:657-681`, `:724`, `:759`, and the map generated from OpenStreetMap at `:662-670`.
- `:866`: "Built with ❤️ in 🇯🇵".

Japanese copy (`:638`, the footer sign-off) and the "why Issho" kanji section (`:834-837`) are brand. They are not Tokyo-first claims, so the owner decides whether they stay.

This is wrong content on a right model, so amber, not red.

**4. Docs match: green.**

- `site/README.md` agrees with the code, including the placeholder form: `README.md:9` and `index.html:844`, `:872-873`. The form prevents submission and sends nothing.
- The README also says to "update the privacy wording at the same time". That wording is `:850`.

**5. Defect history: amber.** 12 `fix(...)` commits since 2026-10-20, and no open bug issue:

- 56b0474 (#14);
- d16d642 (#115);
- #114, #116, #117 and #121–#127.

All but #14 and #115 landed on 2026-10-26 and 2026-10-27, all visual polish. There have been none since. The count is high for one file, but none was a functional failure.

**6. Pending refactor load: green.** No refactor ticket targets it. Frozen grillings that will shape it:

- #339 Waitlist and unlock, which must take in the site's entries;
- #362 Legal operator and Vietnam filings, which decides whose name goes on the policy;
- #342 Analytics and consent.

### Cost to reach the hygiene bar and the Pilot's site gates

**Renovate: about 2.5 sessions, for the site alone.**

- About 1 session for Da Nang–Hoi An content:
  - remove the "Tokyo first, then your city." heading (`:842`) and the 🇯🇵 line (`:866`);
  - pick a Hoi An or Da Nang walk and rerun `build_map.py`;
  - rewrite the stage fragments' place names;
  - regenerate the README panels (`make readme-panels`).
- About 0.5 sessions for the legal pages, as static pages next to `index.html`: privacy policy, terms and account deletion. The owner supplies the text (#362, #412's legal floor). Link them from the footer.
- About 0.5 sessions to wire the form:
  - a consent tick box for this one purpose, linked to the privacy policy (#417);
  - a POST to the new public RPC;
  - success, error and rate-limited states;
  - the bot-check widget, if one is chosen;
  - a small browser or jsdom test of the form script.
- About 0.25 sessions for a `_headers` file like `apps/admin/public/_headers`, with CSP and a `connect-src` limited to the API. Plus a decision on Google Fonts (`index.html:18-21`): self-host them, or disclose in the privacy policy that they're loaded from Google.

**Outside this part (backend, schema and admin), but needed before the form goes live (#417):**

- a waitlist table in `app`;
- a public, unauthenticated RPC, listed in `auth.Interceptor`'s public procedures (`backend/cmd/api/main.go:123`);
- rate limiting, which the API has none of today;
- a bot check;
- removal on request, plus an unsubscribe link in any email;
- the admin list page;
- `ALLOWED_ORIGINS` including `https://joinissho.com`.

That is about 2–3 sessions, to be counted in #426 and the admin pages, not here.

**Rebuild: about 3.5 sessions.** A new Da Nang–Hoi An page, with the legal pages and the form as above. The animated story is most of the current file (the script alone is `:871-1100`), so rebuilding it costs more than renovating it.

### Verdict: Renovate

There's a red (tests), so the part isn't kept. Rebuild isn't open: domain fit is amber, not red, and tests are red but structure is only amber. That leaves Renovate. The cost check doesn't arise, but renovating (about 2.5 sessions) would win it anyway (rebuilding is about 3.5).

**Note for the owner.** The hygiene bar's test gates name RPCs, core-loop screens, admin pages and the end-to-end test, not the site. If the owner judges the site's red tests irrelevant to a static page, Keep plus the content, legal and form work above is the alternative. That would be an overrule, to record in the ADR.

---

## 4. Hygiene-bar Pilot-gate gaps found in this area

### Dead-code check (knip, Pilot gate)

There's no knip config, and running it with none (`npx knip@5`) gives results that are mostly false positives. Real findings:

- **Unused files (15), all false positives:**
  - 13 Expo `*.web.ts(x)` platform variants: `components/app-tabs.web.tsx`, `splash-overlay.web.tsx`, `lib/secure-storage.web.ts`, `push.web.ts` and others. Metro resolves these by platform extension, which knip doesn't.
  - `apps/mobile/metro.config.js`, which Expo loads.
  - `scripts/ts-coverage-summary.mjs`, which CI runs at `.github/workflows/ci.yml:293`.
  - Because of the variants, knip also reports `components/count-badge.tsx` as unused, but `app-tabs.web.tsx:4` uses it.
- **Unused dependencies:**
  - `@vis.gl/react-google-maps`: used only by the `.web` maps, so a false positive.
  - `expo-font`: the peer behind `useFonts`, so a false positive.
  - `@bufbuild/protoc-gen-es` in `packages/api-client`: belongs to #426.
- **Exported but only used in their own file (true findings, trivial):**
  - mobile: `RefreshFailed` (`components/query-screen.tsx:46`), `columnStyle` (`components/screen-frame.tsx:14`), `untilNextBoundary` (`features/chat/room.ts:61`), `PlanCardsMax` (`features/plans/data.ts:37`) and `PIN` (`features/plans/map-pins.ts:9`), plus 8 exported types;
  - admin: `JOB_POLL_MS` and `POLL_FAILURES_SHOWN` (`pages/Cities.tsx`, a parked page) and the `FakeApi`, `RpcName` and `Call` types in `src/test/fakeApi.ts`.
- **Generated `packages/api-client/src/gen/**` exports:** these need an ignore rule (#426).

**Gate gap:** a knip config that resolves the `.web` and `.native` extensions, treats Expo config files as entries, and ignores generated code (shared with #428's CI work). Then un-export or use the eight names above.

### Untested screens and flows in the mobile shell (core-loop gate: "render-and-interaction test")

- The root navigator: the sign-in, onboarding and tabs guards, the suspended screen, and the account loading and error screen (`src/app/_layout.tsx:96-157`). This is the first screen of the core loop after sign-in.
- Account deletion (`features/account/account-actions.tsx`), which is part of the legal floor.
- The tab layout's push registration and tap routing (`src/app/(tabs)/_layout.tsx`), and the tab badge (`components/app-tabs.tsx`).
- The tabs themselves: only Plans has a screen test (`features/plans/__tests__/plans-tab.test.tsx`). Together (`(tabs)/index.tsx`, 362 lines) and You (`(tabs)/profile.tsx`, 177) have none. They belong to the plans and identity slices, but they hang off the shell's navigator.
- No end-to-end harness exists anywhere in the repo: no Maestro, Detox or Playwright. The core-loop end-to-end gate starts from nothing.

### Admin shell

- The parked Translations and Cities pages are still in the nav, and the role check rides on `TranslationService.GetTranslator` (`App.tsx:83`).
- The Pilot's admin pages that don't exist yet (#417) are scope work, not hygiene: a waitlist list, crew candidates for by-hand offers, and the Metrics success bar.
- Sentry: the hygiene bar names the API, worker and app. The admin SPA has no error reporting. The owner should decide whether it counts.

### Web build (fallback platform)

- The session is kept in `localStorage` (`lib/secure-storage.web.ts:5-10`), and `public/_headers` has no full CSP, only `frame-ancestors 'none'`, `nosniff` and a referrer policy. Both comments defer this to the web-release decision, now #340, which is frozen. If Pilot testers use the web fallback, that decision (a strict CSP, or cookie sessions) is a gate.
- Sentry is off in the web build (`README.md:242`), so web errors wouldn't reach Sentry (an operations gate).

### EAS and iOS (Pilot platform, not hygiene)

- `eas.json` has build profiles and channels only, with no `submit` config.
- `app.config.js` passes only an Android Maps key (`:26-35`).
- iOS has never been built, per the README's M9 lines. TestFlight is a Pilot platform (#412), so it is Distribution work (#343), not part of this verdict.

### What the site needs for the waitlist signup and the legal pages

Before the form goes live, from #417 and #412:

1. Remove "Tokyo first, then your city." (`index.html:842`) and the other Tokyo-first copy. Lead with Da Nang–Hoi An.
2. A consent tick box for this one purpose, linked to the privacy policy. Replace the line at `:850`.
3. Privacy policy, terms and deletion pages. None exist today: the site has no other pages, and nothing in the repo links a policy. The text depends on #362, the legal operator.
4. Wire the form to a public Go API RPC that stores the entry in an `app` table. It must be listed in the public procedures (`backend/cmd/api/main.go:123`) and allowed from `https://joinissho.com` (`ALLOWED_ORIGINS`).
5. Rate limiting (none in the API today) and a bot check. #417 bars a third-party *form service*, but doesn't say whether a third-party bot check is allowed.
6. Removal: an unsubscribe link in any email, and deletion on request.
7. An admin list of entries.
8. A CSP and `_headers` for the site, as the admin has, and a decision on Google Fonts.
9. #339 must take these entries in as the waitlist's first rows.

## Open questions for the verdict ADR

- **Site:** follow the rules (Renovate), or overrule to Keep because the test gates don't name static pages?
- **Admin role check:** a new caller RPC on `AdminService`, or keep `GetTranslator` alive while the translator role is parked?
- **Web fallback:** is #340's session-security decision a Pilot gate, given that web is the Pilot's fallback?
- **Bot check:** may it be a third-party widget such as Turnstile, or must it be self-hosted?
- **Admin SPA errors:** do they need to reach Sentry for the operations gate?
