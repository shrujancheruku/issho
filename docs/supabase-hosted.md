# Hosted Supabase project setup

How a hosted Supabase project is configured for Issho, almost entirely with the **Supabase CLI**. `supabase/config.toml` is the source of truth. Settings that must differ from the local stack live in a `[remotes.<name>]` block, which `supabase config push` applies to that project.

Current projects:

| Name | Ref | Region | Plan |
|---|---|---|---|
| `issho-dev` | `yjnzrhusfxilktbjrzsw` | ap-northeast-2 (Seoul) | Free |

For **production**, create the project in **ap-northeast-1 (Tokyo)**, closest to the first city's users. A project's region can't be changed later.

> **Secrets** (database password, SMTP key, Supabase secret key) live only in gitignored files on your machine and in the deployment's secret store. Never paste them into chats, commands or issues. Commands below read them from files and filter their output. See [secrets.md](secrets.md).

## 0. Prerequisites

- Supabase CLI (`brew install supabase/tap/supabase`).
- **Log in with a scoped personal access token**, not the browser flow. The browser flow creates a classic token with full access to every organisation and project on your account. Create a token limited to the Issho project(s) under **Account → Access Tokens**, then run `supabase login --token <token>` **in your own terminal**.

## 1. Create and link the project

Create the project in the dashboard (name, region, generated DB password saved in your password manager), then:

```sh
supabase projects list               # find its ref
supabase link --project-ref <ref>    # stored in supabase/.temp (gitignored)
```

Add a `[remotes.<name>]` block for it in `supabase/config.toml`, copying `[remotes.issho-dev]` (see section 3 for why each override exists).

## 2. Secret files (gitignored)

| File | Contents | Used by |
|---|---|---|
| `.env.<name>` (e.g. `.env.issho-dev`) | `DATABASE_URL=postgres://postgres.<ref>:<password>@<pooler-host>:5432/postgres?sslmode=require`. The **session pooler** URL from the dashboard's **Connect** dialog. | `make … ENV_FILE=.env.<name>` |
| `supabase/.env` | `SUPABASE_SMTP_PASS=<Resend API key>` | `supabase config push` (resolves `env(SUPABASE_SMTP_PASS)`) |

- **Use the session pooler (port 5432)**, not the transaction pooler (6543). pgx's prepared statements and River's LISTEN/NOTIFY don't work through the transaction pooler. The direct connection also works, but it's IPv6-only.
- The Makefile reads `ENV_FILE` with make's `include`. Values must not contain `$` or `#`; if a password does, reset it to one without them.

## 3. Push the configuration

```sh
supabase config diff        # always review first: every line should be intended
supabase config push --yes
```

What the `[remotes.issho-dev]` overrides keep different from the local stack, and why:

| Setting | Local | Hosted | Why |
|---|---|---|---|
| `auth.email.enable_confirmations` | off | **on** | New users verify their address with the same code |
| `auth.email.max_frequency` | 1s | **1m** | At most one code request per address per minute |
| `auth.rate_limit.email_sent` | 2/h | **30/h** | The local default would block testing; raise it before launch |
| `auth.email.smtp` | Mailpit | **Resend** | See below |
| `auth.site_url`, redirect URLs | 127.0.0.1 | Supabase default | Sign-in uses codes, not links |
| `db.pooler` sizes | 20/100 | 15/200 | Keep the hosted plan's limits |
| `db.ssl_enforcement` | n/a | **on** | Set with `supabase ssl-enforcement update --experimental --project-ref <ref> --enable-db-ssl-enforcement` |

Shared settings pushed from the base config include:
- **Data API disabled** (`[api] enabled = false`). Anonymous clients could otherwise read and write public tables such as PostGIS's `spatial_ref_sys`.
- The **code email templates** (`supabase/templates/*.html`).
- TOTP multi-factor auth **off** (unused).

**Not managed by `config push`** (CLI 2.121); set these in the dashboard:
- **Email OTP expiration: 600 s.** Authentication → Sign In / Providers → Email. The default is 3600.

### Email (SMTP) is required

On the **Free plan**, Supabase refuses custom email templates while its built-in email service is in use. The built-in templates only contain a sign-in *link*, so **the app's code sign-in doesn't work until custom SMTP is configured**. The built-in service also only reaches organisation members, at about 2 emails an hour.

Issho uses **Resend** (free tier): host `smtp.resend.com`, port 465, user `resend`, password = a Resend API key with sending access.
- **Development:** without a verified domain, Resend only sends from `onboarding@resend.dev`, and **only to the Resend account owner's address**.
- **Before real users:** verify a sending domain in Resend (DNS records), then change `admin_email` to e.g. `no-reply@<domain>`.

## 4. Keys

```sh
supabase projects api-keys --project-ref <ref> -o json   # contains the SECRET key: filter output, never paste it
```

| Key | Goes to |
|---|---|
| **Publishable** (`sb_publishable_…`) | Mobile app `EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY` (public by design) |
| **Secret** (`sb_secret_…`), ideally a dedicated `issho-api` key (dashboard) | API server `SUPABASE_SECRET_KEY` only |

In the **dashboard** (no CLI equivalent):
- **Project Settings → API Keys → Legacy API keys:** disable the legacy `anon` and `service_role` keys.
- **Project Settings → JWT Keys:** new projects already sign with **ES256**. Revoke any legacy JWT secret listed under *Previously used*.

## 5. Schema and seed

```sh
make migrate-up seed ENV_FILE=.env.<name>
```

This creates the `app` schema (with no access for `anon`/`authenticated`), links `app.users` to `auth.users`, puts goose's bookkeeping table in its own `goose` schema, and loads the activity catalogue and the Tokyo city pack.

## 6. Verify

```sh
# Database state (runs through the Management API, no password needed)
supabase db query --linked "select to_regclass('goose.db_version') is not null as goose_private,
  has_schema_privilege('anon','app','USAGE') as anon_can_use_app,
  exists(select 1 from pg_constraint where conname='users_auth_fk') as users_fk"
# expected: goose_private = true, anon_can_use_app = false, users_fk = true

# Data API is off: anonymous REST calls get 503
curl -s -o /dev/null -w "%{http_code}\n" "https://<ref>.supabase.co/rest/v1/spatial_ref_sys" -H "apikey: <publishable key>"

# Signing keys: expect an ES256 key
curl -s "https://<ref>.supabase.co/auth/v1/.well-known/jwks.json"

# Advisors: only the known PostGIS "extension_in_public" warning is expected
supabase db advisors --linked
```

Then run the API against the project and check `HealthService/Check` returns `"databaseOk": true`:

```sh
(set -a; . ./.env.<name>; set +a; SUPABASE_URL=https://<ref>.supabase.co make api)
```

Finally, sign in from the app. Point `apps/mobile/.env` at the hosted project (`EXPO_PUBLIC_SUPABASE_URL=https://<ref>.supabase.co` and its publishable key) and request a code. During development, the code arrives only at the Resend account owner's inbox.

## Known advisor warning

`extension_in_public` for `postgis`. Harmless while the Data API is off; moving PostGIS into the `extensions` schema is tracked in [ROADMAP.md](ROADMAP.md#5-known-gaps--risks).
