<p align="center">
  <img src="assets/brand/logo.svg" alt="Issho 一緒" width="380">
</p>

<h3 align="center">Stop scrolling for plans. We'll make them for you.</h3>

Love film cameras, flat whites and wandering down side streets? Issho finds the handful of people near you who do too, and hands you all the same plan:

> 📸 **Sunday, 10am.** Photo walk from Kichijoji station, ending at a café we've already picked. **5 people. Just say yes.**

- 🗺️ **Plans, not profiles.** No swiping, no endless "so… what should we do?" chats. Real outings, built from what you actually love.
- 👋 **Small groups, real chemistry.** A few people, matched on interests, schedule and language.
- 🔁 **Clicked? Do it again.** After each outing, quietly pick who you'd hang out with again. When it's mutual, Issho brings you back together, and the best groups become **crews**.
- 🤝 **Friends first.** Built for platonic connection, with safety designed in from day one.
- 🌏 **Starting in Tokyo.** Ready for any city.

*Issho (一緒) means "together" in Japanese.*

> 🚧 **Pre-release.** We're building it now: see the [roadmap & status](docs/ROADMAP.md).

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
| [`assets/brand/`](assets/brand/README.md) | Logo, mark and brand colours (with the story behind the kanji) |
| `scripts/` | CI helpers (`check-roadmap.sh`) and the logo generator (`brand/build_logo.py`) |
| `.githooks/` | Git hooks: secret scan (pre-commit) and commit-message lint (commit-msg). Enable with `make setup`. |

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
