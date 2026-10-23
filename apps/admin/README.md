# @issho/admin

The admin SPA (M8): Vite + React, signed in with Supabase Auth (email code, like the app), talking to the Go API through [`@issho/api-client`](../../packages/api-client). It never talks to the database. Planned home: Cloudflare Pages (not deployed yet).

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

## Checks

`pnpm --filter @issho/admin typecheck`, `test` (Vitest: flag wording, placeholder check) and `build`. CI runs all three.
