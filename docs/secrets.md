# Secrets management

**This repository is public.** Anything committed, even to a branch, even briefly, should be treated as leaked. If that happens, rotate the secret first and clean up the history afterwards.

This file names where each of v2's secrets lives. It never holds a value.

## Rules

1. **Never commit a secret.** Secrets live in the places listed below, never in the repo.
2. **`EXPO_PUBLIC_*` variables are public.** Expo bundles them into the app binary, so anyone can extract them. Only put values there that are safe to publish, such as the API's base URL.
3. **Supabase keys:**
   - The **publishable key** (`sb_publishable_…`) is public by design.
   - The **secret key** (`sb_secret_…`), the legacy **service_role** JWT and the **JWT signing keys** are server-only.
   - The Go API needs **none** of these to verify tokens: it uses the public JWKS.
4. **Least privilege:** each environment (dev, and later production) gets its own keys and its own secrets.
5. **Only CI reads dev's secrets at deploy time.** Workers never read a secret's value on their machine; see [orchestration](agents/orchestration.md#environment).

## Where secrets live

### Dev: Google Secret Manager

Dev's secrets are in Google Cloud project `issho-dev-512213` (Secret Manager). The deploy reads them at run time; the repo holds only their names, in `.kamal/secrets.dev` and `scripts/deploy-dev.sh`.

| Secret name | Read by | What it is |
|---|---|---|
| `dev-database-url` | the API (`DATABASE_URL`, through `.kamal/secrets.dev`) | the Supabase transaction pooler's URL for the app's own role |
| `dev-supabase-auth-url` | the API (`SUPABASE_AUTH_URL`, through `.kamal/secrets.dev`) | `https://<ref>.supabase.co/auth/v1`, the tokens' issuer and where the API reads their keys. Public; stored only to keep the project's ref out of this repository. The founder adds it before the next deploy |
| `dev-supabase-url` | the web build (`EXPO_PUBLIC_SUPABASE_URL`, a Kamal builder secret through `.kamal/secrets.dev`) | `https://<ref>.supabase.co`, the project's address for sign-in. Public once bundled; stored only to keep the ref out of this repository. The founder adds it before the next deploy |
| `dev-supabase-publishable-key` | the web build (`EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY`, a Kamal builder secret) | the project's publishable key (`sb_publishable_…`). Public once bundled; never the secret key. The founder adds it before the next deploy |
| `dev-worker-database-url` | the worker (`WORKER_DATABASE_URL`, through `.kamal/secrets.dev`) | the session pooler's URL; River's LISTEN/NOTIFY needs a session connection, not the transaction pooler |
| `dev-migrate-database-url` | the migration step of `scripts/deploy-dev.sh`, before any container is replaced | the URL goose migrates with |
| `dev-resend-api-key` | the worker (`RESEND_API_KEY`) | the Resend API key for product email |
| `dev-email-from` | the worker (`EMAIL_FROM`) | the sender address |
| `dev-ssh-private-key` | Kamal (`SSH_PRIVATE_KEY`) | the key the deploy uses to reach the dev VM |
| `dev-ssh-host-key` | `scripts/dev-ssh-pin.sh`, with `gcloud`, before Kamal connects (not through `.kamal/secrets.dev`) | the VM's public host key, pinned so an unknown or changed host is refused (#605) |
| `dev-origin-cert`, `dev-origin-key` | Kamal (`ORIGIN_CERT`, `ORIGIN_KEY`) | the Cloudflare origin certificate and key for the dev names |
| `dev-cloudflare-api-token` | `scripts/deploy-dev.sh`, which deploys the admin Worker | an API token that can deploy Workers |

**Native builds** take the same two values from EAS environment variables, not Secret Manager. The founder sets them (a worker never runs `eas`), for each EAS environment the builds use:

```sh
eas env:set --name EXPO_PUBLIC_SUPABASE_URL --value "<project url>" --environment development --visibility plaintext
eas env:set --name EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY --value "<publishable key>" --environment development --visibility plaintext
```

`--environment` names an EAS environment (`development`, `preview` or `production`), not a build profile in `eas.json`, so these commands are right as written without one. The founder confirms which environment the dev build uses when running them; repeat for each environment the builds use.

Kamal's own registry login is not in Secret Manager; see CI below.

### CI: GitHub Actions

- **No cloud key is stored in GitHub.** The deploy job signs in to Google Cloud by Workload Identity Federation (#605). Its provider and deploy service account are named in `.github/workflows/deploy-dev.yml`, and the provider accepts only `main` (#588, #605).
- **`KAMAL_REGISTRY_PASSWORD`** is the job's own `GITHUB_TOKEN`, passed in its environment. It expires with the job.
- **v1's `CLOUDFLARE_API_TOKEN` secret and `CLOUDFLARE_ACCOUNT_ID` variable were deleted** from GitHub Actions by the founder (#605). The admin's token is read from Secret Manager instead.

- **Not a secret, but required:** the Cloudflare zone's "Remove visitor IP headers" managed transform, set by hand by the founder, keeps members' addresses out of the proxy's log ([Deploying dev](deploy.md#a-required-cloudflare-zone-setting), ADR 0005).

### Local

- The repo stores no secret. Real `.env` and `.env.*` files are gitignored, as are `supabase/.env`, `supabase/signing_keys.json` and `.kamal/secrets*`. The one `.kamal/` file that is committed holds names only (`.kamal/secrets.dev`).
- Workers never read any of these files.

### Production

Not set up. Production stands up at the end of milestone 3 ([build order](spec/build-order.md)). Its secrets are listed here when it does.

## Automated protection

| Layer | What | Where |
|---|---|---|
| `.gitignore` | Blocks `.env*`, `*.pem`, `*.p8`, `*.p12`, `*.key`, keystores, `credentials.json`, `google-services.json`, `GoogleService-Info.plist`, `supabase/signing_keys.json`, `supabase/.env` and `.kamal/secrets*`, except `.kamal/secrets.dev` (names only) | repo root |
| Pre-commit hook | gitleaks scans staged changes and blocks the commit if it finds a secret | `.githooks/pre-commit` (enabled by `make setup`) |
| CI `secrets` job | `make secrets-scan`: gitleaks scans **all history on all branches** on every push and PR | `.github/workflows/ci.yml` |
| Custom rules | Private JWKs (the `"d"` field), Supabase `sb_secret_…` keys and service_role JWTs, plus gitleaks' default rules | `.gitleaks.toml` |
| GitHub | Secret scanning and **push protection** (free for public repos). **Enable both:** Settings → Code security. | repo settings |

Run a full scan any time with `make secrets-scan`.

### False positives

Never bypass the hook with `git commit --no-verify`. Add a narrowly scoped `[allowlist]` entry (path or regex) to `.gitleaks.toml` in a PR, so the exception gets reviewed.

## If a secret leaks

1. **Rotate it immediately** in the provider's console (Supabase, Google Cloud, Resend, Cloudflare, …). Assume it's already been scraped; bots scan public GitHub within minutes.
2. Check the provider's logs and usage for abuse.
3. Remove it from the code. If you're rewriting history (`git filter-repo`), coordinate first, since everyone will need to re-clone. Rotation is what actually fixes the leak; the rewrite is cleanup.
4. Add a detection rule to `.gitleaks.toml` if the scanners missed it.
5. Record what happened in the PR or issue.
