# supabase

Configuration for the **local** Supabase stack (`supabase start`). It provides Postgres 17 + PostGIS, Auth, Storage, Studio and Mailpit in Docker.

## What we use and what we don't

| Supabase feature | Used? | Notes |
|---|---|---|
| Postgres + PostGIS | ✅ | Our tables live in schema `app`. **goose** owns migrations (`backend/db/migrations`), not the Supabase CLI. |
| Auth | ✅ | Issues JWTs that the Go API verifies via JWKS |
| Storage | planned | Photos (S3-compatible API) |
| Studio | ✅ (local) | DB browser at http://127.0.0.1:54323 |
| Mailpit | ✅ (local) | Catches auth emails at http://127.0.0.1:54324 |
| PostgREST / Data API | ❌ | Clients go through the Go API. `anon` and `authenticated` have no grants on `app`. |
| Realtime, Edge Functions, Analytics, Vector | ❌ | Disabled in `config.toml` to keep the stack light. The Go API and River worker cover these needs. |
| `supabase/migrations`, `seed.sql` | ❌ | Disabled; see goose and `cmd/tool seed` |

## Commands

```sh
supabase start        # start (first run pulls images; needs Docker/OrbStack)
supabase status       # URLs and local keys (publishable key, DB URL, …)
supabase stop         # stop, keeping data
supabase db reset     # WIPE local DB (then: make migrate-up seed)
```

## JWT signing keys

The API only accepts **asymmetric** tokens (ES256/RS256). Locally, Supabase signs with the key in `signing_keys.json`:

```sh
echo '[]' > supabase/signing_keys.json
supabase gen signing-key --algorithm ES256
```

`signing_keys.json` holds a **private key**. It's gitignored and blocked by the secret scanner, and each developer generates their own. In hosted projects, enable asymmetric JWT signing keys in the dashboard; the private key never leaves Supabase.

## Sign-in emails

The app signs in with **6-digit email codes** (not links, which can't reliably open a native app). `config.toml` points the `magic_link` and `confirmation` templates at `templates/*.html`, which show `{{ .Token }}` in English and Japanese. Codes expire after **10 minutes** (`otp_expiry = 600`). Locally, every email lands in Mailpit (http://127.0.0.1:54324). After editing templates or config, run `supabase stop && supabase start`.

For the hosted project, the same templates and expiry must be set in the dashboard (Auth → Emails), together with a production SMTP relay, and a sensible resend limit (`max_frequency`, e.g. 60s).

## Auth providers (M2)

Apple, Google and email OTP get configured in `config.toml` under `[auth.external.*]` and `[auth.email]`. Provider secrets are referenced as `env(...)` and supplied from the gitignored `supabase/.env`. Never write them inline.
