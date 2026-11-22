# supabase

Configuration for the **local** Supabase stack: `supabase start` runs Postgres 17, Auth, Studio and a mail catcher in Docker, with the ports in `config.toml` (54320 to 54329, the same for every worktree, so one stack at a time).

## What is on and what is off

| Feature | State | Why |
|---|---|---|
| Postgres | on | The store, the job queue and the chat fan-out. App tables are in the `app` schema, River's in `river`, goose's bookkeeping in `goose`; nothing is created in `public`. |
| Auth | on | Sign-in only, with an email code. The Go API verifies the tokens against the JWKS. |
| Studio, mail catcher | on | A database browser at http://127.0.0.1:54323; sign-in emails land at http://127.0.0.1:54324. |
| Data API (PostgREST) | **off** (`[api] enabled = false`) | Clients never query the database directly; everything goes through the Go API. Don't grant `anon` or `authenticated` anything or write RLS policies to expose data: add an RPC. |
| Migrations and seed | **off** (`[db.migrations]`, `[db.seed]`) | goose owns the migrations (`backend/db/migrations`). Never use `supabase db push`, `supabase db pull`, `supabase migration new` or a `supabase/migrations/` directory. |
| Realtime, Storage, Edge Functions, Analytics | off | Chat and jobs run in the Go API and the worker, and v2 has no file storage. |

## Sign-in

Sign-in is a 6-digit code emailed by Auth (`[auth.email]`: `otp_length = 6`, good for ten minutes), never a link. A new person is sent the "confirmation" email and a returning one the "magic link" email, so `[auth.email.template.confirmation]` and `[auth.email.template.magic_link]` both use `templates/code.html`, which shows the code. Locally the email lands in the mail catcher (`[local_smtp]`, http://127.0.0.1:54324); a hosted project sends it through Resend's SMTP, set in its `[remotes.<project>]` override when one exists. The Go API checks the tokens against the project's public JWKS (`<auth url>/.well-known/jwks.json`), accepts ES256 and RS256 only, and never uses the JWT secret (`backend/README.md`, "Accounts and sign-in").

### A password for one account

The store reviewers sign in with an email and a password ([Technical operations](../docs/spec/operations.md#the-store-reviewers-way-in)). Auth allows it beside the code: the email provider offers both methods to every account, and password sign-in works only for an account that has a password. An account made by the code never has one, so creating the reviewer's account with a password (the dashboard's "Add user", or the admin API with the email confirmed) gives that one account a password and leaves every other on the code. There is no per-account switch, and Auth's own `signUp` with a password stays callable by anyone who reaches the project's Auth URL; the app never calls it. Checked against Supabase's documentation, not yet against a running stack (workers never start one): the first `supabase start` should sign in once by code and once by password.

The CLI (3.2.0) warns that `[auth.email] enable_signup` is deprecated in favour of `enabled`; it still works and is left as the Foundation set it.

## Commands

```sh
supabase start      # first run pulls the images; needs Docker
supabase status     # URLs and local keys
make migrate-up     # apply the goose migrations to it (backend/README.md, "Migrations")
make migrate-down   # roll back the latest one
supabase stop       # stop the stack, keeping its data
```

`make migrate-up` and `make migrate-down` always use the local database (`postgresql://postgres:postgres@127.0.0.1:54322/postgres`) and ignore a `DATABASE_URL` in the environment, which usually points at a hosted project.

A local database left by v1 (it used the same project ID, `issho`) still holds v1's `app` schema, so `make migrate-up` stops with `schema "app" already exists`. Wipe it once with `supabase stop --no-backup` and start again; this deletes the local data.

Hosted configuration is pushed from this file with `supabase config diff` and `supabase config push`, using a `[remotes.<project>]` override for each project. Always review `config diff` first. No project is set up here yet.
