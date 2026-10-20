# Hosted Supabase project setup

How to configure a hosted Supabase project for Issho so it matches the local stack (`supabase/config.toml`) and the security assumptions in the code. Do this once per environment (e.g. `issho-dev`, later `issho-prod`).

> **Secrets** created along the way (database password, secret API key, SMTP password) go into a password manager and from there into the API server's environment. Never put them in the repo, a `.env.example` or an `EXPO_PUBLIC_*` variable. See [secrets.md](secrets.md).

Dashboard menu names change from time to time. If a label below doesn't match, search the Supabase docs for the setting name.

## 1. Create the project

| Setting | Value |
|---|---|
| Name | `issho-dev` (or `issho-prod`) |
| Region | **Northeast Asia (Tokyo)**, closest to the first city's users |
| Database password | Generate a long random one and store it in the password manager |
| Plan | Free for development. Free projects **pause after 7 days without activity**, so production needs Pro. |

## 2. Turn off the Data API

**Project Settings → Data API → disable.**

The app never queries Supabase's database directly; everything goes through the Go API. With the Data API on, anonymous clients could read and *write* tables in the `public` schema. That includes PostGIS's `spatial_ref_sys`, whose grants belong to Supabase's internal admin role and can't be revoked by our migrations. Auth keeps working with the Data API off.

## 3. Authentication: email codes only

**Authentication → Sign In / Providers**

| Setting | Value | Why |
|---|---|---|
| Allow new users to sign up | On | The first sign-in creates the account |
| **Email** provider | On | The only sign-in method in the MVP |
| Confirm email | On (default) | New users verify their address with the same 6-digit code |
| Email OTP expiration | **600** seconds | Matches `otp_expiry` locally (the default is 3600) |
| Email OTP length | 6 (if shown) | The app expects 6 digits |
| Phone, anonymous sign-ins, all social providers (Apple, Google, …) | Off | Not used. Apple and Google sign-in are deferred until after the MVP. |

**Authentication → URL Configuration:** set **Site URL** to the product site (e.g. `https://joinissho.com`). No redirect URLs are needed, because sign-in uses codes, not links.

## 4. Email templates

**Authentication → Emails → Templates.** Set **both** templates. Existing users get "Magic link"; brand-new users get "Confirm signup".

| Template | Subject | Body (paste the file's contents) |
|---|---|---|
| Confirm signup | `Your Issho sign-in code` | [`supabase/templates/confirmation.html`](../supabase/templates/confirmation.html) |
| Magic link | `Your Issho sign-in code` | [`supabase/templates/magic_link.html`](../supabase/templates/magic_link.html) |

The bodies show `{{ .Token }}`, the 6-digit code, rather than a link.

## 5. Email delivery (SMTP)

Supabase's built-in email only reaches members of your Supabase organisation, at about **2 emails per hour**. That's fine for a quick test of your own account, but real users need a custom SMTP relay.

1. Create an account with a relay (e.g. Resend or Amazon SES, both with free tiers) and verify a sending domain, e.g. `joinissho.com` (DNS records for SPF/DKIM).
2. **Authentication → Emails → SMTP Settings:** enable custom SMTP and fill in host, port, username, **password** (a secret), sender address `no-reply@<your domain>` and sender name `Issho`.
3. **Authentication → Rate Limits:** custom SMTP starts at **30 emails/hour**. Raise it for launch (e.g. 100–300) according to the relay's limits. Keep the default of one code request per user every 60 seconds.

## 6. JWT signing keys (ES256)

The Go API only accepts asymmetric tokens (ES256/RS256) and verifies them with the project's public JWKS. It rejects tokens signed with the legacy shared secret.

**Project Settings → JWT Keys** (if the project still shows a legacy JWT secret):

1. **Migrate JWT secret.** This creates a standby asymmetric key; make sure it's **ES256**.
2. **Rotate keys.** New tokens are now signed with ES256, and existing ones stay valid.
3. **Project Settings → API Keys:** disable the legacy `anon` and `service_role` keys. We use the new publishable and secret keys instead.
4. After at least **1 hour 15 minutes** (access-token lifetime plus caching), **revoke** the legacy secret.

Check it worked: `https://<project-ref>.supabase.co/auth/v1/.well-known/jwks.json` returns a key with `"alg":"ES256"`.

Session settings (Authentication → Sessions) can stay at their defaults: 1-hour access tokens, refresh-token rotation on.

## 7. API keys

**Project Settings → API Keys**

| Key | Goes to | Public? |
|---|---|---|
| **Publishable** key (`sb_publishable_…`) | Mobile app: `EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | Yes, by design |
| **Secret** key (`sb_secret_…`); create a dedicated one named `issho-api` | API server only: `SUPABASE_SECRET_KEY` | **No.** It bypasses all access rules. |

A dedicated secret key per service can be rotated or revoked without affecting anything else.

## 8. Database connection

**Project Settings → Database:** turn on **Enforce SSL**.

From the **Connect** dialog, copy the **Session pooler** connection string (IPv4, port **5432**):

```
postgres://postgres.<project-ref>:<db-password>@aws-<n>-ap-northeast-1.pooler.supabase.com:5432/postgres?sslmode=require
```

Don't use the **transaction pooler** (port 6543). pgx's prepared statements and the River job queue's LISTEN/NOTIFY don't work through it. The direct connection (`db.<project-ref>.supabase.co:5432`) also works, but only over IPv6.

## 9. Apply migrations and seed data

From your machine, with the session-pooler URL. The database password is in it, so don't paste it into shared shells or scripts:

```sh
export DATABASE_URL='postgres://postgres.<project-ref>:<db-password>@<pooler-host>:5432/postgres?sslmode=require'
make migrate-up seed
unset DATABASE_URL
```

This creates the `app` schema (with no access for `anon`/`authenticated`), links `app.users` to `auth.users`, and loads the activity catalogue and the Tokyo city pack. goose's bookkeeping table goes into its own `goose` schema.

## 10. Wire up the environments

| Where | Variable | Value |
|---|---|---|
| Mobile `apps/mobile/.env` | `EXPO_PUBLIC_SUPABASE_URL` | `https://<project-ref>.supabase.co` |
| | `EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY` | publishable key |
| | `EXPO_PUBLIC_API_URL` | URL of the Go API (local or deployed) |
| API server environment | `SUPABASE_URL` | `https://<project-ref>.supabase.co` |
| | `DATABASE_URL` | session-pooler URL (step 8) **(secret)** |
| | `SUPABASE_SECRET_KEY` | `issho-api` secret key **(secret)** |

## 11. Verify

1. `curl https://<project-ref>.supabase.co/auth/v1/.well-known/jwks.json` shows an ES256 key.
2. Run the API locally against the hosted project (variables above) and run `make api`. Then `HealthService/Check` should return `"databaseOk": true`.
3. Sign in from the app with your own email. The code arrives through your SMTP relay. Before SMTP is set up, the built-in service only delivers to your organisation's members.
4. `IdentityService/GetMe` returns your user, and **Delete account** removes it (check **Authentication → Users**).

## Expected advisor warnings

Supabase's **Advisors** page will flag *"Extension in public schema"* for `postgis`, and *"RLS disabled"* on `public.spatial_ref_sys`. Both are mitigated by the Data API being off: no anonymous path reaches them. Moving PostGIS into the `extensions` schema is tracked in [ROADMAP.md](ROADMAP.md#5-known-gaps--risks). It's cheapest to do before any production data exists.
