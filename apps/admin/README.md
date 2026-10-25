# @issho/admin

The admin SPA (M8): Vite + React, signed in with Supabase Auth (email code, like the app), talking to the Go API through [`@issho/api-client`](../../packages/api-client). It never talks to the database. Deployed as a static-assets-only Cloudflare Worker (below).

## Pages

| Page | What it does | API |
|---|---|---|
| Plans | Per city: the **review switch** (on: the matcher's plans wait as drafts), drafts oldest first, open and closed plans. A plan's page (`#/plan/<id>`) shows it as people would, in English (stops with venue names live from Google, a walk's notes), where it came from (the matcher's audience and score, or by hand) and its round. Drafts: edit the title and summary, **approve** (opens with signups from today) or **reject** (not suggested again for two weeks). Open plans: edit, **withdraw**. **Edit the walk** (a composed walk, draft or open): reorder, remove and add stops (found by name near the area), minutes at each and English notes; saving checks the places with Google and works out the route again in that order, with warnings when it breaks the composer's rules. **Change venues** (other plans): the venue and the follow-up's, from those found in the area. A warning shows when outings of the plan are already scheduled. **New plan**: area, activity, optional follow-up, venue (picked from those found there, or left to pick), languages, optional title and summary; it opens at once. | `ReviewService` |
| Groups | Per city: **proposed** rounds (a city under review: the scheduler's date and groups wait here, nobody invited) and **upcoming** scheduled events. An event's page (`#/event/<id>`): start (local to the city) and answer deadline, each group with its people (gender, age, a link to their person view, "withdrew" if they're no longer signed up) and its meeting point. On a proposal: **move** someone to another group, **take out** someone (they stay signed up), **change the start**, then **approve** (invitations go out) or **reject** (everyone stays signed up; the plan rests a day). A group below its minimum is marked and blocks approval. Meeting points can also be set on scheduled events. | `ReviewService` (events) |
| Venues | Per city: places **to review**, approved and rejected, by area, filtered by area or kind, 50 a page, names and addresses live from Google; a **flag** marks names that look like a snack bar (スナック), hostess bar or tattoo studio, and "In plans" counts draft or open plans using a place. **Approve** or **reject** one or a selection (a place in every kind it's listed as). The **approved-only switch**: on, new plans use only approved places in the city. **Search** by name with Google in every area or one (one search per area); **Look for snack bars** searches スナック and shows only the places we have; a place we don't have can be **added**, approved, in a kind its Google types fit. | `CurationService` |
| Metrics | Per city: the **north star** (attendees who came back: two or more outings so far) for the last full week and this week so far, the city now, a weekly stacked bar chart (came back / one outing so far) and a table per week (went, came back, first outing, groups, said yes, showed up, ratings, meet-again picks, mutual pairs, new people, crews, reports). | `MetricsService` |
| Cities | City packs on the default branch and in the database, with areas still needing travel times or venues, and the editor's open pull requests. **Edit** a pack, or make a **new city**: settings (names, time zone, currency, languages, travel modes), areas (add, rename, move; a map link per area; never remove) and activities (offered, not offered, or a local English name and its months). Saving **opens a pull request** (needs `GITHUB_TOKEN` on the API). After it's merged: **Apply from main** (seeds the database; rebuilds travel times when areas changed) and **Find venues** for areas without them (one Google search per kind of place). | `CityPackService` |
| Reports | Reports about people (`app.reports`): open ones oldest first, with who, where they met and how many reports that person has; mark actioned, dismiss or reopen, with a note. "N reports in all" lists everything about that person. A report changed by the reporter after review is marked. | `ListReports`, `ResolveReport` |
| Flags | System signals for a person to look at (today: meet-again picks leaning to one gender, in words); resolve with a note | `ListFlags`, `ResolveFlag` |
| Crew leaves | Why people left crews, newest first, by reason; "made me uncomfortable" stands out | `ListCrewLeaves` |
| People | Search by name, email or user id; everyone suspended now. A person view (`#/person/<id>`, also linked from names on reports and flags): email, joined, gender and age, home, report counts (with a link to their reports), flags, the last 50 outings (answer, checked in, what their feedback said) and suspension history. **Suspend** with a reason (after a confirm step) or **lift** with a note. | `SearchPeople`, `ListSuspended`, `GetPerson`, `SuspendPerson`, `LiftSuspension` |
| Translations | The translation editor, for admins and translators. **App text:** every UI string of a language with its plain English, the owner's livelier English, the note, the machine translation and any human version; filters *To check*, *English changed* (stale), *In pull request* and *All*. **Looks right** keeps the machine text as checked, **Save** keeps an edited one, **Use the machine text** removes a human version, and **Suggest a better version** (in English, **a livelier version**) fills the box with Gemini's draft to check before saving; each is a commit to the language's one open pull request (linked at the top), which the owner merges. Opening a language starts its machine translations, so it opens on one with an open pull request, then one with human versions, then English. **Activities, places and emails:** machine translations corrected in the database, as before. Placeholders like `{email}` must be kept (checked here and by the API). | `TranslationService` |

`ModerationService`, `ReviewService`, `CurationService`, `MetricsService` and `CityPackService` are admin-only (see below). Report and flag decisions don't change matching (a report already keeps the pair apart); a suspension does: see the backend README.

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

`issho-admin` is a Cloudflare Worker with static assets only (`wrangler.jsonc`: `dist/`, unknown paths get `index.html`), at https://issho-admin.shrujancheruku.workers.dev. The app's web build deploys the same way as `issho-web`. Both are built against `issho-dev`.

Until the API is hosted (M9) it runs on a dev machine behind a **Cloudflare quick tunnel**, a temporary `https://….trycloudflare.com` address (no account or domain needed; it changes every time the tunnel starts and only works while that machine runs the API):

```sh
cloudflared tunnel --url http://localhost:8080      # prints the https://….trycloudflare.com URL
SUPABASE_URL=<issho-dev URL> \
ALLOWED_ORIGINS=https://issho-admin.shrujancheruku.workers.dev,https://issho-web.shrujancheruku.workers.dev \
  make api ENV_FILE=.env.issho-dev
scripts/deploy-web.sh https://….trycloudflare.com  # both sites; add `admin` or `web` for one
```

The API URL is baked into each build, so redeploy after restarting the tunnel. `scripts/deploy-web.sh` takes the Supabase URL and publishable key from the environment (`EXPO_PUBLIC_*`) or else `apps/mobile/.env`, and deploys with your `wrangler login` (or `CLOUDFLARE_API_TOKEN`). `public/_headers` sets a strict Content-Security-Policy, `noindex` and no framing. Its `connect-src` allows Supabase and `*.trycloudflare.com`; the script narrows the deployed copy to the one API origin it was given, so the page can't call any other tunnel (checked in Chrome).

**From CI:** `.github/workflows/deploy-web.yml` runs the same script on every push to `main` that touches either site. It's skipped until the repository variable `API_URL` is set, which should wait for the hosted API (M9): a tunnel's address changes too often. The workflow's header lists the other variables and the `CLOUDFLARE_API_TOKEN` secret (an account API token limited to *Workers Scripts: Edit*).

Anyone can open the page; data needs an admin token, which the API checks on every call. There's deliberately no Cloudflare Access in front (owner's decision, 2026-10-25).

## Checks

`pnpm --filter @issho/admin typecheck`, `test` (Vitest: flag wording, placeholder check) and `build`. CI runs all three.
