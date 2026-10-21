<p align="center">
  <img src="assets/brand/logo.svg" alt="Issho 一緒" width="380">
</p>

<h3 align="center">Find your people through the things you already love.</h3>

Issho is a new way to build community, rooted in the way we always have. We bring small groups of like-minded people together for real-world activities, so friendships grow from time spent doing what you enjoy.

### Community, without the awkward part

Making friends as an adult is hard because no one is organising it. Issho does the organising. Tell us what you enjoy and when you're free, and we'll suggest plans near you with people who share your interests:

<br>

<p align="center">
  <sub>TOKYO</sub><br>
  <b>Photo walk from Kichijoji station, ending at a café we've already picked.</b>
  <br><br>
  <sub>LONDON</sub><br>
  <b>Sunday wander through Columbia Road flower market, then a roast at the pub around the corner.</b>
  <br><br>
  <sub>MEXICO CITY</sub><br>
  <b>Sunday bike ride down a car-free Paseo de la Reforma, then tacos al pastor in Roma.</b>
  <br><br>
  <sub>MELBOURNE</sub><br>
  <b>Coffee crawl through the CBD laneways, finishing at Queen Victoria Market.</b>
  <br><br>
  <sub>BUENOS AIRES</sub><br>
  <b>Browse the San Telmo Sunday market, then empanadas on Plaza Dorrego.</b>
  <br><br>
  <sub>CAPE TOWN</sub><br>
  <b>Sunrise hike up Lion's Head, then breakfast together in Sea Point.</b>
</p>

<br>

1. **Pick what you enjoy.** Photo walks, board games, climbing, ramen crawls. Start from activities you already like.
2. **Meet a small group.** Join the plans that appeal to you. Once enough nearby people are in, we match you into a small group and find a time and place that works for everyone.
3. **Keep the ones you click with.** Afterwards, privately choose who you'd like to meet again. Matches become a regular **crew**.

It's built for friendship: no profiles to browse, no one-to-one messages, and safety designed in from the start. Every language is welcome, and every city is just data. We're starting in Tokyo.

### One thread, one sun

Issho (一緒) means "together" in Japanese, and the character tells the story:

| | | |
|---|---|---|
| 一 | *one* | **Togetherness.** It all starts with being one. |
| 糸 | *thread* | **Connection.** The thread that ties us to each other. |
| 者 | *person* | **Community.** People who are tied together. |
| 日 | *sun*, inside 者 | **Belonging.** We are all under the same sky. |

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
| [docs/supabase-hosted.md](docs/supabase-hosted.md) | Step-by-step configuration of a hosted Supabase project |
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
| [`CLAUDE.md`](CLAUDE.md), `.claude/skills/` | Instructions for AI coding agents, and the official Supabase agent skills |
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
