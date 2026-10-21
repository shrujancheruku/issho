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
| PostgREST / Data API | ❌ **disabled** (`[api] enabled = false`) | Clients go through the Go API. With the Data API on, `anon` could read and write public-schema tables, e.g. PostGIS's `spatial_ref_sys`, whose grants belong to `supabase_admin` and can't be revoked by our migrations. `anon` and `authenticated` also have no grants on `app`. |
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

The app signs in with **6-digit email codes** (not links, which can't reliably open a native app). `config.toml` points the `magic_link` and `confirmation` templates at `templates/*.html`, which show `{{ .Token }}`.

**Emails in the user's language.** Each email is written in the language in the account's metadata (`user_metadata.lang`): the app sends the device language when it creates the account, and later keeps it equal to the profile's main language. The templates branch on it (`{{ if eq $lang "ja" }}…`) and fall back to English; the subjects do the same. They're **generated**: edit `templates/source.json` (the English sentences and the list of languages), then run `make email-templates` (needs `DATABASE_URL` for the translation cache and `GOOGLE_TRANSLATE_API_KEY`). It machine-translates through `app.translations` and rewrites `templates/magic_link.html`, `templates/confirmation.html` and the two `subject` lines here; nothing is written unless every language succeeds. Write unambiguous English: "This code expires" became "this *coupon* code" in Chinese until it said "This sign-in code". Never edit the generated files by hand. Codes expire after **10 minutes** (`otp_expiry = 600`). Locally, every email lands in Mailpit (http://127.0.0.1:54324). After editing templates or config, run `supabase stop && supabase start`.

For the hosted project, the templates, subjects, SMTP relay and resend limit are pushed from this config with `supabase config push` (review `supabase config diff` first); the code expiry is set by hand in the dashboard. See [../docs/supabase-hosted.md](../docs/supabase-hosted.md).

## Auth providers

The MVP uses **email OTP only** (`[auth.email]`). All `[auth.external.*]` providers stay disabled; Apple and Google sign-in are deferred until after the MVP. If a provider is added later, reference its secret as `env(...)` and put the value in the gitignored `supabase/.env`. Never write it inline.

The hosted project is set up separately; see [../docs/supabase-hosted.md](../docs/supabase-hosted.md).
