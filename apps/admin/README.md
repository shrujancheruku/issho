# @issho/admin

The admin SPA (M8): Vite + React, signed in with Supabase Auth (email code, like the app), talking to the Go API through [`@issho/api-client`](../../packages/api-client). It never talks to the database. Deployed as a static-assets-only Cloudflare Worker (below).

## Pages

| Page | What it does | API |
|---|---|---|
| Plans | Per city: the **review switch** (on: the matcher's plans wait as drafts), drafts oldest first, open and closed plans. A plan's page (`#/plan/<id>`) shows it as people would, in English (stops with venue names live from Google, a walk's notes), where it came from (the matcher's audience and score, or by hand) and its round. Drafts: edit the title and summary, **approve** (opens with signups from today) or **reject** (not suggested again for two weeks). Open plans: edit, **withdraw**. **Edit the walk** (a composed walk, draft or open): reorder, remove and add stops (found by name near the area), minutes at each and English notes; saving checks the places with Google and works out the route again in that order, with warnings when it breaks the composer's rules. **Change venues** (other plans): the venue and the follow-up's, from those found in the area. A warning shows when outings of the plan are already scheduled. **New plan**: area, activity, optional follow-up, venue (picked from those found there, or left to pick), languages, optional title and summary; it opens at once. | `ReviewService` |
| Groups | Per city: **proposed** rounds (a city under review: the scheduler's date and groups wait here, nobody invited) and **upcoming** scheduled events. An event's page (`#/event/<id>`): start (local to the city) and answer deadline, each group with its people (gender, age, a link to their person view, "withdrew" if they're no longer signed up) and its meeting point. On a proposal: **move** someone to another group, **take out** someone (they stay signed up), **change the start**, then **approve** (invitations go out) or **reject** (everyone stays signed up; the plan rests a day). A group below its minimum is marked and blocks approval. Meeting points can also be set on scheduled events. | `ReviewService` (events) |
| Venues | Per city: places **to review**, approved and rejected, by area, filtered by area or kind, 50 at a time (**Load more**), names and addresses live from Google; a **flag** marks names that look like a snack bar (スナック), hostess bar or tattoo studio, and "In plans" counts draft or open plans using a place. **Approve** or **reject** one or a selection (a place in every kind it's listed as). The **approved-only switch**: on, new plans use only approved places in the city. **Search** by name with Google in every area or one (one search per area); **Look for snack bars** searches スナック and shows only the places we have; a place we don't have can be **added**, approved, in a kind its Google types fit. | `CurationService` |
| Metrics | Per city: the **north star** (attendees who came back: two or more outings so far) for the last full week and this week so far, the city now, a weekly stacked bar chart (came back / one outing so far) and a table per week (went, came back, first outing, groups, said yes, showed up, ratings, meet-again picks, mutual pairs, new people, crews, reports). | `MetricsService` |
| Cities | City packs on the default branch and in the database, with areas still needing travel times or venues, and the editor's open pull requests. **Edit** a pack, or make a **new city**: settings (names, time zone, currency, languages, travel modes), areas (add, rename, move; a map link per area; never remove) and activities (offered, not offered, or a local English name and its months). Saving **opens a pull request**, or adds a commit to the city's open one (needs `GITHUB_TOKEN` on the API): a city has one rolling pull request, so when it has one the editor opens on *its* version of the pack, says so ("Editing a pending change", linking the pull request) and the button reads **Save to the pull request**; the checks, such as never removing an area, are against `main`. After it's merged: **Apply from main** and **Find venues** run as background jobs (the button shows "Running…" and the page asks `AdminService.GetAdminJob` every 2 s until the job ends, then shows the result or why it failed; reloading the page forgets the job, and applying the same pack again joins the one still running; a failed poll counts as still running until three in a row fail). Apply seeds the database and lists the areas it removed or replaced, with the people, crews and plans moved, and the activity overrides it dropped; rebuilds travel times when areas changed. Find venues is for areas without them (one Google search per kind of place). | `CityPackService`, `AdminService` |
| Reports | Reports about people (`app.reports`): open ones oldest first, with who, where they met and how many reports that person has; mark actioned, dismiss or reopen, with a note. "N reports in all" lists everything about that person. A report changed by the reporter after review is marked. | `ListReports`, `ResolveReport` |
| Flags | System signals for a person to look at (today: meet-again picks leaning to one gender, in words); resolve with a note | `ListFlags`, `ResolveFlag` |
| Crew leaves | Why people left crews, newest first, by reason; "made me uncomfortable" stands out | `ListCrewLeaves` |
| People | Search by name, email or user id; everyone suspended now. A person view (`#/person/<id>`, also linked from names on reports and flags): email, joined, gender and age, home, report counts (with a link to their reports), flags, the last 50 outings (answer, checked in, what their feedback said) and suspension history. **Suspend** with a reason (after a confirm step) or **lift** with a note. | `SearchPeople`, `ListSuspended`, `GetPerson`, `SuspendPerson`, `LiftSuspension` |
| Translations | The translation editor, for admins and translators. **App text:** every UI string of a language with its plain English, the owner's livelier English, the note, the machine translation and any human version; filters *To check*, *English changed* (stale), *In pull request* and *All*. **Looks right** keeps the machine text as checked, **Save** keeps an edited one, **Use the machine text** removes a human version, and **Suggest a better version** (in English, **a livelier version**) fills the box with Gemini's draft to check before saving; each is a commit to the language's one open pull request (linked at the top), which the owner merges. Opening a language starts its machine translations, so it opens on one with an open pull request, then one with human versions, then English. **Activities, places and emails:** machine translations corrected in the database, as before. Placeholders like `{email}` must be kept (checked here and by the API). | `TranslationService` |

Every list loads a page at a time with **Load more** (`src/paging.tsx`: the response's `nextPageToken` is the next request's `pageToken`; a changed filter starts again from the first page). **Not yet checked in a browser** (#233 shipped with Vitest coverage only): next time you work on a list page, click Load more on Reports, Flags, People, Plans, Venues and Translations, check each page adds new rows with no repeats, and that changing a filter starts again from page 1; then delete this note.

`ModerationService`, `ReviewService`, `CurationService`, `MetricsService`, `CityPackService` and `AdminService` are admin-only (see below). Report and flag decisions don't change matching (a report already keeps the pair apart); a suspension does: see the backend README.

## Look

`src/styles.css` follows [docs/design-system.md](../../docs/design-system.md), with the same values as the app: indigo ground, gold buttons, `--accent` for the active page and tab, `--error` for errors and moderation dangers (suspended, unsafe, open reports). Dela Gothic One (the `ISSHO` mark) and Zen Kaku Gothic New are bundled from `@fontsource` in `src/main.tsx`, because the CSP allows only `'self'`; only 400 and 700 of the body face are loaded, which keeps the CSS at about 195 kB gzipped. The metrics chart keeps its own validated series colours.

## Admins

Admins are ordinary Supabase users with `app_metadata.admin = true`, carried in their access token and checked by the API (owner's decision: no admin list in our config). From `backend/`:

```sh
go run ./cmd/tool set-admin someone@example.com          # grant
go run ./cmd/tool set-admin -revoke someone@example.com  # remove
```

**Translators** see only the Translations page, for their languages (`app_metadata.translator_langs`):

```sh
go run ./cmd/tool set-translator someone@example.com ja,es   # grant (replaces their languages)
go run ./cmd/tool set-translator -revoke someone@example.com # remove
```

Both need `DATABASE_URL`, `SUPABASE_URL` and `SUPABASE_SECRET_KEY` for the target project (e.g. `make`-style `ENV_FILE`, or exported in your shell; never printed). The account must exist: sign in to the app once (this page never creates accounts: it signs in with `shouldCreateUser: false`). A change applies at the next token refresh; the "No access" screen's **Check again** refreshes at once. The console asks `TranslationService.GetTranslator` who the caller is.

## Run locally

```sh
cp apps/admin/.env.example apps/admin/.env.local   # fill in the publishable key from `supabase status`
ALLOWED_ORIGINS=http://localhost:5173 make api      # with the backend's usual env; the API must allow this origin
pnpm --filter @issho/admin dev                      # http://localhost:5173
```

Local sign-in codes arrive in Mailpit (`supabase status` → `MAILPIT_URL`). `VITE_*` values are bundled into the page and public: only the API URL, Supabase URL and publishable key go there.

## Deploy

`issho-admin` is a Cloudflare Worker with static assets only (`wrangler.jsonc`: `dist/`, unknown paths get `index.html`), at https://admin.joinissho.com (its `workers.dev` address is switched off). The app's web build deploys the same way as `issho-web`, at https://app.joinissho.com. Both are built against `issho-dev` and the hosted API, https://api.joinissho.com.

```sh
scripts/deploy-web.sh                                # both sites, against https://api.joinissho.com
scripts/deploy-web.sh https://api.joinissho.com admin  # one site: the API URL comes first, then `admin` or `web`
```

The API URL is baked into each build, so redeploy if it changes. `scripts/deploy-web.sh` takes the Supabase URL and publishable key from the environment (`EXPO_PUBLIC_*`) or else `apps/mobile/.env`, and deploys with your `wrangler login` (or `CLOUDFLARE_API_TOKEN`). `public/_headers` sets a strict Content-Security-Policy, `noindex` and no framing. Its `connect-src` allows Supabase and `https://api.joinissho.com`; the script swaps in the API origin it was given if that differs, so the page can't call any other origin.

**From CI:** `.github/workflows/deploy-web.yml` runs the same script on every push to `main` that touches either site. It's skipped until the repository variable `API_URL` is set (`https://api.joinissho.com`). The workflow's header lists the other variables and the `CLOUDFLARE_API_TOKEN` secret (an account API token limited to *Workers Scripts: Edit*).

Anyone can open the page; data needs an admin token, which the API checks on every call. There's deliberately no Cloudflare Access in front (owner's decision, 2026-10-25).

## Checks

`pnpm --filter @issho/admin typecheck`, `lint` (ESLint with typescript-eslint and the React hooks rules, `eslint.config.mjs`), `test` (Vitest: flag wording, placeholder check, and the pages' irreversible flows) and `build`. CI runs all four. `pnpm --filter @issho/admin test:coverage` is `test` with a V8 coverage report (`@vitest/coverage-v8`; written to the gitignored `coverage/`); CI runs it instead of `test` and shows the totals in the job summary. Reported, never gated.

### Page tests

`src/pages/*.test.tsx` render a page in jsdom with Testing Library and fake the API at the `@issho/api-client` boundary: `vi.mock('../lib', ...)` swaps the `api` export for `fake.api` from `src/test/fakeApi.ts`, which records each RPC with its exact request and answers only what the test set with `fake.on('service.method', ...)` (an RPC nobody set up fails). `fetch` is never mocked. `renderPage` (`src/test/render.tsx`) gives each test a fresh query cache without retries; `deferred()` holds an RPC open to click twice while it is in flight.

`ConfirmButton` folds back to its first button as soon as Yes is clicked (the parent disables that button while the RPC is in flight, so a double click sends one RPC), whereas Person's suspend uses `keepOpen` and stays on Yes and Cancel after an error. Focus moves to Yes when the confirmation opens and back to the first button on Cancel or Yes (`src/Confirm.test.tsx`).

Covered: applying a city pack and finding venues as jobs (`Cities.test.tsx`: "Running…", polling every 2 s on fake timers, the report, a failure's message, the refused call, and the previous API's synchronous answer), suspending and lifting a person, approving and rejecting a proposed round, approving, rejecting and withdrawing a plan, and approving and rejecting venues (one row, or a selection), plus the People lists and the Venues search results. Each irreversible action asks first (`src/Confirm.tsx`'s `ConfirmButton`, the pattern of Person's suspend button: the first click only shows "Yes, …" and Cancel; the bulk venue buttons ask once and name the count), and each has a success test (exact RPC arguments, what the screen shows after), an error test, a cancel test (no RPC, nothing changed) and a one-RPC-for-a-double-click test on the Yes button. The Venues search results' Approve, Reject and "Add, approved" are reversible status sets and stay first-click; they have success and error tests. `Groups.tsx` and `Plans.tsx` are list and navigation pages with no irreversible actions (the decisions are on `GroupReview.tsx` and `PlanDetail.tsx`), so they have no tests of their own.

`StopsEditor.test.tsx` covers the walk editor: the Meet/Stop/Finish roles by position, the arrows (the ends are disabled), Remove (never below three), edited minutes and notes, Find and Add (a new place goes before the finish; one already in the walk can't be added again; closed and too-far results have no Add), and the exact `setPlanStops` request, plus the saved time, warnings and an error. `App.test.tsx` covers the hash router through `App` with the pages stubbed: every route, the id forms (`person/`, `plan/`, `event/`, `city/`, `reports?about=`), an unknown or malformed hash falling back to Reports, following a `hashchange`, a deep link kept through sign-in (signed out: the sign-in screen and no RPC; after signing in: the linked page), a translator seeing only Translations, and the no-access screen.
