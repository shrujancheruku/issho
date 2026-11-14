# Cities

How a city is described, what varies by city, and which cities exist. Sources are cited as in the [README](README.md). The code is `backend/internal/cities`, `backend/internal/geo` and `data/`.

## What v1 decided

### A city is data

| Decision | Source |
|---|---|
| **A city is a city pack**: one JSON file. "Code never hard-codes a city, so adding one means adding data." | `data/README.md` |
| A pack holds: the city's **name**, **time zone**, **currency**, default **languages**, **travel modes** (walk, drive), its **areas**, its **activity overrides** (rename, disable, seasonal months), and optionally its **matching settings**, **timing** (when a morning, afternoon or evening outing starts) and **meeting-place** order. | `data/README.md`; Log, 2026-11-04 |
| **A pack is a matching pool.** Da Nang and Hoi An share one pack, because Hoi An is too small for many plans alone; each person's travel limit still decides who meets whom. | Log, 2026-10-29 (owner) |
| The pack in the repository is **the single source of truth**. Applying it makes the database match: areas and overrides it drops are deleted; an area still in use is refused unless another area `replaces` it, which moves its people, crews and plans. | Log, 2026-10-24, 2026-11-03 |
| Packs are **changed through pull requests**, so every change is reviewed and checked by CI, and every environment gets the same data. Applying after merge is a separate, deliberate step. | Log, 2026-10-24 (owner) |
| A person has **one home area, in one city**. Plans, the feed, crews and metrics are all per city. | `proto/issho/v1/profile.proto`; Backend README |

### Areas

| Decision | Source |
|---|---|
| An **area** is "the place a person calls home and a plan is held in: one of the local government's units in a city pack ... with its official boundary and its official name in the city's language. A city's areas are chosen this way unless there is a very good reason not to." | Glossary; Log, 2026-11-04 (owner) |
| The reason: hand-picked neighbourhoods and stations overlap and leave gaps; official units cover the city, have boundaries anyone can check, and carry the names on addresses. | Log, 2026-10-29 |
| **Boundaries come from OpenStreetMap**, by relation id, with attribution. No government source published Vietnam's 2025 ward boundaries as open data. | Log, 2026-10-29 |
| **Area names are never machine-translated.** They are stored in English plus the real local name. City names are translated, because cities have real exonyms (Tokio). | Log, 2026-10-21 |
| **Matching and travel times work between areas.** Travel time is the area-to-area time, from Google's Routes API, by the fastest of the city's modes. | Glossary; Log, 2026-10-21 |
| Travel times are a **30-day cache**, rebuilt at 25 days. | Log, 2026-10-20 |
| **Walking and driving only; no public transport.** Google returns no transit routes in Japan. Driving times "understate a train city like Tokyo, so treat them as a relative measure of distance, not a promise". | Log, 2026-10-20 |
| Venues are found over **the whole of an area's boundary**, and assigned to the area that contains them. | Log, 2026-10-29 |

### The cities

