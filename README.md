<p align="center">
  <img src="assets/brand/logo.svg" alt="Issho 一緒" width="380">
</p>

<h3 align="center">Find your people through the things you already love.</h3>
<p align="center">好きなことから、仲間に出会う。</p>

Remember when friends just happened? Same team, same class, same corner café. Issho brings that back. Tell us what you love. We'll find a few people nearby who love it too, and a time and place to do it together.

*Tokyo* · **Photo walk** from Kichijoji station, once around Inokashira Pond, ending at a **café** to compare shots.\
*London* · **Flower market** dash on Columbia Road before the stalls sell out, then a **Sunday roast** at the pub.\
*Mexico City* · **Bike ride** down car-free Reforma on a Sunday morning, then **tacos al pastor** in Roma.\
*Melbourne* · **Coffee crawl** through the CBD laneways, three flat whites in, ending at **Queen Victoria Market**.\
*Buenos Aires* · **Antique hunting** at the San Telmo fair, haggling over old maps, then **empanadas** on Plaza Dorrego.\
*Cape Town* · **Sunrise hike** up Lion's Head with headlamps on, then **breakfast** together in Sea Point.

### How it works

1. **It starts with a plan. Pick something you'd do anyway.** Photo walks, bouldering, ramen crawls, board-game cafés. Tag each activity *Love it*, *Like it* or *Open to try*, and **Plans** shows what's on near you, best matches first. The activity breaks the ice, so you don't have to. Like the look of one? Sign up and mark the times you're free. Nothing's booked until a time works for enough of you.

   <picture>
     <source media="(prefers-color-scheme: dark)" srcset="assets/readme/plan-dark.svg">
     <img src="assets/readme/plan-light.svg" width="400" alt="A plan in the app: a photo walk from Kichijoji station, its route drawing itself on a map of Inokashira Park, with a walker strolling it">
   </picture>

2. **Date set. Enough of you said yes. It's on.** When enough people can make the same time, we lock in a date and split everyone into small groups, matched on what you love, a language you share and the group size you like.

   <picture>
     <source media="(prefers-color-scheme: dark)" srcset="assets/readme/date-dark.svg">
     <img src="assets/readme/date-light.svg" width="400" alt="When people can go: a grid of days and times, where Saturday at 2 PM turns gold">
   </picture>

3. **Going. A few people, not a crowd.** Your invite lands in **Together**, under "Needs you". Confirm your place and see who's coming. Photos are only shared with your group. Groups stay small, so everyone gets a word in and you'll remember every name.

   <picture>
     <source media="(prefers-color-scheme: dark)" srcset="assets/readme/going-dark.svg">
     <img src="assets/readme/going-light.svg" width="400" alt="Can you go? Five seats round a table, filling as people say yes; you confirm and see who is going">
   </picture>

4. **On the day. Show up. The activity does the talking.** The group chat opens ahead of time, so you can say hi and spot each other at the gates. Tap "I'm here" when you arrive, and share the outing with someone you trust.

   <picture>
     <source media="(prefers-color-scheme: dark)" srcset="assets/readme/day-dark.svg">
     <img src="assets/readme/day-light.svg" width="400" alt="The group chat on the day: you type and send a message, then Kenji starts typing">
   </picture>

5. **How was it? Pick who you'd meet again. Privately.** Nobody sees your picks. Pick each other, and you're more likely to share a group again. Anyone you'd rather not see again, you won't.

   <picture>
     <source media="(prefers-color-scheme: dark)" srcset="assets/readme/again-dark.svg">
     <img src="assets/readme/again-light.svg" width="400" alt="Who would you meet again? Aoi, Maya and Sam get ticked, privately">
   </picture>

6. **A crew. Strangers on Saturday, regulars by spring.** Clicked with your group? We'll offer you a crew: your own group chat, with a fresh plan picked for you every week or so. It starts once at least three of you say yes, and you can leave any time.

   <picture>
     <source media="(prefers-color-scheme: dark)" srcset="assets/readme/crew-dark.svg">
     <img src="assets/readme/crew-light.svg" width="400" alt="A crew offer: Maya says yes, then you do, and the crew starts">
   </picture>

### Issho (一緒) means "together"

Look closely: the word tells the whole story.

| | | |
|---|---|---|
| 一 | **One** | Every friendship starts with one yes. |
| 糸 | **Thread** | Each outing ties another knot between you. |
| 者 | **Person** | Real people in real places, not profiles on a screen. |
| 日 | **Sun/Day** | The warmth of a day spent out together. |

See you out there.

一緒に行こうか。

[joinissho.com](https://joinissho.com)

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
| [`apps/mobile/`](apps/mobile/README.md) | Expo (SDK 58) app for iOS, Android and the web: Together, Plans and You, in light and dark |
| [`apps/admin/`](apps/admin/README.md) | Admin web app: moderation, plan review, venue curation, metrics, city packs |
| [`site/`](site/README.md) | The marketing site at [joinissho.com](https://joinissho.com) |
| [`packages/api-client/`](packages/api-client/README.md) | Generated TypeScript API client |
| [`proto/`](proto/README.md) | API contract (protobuf), source of truth for types and docs |
| [`data/`](data/README.md) | Activity catalogue and city packs |
| [`supabase/`](supabase/README.md) | Local Supabase stack config |
| [`assets/brand/`](assets/brand/README.md) | Logo and mark, plain and in round blue and white versions, as SVG and PNG; brand colours and the story behind the kanji |
| `scripts/` | CI and deploy helpers, the brand generators (`brand/`: the logo, its round versions, the app icons) and the site's map (`site/build_map.py`) |
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

Expo / React Native · Go (ConnectRPC, pgx, sqlc, goose, River) · Supabase (Postgres + PostGIS, Auth, Storage) · Google Maps Platform · Cloudflare Workers · Sentry · Claude · GitHub Actions · release-please. The reasons for each choice are in the [decision log](docs/ROADMAP.md#4-decision-log).
