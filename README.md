# Issho (一緒)

Issho proposes real-world outings to small groups of nearby people who share interests, for example *a Sunday photo walk that ends at a café, with 5 people near you*. After each outing, people privately pick who they'd like to meet again. Mutual picks shape future groups and recurring **crews**. The app is platonic and location-agnostic. The first city is Tokyo.

**Status:** pre-release, milestone M1 (foundations). See the **[build plan & status → docs/ROADMAP.md](docs/ROADMAP.md)**.

## Documentation

| Doc | Read it for |
|---|---|
| [docs/onboarding.md](docs/onboarding.md) | **Start here.** Set up a machine from scratch and run everything locally. |
| [docs/ROADMAP.md](docs/ROADMAP.md) | Full build plan, what's done and what isn't, and the decision log. Updated with every change. |
| [docs/architecture.md](docs/architecture.md) | System design, request lifecycle, data model, code generation |
| [docs/generated/api.md](docs/generated/api.md) | API reference, generated from the protos |
| [docs/contributing.md](docs/contributing.md) | Workflow, Conventional Commits, releases, testing, common tasks |
| [docs/secrets.md](docs/secrets.md) | Where secrets live and how leaks are prevented (**public repo**) |
| [SECURITY.md](SECURITY.md) | Reporting vulnerabilities |

Each directory has its own README:

| Path | What |
|---|---|
| [`backend/`](backend/README.md) | Go API (ConnectRPC), migrations, seeding tool, tests |
| [`apps/mobile/`](apps/mobile/README.md) | Expo (SDK 58) iOS/Android app |
| [`packages/api-client/`](packages/api-client/README.md) | Generated TypeScript API client |
| [`proto/`](proto/README.md) | API contract (protobuf), source of truth for types and docs |
| [`data/`](data/README.md) | Activity catalogue and city packs |
| [`supabase/`](supabase/README.md) | Local Supabase stack config |
| `scripts/` | CI helper scripts (e.g. `check-roadmap.sh`) |
| `.githooks/` | Pre-commit secret scan (enable with `make setup`) |

## Quick start

Prerequisites: Go, Node 24, pnpm, OrbStack (Docker) and the Supabase CLI. Details are in [onboarding](docs/onboarding.md).

```sh
make setup                                   # deps + secret-scanning git hook
echo '[]' > supabase/signing_keys.json && supabase gen signing-key --algorithm ES256   # first time only
supabase start
export $(grep -v '^#' .env.example | xargs)
make migrate-up seed api                     # API on http://localhost:8080
cd apps/mobile && cp .env.example .env && npx expo start
```

## Common commands

`make gen` · `make lint` · `make test` · `make secrets-scan` · `make migrate-up` · `make seed` · `make api`

## Stack

Expo / React Native · Go (ConnectRPC, pgx, sqlc, goose, River) · Supabase (Postgres + PostGIS, Auth, Storage) · Google Maps Platform · Claude · GitHub Actions · release-please. The reasons for each choice are in the [decision log](docs/ROADMAP.md#4-decision-log).