| Pack | What it is | Source |
|---|---|---|
| **Da Nang–Hoi An** | The first city. 15 wards as reorganised on 1 July 2025 (12 of Da Nang, 3 of Hoi An). Languages: Vietnamese and English. Chosen on 2026-11-04 over Tokyo because the owner will be based there. | `data/citypacks/da-nang-hoi-an.json`; [Wayfinder: soft launch in Da Nang and Hoi An](https://github.com/shrujancheruku/issho/issues/334) |
| **Tokyo** | The original launch city. 7 municipalities of west Tokyo, moved from 12 station areas on 2026-11-04. Marked for removal. | Log, 2026-11-04; ADR 0002 (verdicts) |
| **Austin** | A synthetic pack, "not a launch city": a real car-based city that proves a second city needs no code changes. To become a test fixture. | Log, 2026-10-20; [Road to the Pilot](https://github.com/shrujancheruku/issho/issues/433), re-plan of 2026-11-07 |

### What varies by city today

Everything in the pack: areas, languages, time zone, currency, travel modes, which activities are offered and what they're called, seasons, matching weights and sizes, how many plans per person, outing start times, meeting-place preference, language pairs. Plus two switches an admin sets in the database: **review** (plans and groups wait for approval) and **approved venues only** (Log, 2026-10-24).

What doesn't vary: the catalogue itself, venue categories, the rules of an outing, the code of conduct, the windows for chat, check-in, feedback and reports.

### Decided on tickets after the log froze

| Decision | Built? | Source |
|---|---|---|
| **Rides between stops** are a per-pack setting, with per-pack limits ([plans-and-catalogue.md](plans-and-catalogue.md)). | No | [Rides between stops in Vietnam](https://github.com/shrujancheruku/issho/issues/393) |
| **Wind-down times and search radius** are per pack ("Spain eats later"). | No | [meals and beverages become an optional wind-down](https://github.com/shrujancheruku/issho/issues/347) |
| **The owner opens an area through a pull request** to the pack; for the Pilot the owner chooses which areas open. | Process | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |
| **Unlock**: "an area opening to the people waiting in it, once enough of them have joined." A recommendation said unlock state should live in its own table, not the pack. | No | Glossary; [Waitlist and unlock](https://github.com/shrujancheruku/issho/issues/339) |
| Production runs in **Singapore**, the nearest region to Vietnam. | Not cut over | [Production stack](https://github.com/shrujancheruku/issho/issues/341) |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| Tokyo first | Da Nang–Hoi An first | The owner will be based there | 2026-11-04 |
| Areas as hand-picked neighbourhoods and stations | Local government units, everywhere | Gaps and overlaps; Hoi An divided poorly | Log, 2026-10-29, 2026-11-04 |
| Public transport in travel times | Walk and drive only | No transit data in Japan | Log, 2026-10-20 |
| Venues assigned to the nearest area's centre point | To the area whose boundary contains them | One search reached 800 m; most of a large ward went unsearched | Log, 2026-10-29 |
| A pack edited in the database | Through pull requests | The repository as the single source of truth | Log, 2026-10-24 |
| Two ways to write a pack | One (`cities.Apply`) | The two had drifted | Log, 2026-11-03 |
| Areas could never be removed | `replaces` retires an area | Needed to move Tokyo to municipalities | Log, 2026-11-03 |
| Matching settings writable only by raw SQL | Part of the pack | Block starts are cultural | Log, 2026-11-04 |
| An in-app city-pack editor (admin SPA, opening pull requests) | Parked for the Pilot | Out of scope | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |

## Left open, parked, cut or only planned

- **Da Nang's data is thin where it matters.** Walks compose in 5 of 15 wards. Google's data lags the 2025 ward merger ([Local recommendation sources](https://github.com/shrujancheruku/issho/issues/536)). A ward's centre point "can sit far from where its life is".
- **People outside a pack's areas**: never decided. A person must pick a home area that exists.
- **Someone in two cities**, or moving: never decided.
- **How a second city is chosen and opened**: never decided beyond "adding a JSON file and running three commands".
- **Unlock thresholds**: deferred until the Pilot's numbers exist.
- **Previewing a pack's effect before applying it**: Ideas.
- **OpenStreetMap as a fallback for Google**: one line in Ideas.
- **Hoi An Old Town's timed motorbike ban** and similar local rules have no place in a pack ([Route quality sources](https://github.com/shrujancheruku/issho/issues/538)).
- **Travel times after rides.** Which mode a ride uses, and whether "drive" stands in for a motorbike, was left for the build.

## Where the code and the documents disagree

- `docs/architecture.md:97` calls an area "a neighbourhood or station"; `:16` says "The first city is Tokyo". The Glossary and the log say otherwise.
- `data/README.md` calls `tokyo.json` the "launch city".
- A comment in `cities/pack.go` says "Tokyo's areas are named after stations"; they are municipalities.
- The Glossary's **Area** entry gives Tokyo's wards as its example, and **Meeting place** gives "a station in Tokyo, a café in Da Nang"; the Da Nang pack sets no meeting-place order.
- `docs/architecture.md` describes the admin editor as the way cities change; the editor is parked.
- `docs/supabase-hosted.md` said Tokyo for the production region until the 2026-11-07 correction.
- The 2026-11-04 log row on Tokyo's municipalities ends with an unticked step: apply the pack on `issho-dev`, rebuild the matrix, search venues again.
- The v2 research found two v1 practices break Google's current terms: assigning a venue to an area from its coordinates, and the 30-day travel-time cache ([Terms and law on scraping and reuse](https://github.com/shrujancheruku/issho/issues/537)). v1's own note says the terms "don't state a window for durations" and left a re-check to the owner.
