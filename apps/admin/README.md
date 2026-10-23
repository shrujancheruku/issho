# @issho/admin

The admin SPA (M8): Vite + React, signed in with Supabase Auth (email code, like the app), talking to the Go API through [`@issho/api-client`](../../packages/api-client). It never talks to the database. Deployed as a static-assets-only Cloudflare Worker (below).

## Pages

| Page | What it does | API |
|---|---|---|
| Reports | Reports about people (`app.reports`): open ones oldest first, with who, where they met and how many reports that person has; mark actioned, dismiss or reopen, with a note. "N reports in all" lists everything about that person. A report changed by the reporter after review is marked. | `ListReports`, `ResolveReport` |
| Flags | System signals for a person to look at (today: meet-again picks leaning to one gender, in words); resolve with a note | `ListFlags`, `ResolveFlag` |
| Crew leaves | Why people left crews, newest first, by reason; "made me uncomfortable" stands out | `ListCrewLeaves` |
| Translations | Machine translations per language, unchecked first, with search; correct or confirm one ("Looks right"). Placeholders like `{email}` must be kept (checked here and by the API). A checked translation is never replaced by a machine one. | `ListTranslationLanguages`, `ListTranslations`, `ReviewTranslation` |

All of `ModerationService` is admin-only (see below). Decisions don't change matching: a report already keeps the pair apart.

## Admins

Admins are ordinary Supabase users with `app_metadata.admin = true`, carried in their access token and checked by the API (owner's decision: no admin list in our config). From `backend/`:

```sh
go run ./cmd/tool set-admin someone@example.com          # grant
go run ./cmd/tool set-admin -revoke someone@example.com  # remove
```

It needs `DATABASE_URL`, `SUPABASE_URL` and `SUPABASE_SECRET_KEY` for the target project (e.g. `make`-style `ENV_FILE`, or exported in your shell; never printed). The account must exist: sign in to the app once (this page never creates accounts: it signs in with `shouldCreateUser: false`). A change applies at the next token refresh; the "Not an admin" screen's **Check again** refreshes at once.

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

The API URL is baked into each build, so redeploy after restarting the tunnel. `scripts/deploy-web.sh` takes the Supabase URL and publishable key from `apps/mobile/.env` and deploys with your `wrangler login`. `public/_headers` sets a strict Content-Security-Policy (only Supabase and `*.trycloudflare.com` can be called; narrow it to the API's own origin once it's hosted), `noindex` and no framing.

Anyone can open the page; data needs an admin token, which the API checks. Putting Cloudflare Access in front of it (free for up to 50 users) would add a check before the page loads; not set up yet.

## Checks

`pnpm --filter @issho/admin typecheck`, `test` (Vitest: flag wording, placeholder check) and `build`. CI runs all three.
