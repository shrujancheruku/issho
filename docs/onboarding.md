# Engineer onboarding

Follow this top to bottom on a fresh machine. By the end you'll have the API, the database and the mobile app running locally, and you'll know how to make and ship a change. It's written for macOS on Apple Silicon; Linux works with equivalent packages.

## 1. Install tools

| Tool | Version | Install |
|---|---|---|
| Git | any recent | Xcode Command Line Tools (`xcode-select --install`) |
| Go | as in `backend/go.mod` (`go` line) | `brew install go` |
| Node.js | 24 | `brew install node@24` (or fnm/nvm) |
| pnpm | as in root `package.json` (`packageManager`) | `corepack enable` (bundled with Node) |
| Docker runtime | any | `brew install --cask orbstack`, then open OrbStack once |
| Supabase CLI | latest | `brew install supabase/tap/supabase` |
| Xcode + iOS Simulator | latest | App Store (only for running the iOS app) |

You **don't** need to install buf, sqlc, goose or gitleaks. They're pinned in `backend/go.mod` (or the `Makefile`) and run through `go tool` / `go run`.

## 2. Clone and set up

```sh
git clone git@github.com:shrujancheruku/issho.git
cd issho
make setup        # pnpm install + enables git hooks (secret scan, commit-message lint)
```

`make setup` sets `git config core.hooksPath .githooks`, which enables two hooks:
- **pre-commit:** scans staged changes for secrets. CI scans every push too, but by then a secret is already public. See [secrets.md](secrets.md).
- **commit-msg:** checks the message against Conventional Commits, the same rules CI applies.

## 3. Start the local backend stack

```sh
# First time only: create the local JWT signing key (gitignored, never commit it)
echo '[]' > supabase/signing_keys.json
supabase gen signing-key --algorithm ES256   # answer Y to append

supabase start    # Postgres+PostGIS, Auth, Storage, Studio, Mailpit (~a few GB of images)
```

`supabase start` prints local URLs and keys. Useful ones:
- **Studio** (database UI): http://127.0.0.1:54323
- **Mailpit** (catches sign-in emails): http://127.0.0.1:54324
- **Publishable key** (`sb_publishable_…`): the public key the app uses. Get it any time with `supabase status`.

Then migrate, seed and run the API:

```sh
export $(grep -v '^#' .env.example | xargs)   # local DATABASE_URL, SUPABASE_URL, API_ADDR
# Optional, enables account deletion locally (server-only secret, never commit):
export SUPABASE_SECRET_KEY=$(supabase status -o env | grep '^SECRET_KEY=' | cut -d= -f2 | tr -d '"')
make migrate-up   # applies backend/db/migrations
make seed         # 66 activities, 48 venue categories, every city pack (Tokyo, Austin)
make api          # http://localhost:8080
```

Check that it works:

```sh
curl -s -X POST localhost:8080/issho.v1.HealthService/Check \
  -H 'Content-Type: application/json' -d '{}'
# {"databaseOk":true,"version":"dev"}
```

To call an authenticated endpoint, create a local user and use its token:

```sh
KEY=$(supabase status -o env | grep PUBLISHABLE_KEY | cut -d= -f2 | tr -d '"')
TOKEN=$(curl -s -X POST "$SUPABASE_URL/auth/v1/signup" -H "apikey: $KEY" \
  -H 'Content-Type: application/json' \
  -d '{"email":"dev@example.com","password":"dev-password-123"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["access_token"])')
curl -s -X POST localhost:8080/issho.v1.IdentityService/GetMe \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" -d '{}'
```

(If the user already exists, swap `/signup` for `/token?grant_type=password`.)

## 4. Run the mobile app

```sh
cd apps/mobile
cp .env.example .env          # fill in EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY from `supabase status`
npx expo start                # i = iOS Simulator (needs Xcode), or scan the QR code with Expo Go
```

On a physical phone, swap `localhost`/`127.0.0.1` in **both** URLs for your Mac's LAN IP. Sign in with any email; the 6-digit code arrives in **Mailpit** (http://127.0.0.1:54324). See [../apps/mobile/README.md](../apps/mobile/README.md#signing-in-locally).

