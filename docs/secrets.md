# Secrets management

**This repository is public.** Anything committed, even to a branch, even briefly, should be treated as leaked. If that happens, rotate the secret first and clean up the history afterwards.

## Rules

1. **Never commit a secret.** Secrets live in the places listed below, never in the repo.
2. **`EXPO_PUBLIC_*` variables are public.** Expo bundles them into the app binary, so anyone can extract them. Only put values there that are safe to publish (API base URL, Supabase publishable key, mobile Maps keys restricted by bundle ID).
3. **Supabase keys:**
   - The **publishable key** (`sb_publishable_…`) is public by design.
   - The **secret key** (`sb_secret_…`), the legacy **service_role** JWT and the **JWT signing keys** are server-only.
   - The Go API needs **none** of these to verify tokens: it uses the public JWKS.
4. **Google Maps keys:** use one *server* key, restricted to the Places and Routes APIs and used only by the backend, plus separate *iOS* and *Android* keys restricted by bundle ID and package name. Set quotas and a budget alert. Setup steps: [google-maps.md](google-maps.md).
5. **Least privilege:** each environment (local, staging, production) gets its own keys.

## Where secrets live

| Secret | Local dev | CI (GitHub Actions) | Production |
|---|---|---|---|
| Supabase JWT signing key | `supabase/signing_keys.json` (gitignored, generated per machine) | not needed | Supabase dashboard (never exported) |
| `DATABASE_URL` (with password) | `.env.example` defaults are the well-known **local-only** Supabase credentials | service container, throwaway | Kamal secrets (`.kamal/secrets`, gitignored, filled from a password manager) |
| Hosted DB URL for a Supabase project (with password) | `.env.<project>` (e.g. `.env.issho-dev`, gitignored), used via `make … ENV_FILE=` | not used | Kamal secrets (`DATABASE_URL`) |
| SMTP password (Resend API key) | `supabase/.env` as `SUPABASE_SMTP_PASS` (gitignored); `config push` uploads it to Supabase as a masked secret | not used | Supabase project config |
| Supabase CLI access token | macOS keychain, via `supabase login --token` with a **scoped** token run in your own terminal (the browser login grants full-account access) | not used | n/a |
| Supabase secret key (`SUPABASE_SECRET_KEY`) | `SECRET_KEY` from `supabase status`, exported in your shell or a gitignored `.env` (never in `.env.example`) | not used (tests use a fake admin) | Kamal secrets |
| Google Maps server key (`GOOGLE_MAPS_API_KEY`) | your shell env or `.env.issho-dev` (gitignored), used via `make … ENV_FILE=` | not used (tests use `httptest` fakes; CI only dry-runs) | Kamal secrets |
| Google Cloud Translation key (`GOOGLE_TRANSLATE_API_KEY`, API server) | shell env / untracked `.env` | not used (tests use `httptest` fakes) | Kamal secrets |
| Google Cloud Agent Platform key for Gemini (`GOOGLE_GEMINI_API_KEY`, API server: interest suggestions; starts `AQ.`) | shell env / untracked `.env` | not used (tests use `httptest` fakes) | Kamal secrets |
| Anthropic API key (`ANTHROPIC_API_KEY`, M6 plan text) | shell env / untracked `.env` | not used (tests use fakes) | Kamal secrets |
| Apple / Google OAuth secrets (post-MVP; not used) | `supabase/.env` (gitignored), referenced as `env(...)` in `supabase/config.toml` | not used | Supabase dashboard |
| Apple `.p8` keys, Android keystores | not stored locally; EAS manages signing credentials | n/a | EAS (Expo) credential store |
| SMTP relay credentials (M2) | not needed (Mailpit) | n/a | Supabase dashboard (custom SMTP) |

`.env.example` files contain **only** local-only defaults or empty placeholders, and are committed on purpose. Any real `.env`/`.env.*` file is gitignored.

## Automated protection

| Layer | What | Where |
|---|---|---|
| `.gitignore` | Blocks `.env*`, `*.pem`, `*.p8`, `*.p12`, `*.key`, keystores, `credentials.json`, `google-services.json`, `GoogleService-Info.plist`, `supabase/signing_keys.json`, `supabase/.env`, `.kamal/secrets*` | repo root |
| Pre-commit hook | gitleaks scans staged changes and blocks the commit if it finds a secret | `.githooks/pre-commit` (enabled by `make setup`) |
| CI `secrets` job | gitleaks scans **all history on all branches** on every push and PR | `.github/workflows/ci.yml` |
| Custom rules | JWK private keys (`"d"`), Supabase `sb_secret_…` keys, service_role JWTs, plus gitleaks' ~150 default rules (AWS, GCP, Stripe, Anthropic, private keys, …) | `.gitleaks.toml` |
| GitHub | Secret scanning + **push protection** (free for public repos). **Enable both:** Settings → Code security. | repo settings |

Run a full scan any time with `make secrets-scan`.

### False positives
Never bypass the hook with `git commit --no-verify`. Add a narrowly scoped `[allowlist]` entry (path or regex) to `.gitleaks.toml` in a PR, so the exception gets reviewed.

## If a secret leaks

1. **Rotate it immediately** in the provider's console (Supabase, Google Cloud, Anthropic, …). Assume it's already been scraped; bots scan public GitHub within minutes.
2. Check the provider's logs and usage for abuse.
3. Remove it from the code. If you're rewriting history (`git filter-repo`), coordinate first, since everyone will need to re-clone. Rotation is what actually fixes the leak; the rewrite is cleanup.
4. Add a detection rule to `.gitleaks.toml` if the scanners missed it.
5. Record what happened in the PR or issue.