## 5. Everyday commands

| Command | What it does |
|---|---|
| `make gen` | Regenerates protobuf (Go, TS, API docs) and sqlc code |
| `make lint` | buf lint, `go vet`, `sqlc vet`, TypeScript typecheck, ESLint (via Turborepo) |
| `make test` | Go tests and mobile unit tests (Jest) |
| `make secrets-scan` | Scans all git history for secrets |
| `make migrate-up` / `make migrate-status` | goose migrations against `DATABASE_URL` |
| `make seed` | Loads the catalogue, venue categories and every city pack (idempotent) |
| `make matrix CITY=tokyo ARGS=-dry-run` | Plans a city's travel-time matrix; without `-dry-run` it calls Google ([google-maps.md](google-maps.md)) |
| `make venues CITY=tokyo ARGS=-dry-run` | Plans venue discovery for a city; same key requirement |
| `make api` | Runs the API |
| `supabase stop` / `supabase start` | Stops or starts the local stack (data persists between runs) |
| `supabase db reset` | **Wipes** the local database. Then run `make migrate-up seed`. |

## 6. Make your first change

Read [contributing.md](contributing.md). In short:
1. Branch from `main`.
2. Make the change. If you edit a `.proto` or `.sql` query, run `make gen`.
3. Run `make lint test`.
4. **Update [ROADMAP.md](ROADMAP.md).** CI requires it for any code change.
5. Commit with a Conventional Commit message (`feat(backend): …`).
6. Open a PR. CI must be green.

## 7. Where to read next
- [architecture.md](architecture.md): how the pieces fit together
- [supabase-hosted.md](supabase-hosted.md): configuring a hosted Supabase project (dev/prod)
- [ROADMAP.md](ROADMAP.md): what's built, what's next, and why
- [generated/api.md](generated/api.md): every API endpoint
- READMEs in each directory: [backend](../backend/README.md), [mobile](../apps/mobile/README.md), [api-client](../packages/api-client/README.md), [proto](../proto/README.md), [data](../data/README.md), [supabase](../supabase/README.md)

## Troubleshooting

| Symptom | Fix |
|---|---|
| `supabase start` → "no space left on device" | OrbStack's VM disk hadn't grown yet, or the Mac's disk is nearly full. Free some space, then retry. Check with `docker run --rm alpine df -h /`. |
| API exits with `load jwks` | Supabase isn't running, `SUPABASE_URL` is wrong, or the signing keys aren't set up (step 3). |
| `unauthenticated: invalid token` | Token expired (they're short-lived), was issued by a different Supabase project, or is HS256 (legacy keys). Sign in again. |
| Pre-commit hook blocks a commit | It found a secret. Remove it from the staged files. If it's a confirmed false positive, add an allowlist entry to `.gitleaks.toml` in a reviewed PR. |
| CI `roadmap` job fails | Your change touched code but not `docs/ROADMAP.md`. Update the status. |
| CI "Generated code is up to date" fails | Run `make gen` and commit the result. |
| `docker run postgis/postgis…` → "no matching manifest for linux/arm64" | That image is amd64-only. Use `supabase start`, or add `--platform linux/amd64` (OrbStack emulates it). |
| CI "pnpm install" → `ERR_PNPM_IGNORED_BUILDS` | A new dependency has an install script. Run `pnpm approve-builds '<pkg>'` (allow) or `'!<pkg>'` (deny) and commit `pnpm-workspace.yaml`. |
| `DeleteAccount` → `failed_precondition` | The API has no `SUPABASE_SECRET_KEY`. Export it (step 3) and restart the API. |
| Sign-in code email never arrives | Locally, emails only go to Mailpit (http://127.0.0.1:54324). After changing `supabase/config.toml` or templates, run `supabase stop && supabase start`. |
| Expo Go says the project needs a newer SDK | The store version of Expo Go lags behind new SDKs. Let the CLI install the right one: see [Running on a device](../apps/mobile/README.md#running-on-a-device). |
| App shows "unreachable" | API not running, or `EXPO_PUBLIC_API_URL` isn't reachable from the simulator or phone. |
