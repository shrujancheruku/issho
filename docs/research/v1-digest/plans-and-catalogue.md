# Plans and the catalogue, walks, rides and wind-downs

What a plan is made of and where its content comes from. Who gets grouped and when is in [matching.md](matching.md); what happens once a plan has a date is in [outing.md](outing.md). Sources are cited as in the [README](README.md).

## What v1 decided

### The plan

| Decision | Source |
|---|---|
| A **plan** is "an outing people can sign up for, before it has a date or a group: an activity or a set of activities in an area". | Glossary |
| A plan is **a 2–4 hour outing around one or two shared activities**. "A temple plan that is one temple, or a photo walk that is a station, isn't worth going to. Quality matters more than the number of plans." | Log, 2026-10-22 |
| A plan belongs to **one area** and has **one or two languages** ([languages.md](languages.md)). | Backend README |
| A plan **names its venues when it opens**, so people sign up for a concrete outing. Approved venues are picked first, rejected ones never, otherwise at random "so one café isn't in every plan". The groups of an outing share the venues. | Log, 2026-10-22; Backend README |
| A plan is open in **rounds**: a 14-day window, signups for the first 7 days. People not placed roll over to the next round. A plan closes as `done`, `no_group` or `withdrawn`. A plan that closed without a group can't be suggested again for 14 days. | Log, 2026-10-22; Backend README |
| **Titles come from a template** ("X, then y"), composed in English and translated. Area names are left out of titles, because machine translation mangles proper nouns. A model may write titles later behind the same field. | Log, 2026-10-22 |
| **Plan descriptions wait for model-written text.** A template paragraph would only repeat the title, timeline and map. | Log, 2026-10-22 |
| A plan screen shows the **steps**: each activity with its venue's name, short address and Maps link fetched live from Google, the walk between them, and a map. | Backend README, `GetPlan` |
| A closed plan stays viewable for **14 days** with why it closed, so a push or a link about it explains what happened. | Backend README, `GetPlan` |
| **Admins can create a plan by hand**; it opens at once. A city can be put **under review**, where the matcher's plans are drafts an admin approves, edits or rejects. | Log, 2026-10-24 |

### The catalogue

| Decision | Source |
|---|---|
| One **global catalogue of 66 activities in 8 categories** (sports 20, culture 11, outdoors 8, food 7, games 7, creative 6, learning 4, wellness 3). Each has: indoor or not, energy, cost level, conversational or not, skill-dependent or not, minimum and maximum group size, hours, the parts of the day it suits, and the venue categories it can happen at. | `data/activities.json`; `data/README.md` |
| **Broad activities: 230 were merged into 66.** Owner's feedback from a device test: too many overlapping options would lose people in onboarding, and broad activities pool interest per plan. Specifics (a waterfall hike, sake tasting) move into each plan's description. | Log, 2026-10-21 |
| **Each sport stays its own activity.** "Tennis and basketball aren't interchangeable." "Try a new sport" was dropped: that is what "open to try" means. | Log, 2026-10-21 |
| Merges are **data** (`replaces` in the catalogue), applied when seeding, which moves people's preferences to the surviving activity. | Log, 2026-10-21 |
| Activities link to **our own venue categories** (48 of them), which map to Google place types with a search radius. This keeps the catalogue provider-neutral. | Log, 2026-10-20 |
| A **city can rename, disable or set seasonal months** for an activity. Da Nang–Hoi An disables Japanese arts, anime, skiing and seasonal viewing; renames temples to "Pagodas and temples", sento to "Spa or massage" and language exchange to "Vietnamese–English exchange"; and limits beach days to April–August. | `data/citypacks/da-nang-hoi-an.json` |
| The catalogue is stored **in English only** and translated on demand. | Log, 2026-10-20 |

### Venues

| Decision | Source |
|---|---|
| **Google Maps Platform** is the source of venues, chosen over OpenStreetMap for venue data (hours, ratings) and for having no routing servers to run. | Log, 2026-10-20 |
| **Only Google place ids are stored**, one row per place and category. Names, addresses and locations are fetched live in each displayed language. | Log, 2026-10-20 |
| **Discovery** runs a Nearby Search per area and venue category and keeps places whose *primary* type matches. It keeps the 20 most popular per search, splits an area's search where a page comes back full, and stops at a budget of 4,000 requests. | Log, 2026-10-29; Backend README |
| **Curation is per place.** Every venue starts as a candidate. An admin approves or rejects; names suggesting a snack bar, hostess bar or tattoo studio are flagged. A city can be switched to **approved venues only** (off by default: Tokyo had 4,424 discovered places). Admins can add a place by name. | Log, 2026-10-24 |
| **Opening hours are checked only when a round is about to be scheduled**, because hours are Google's dearest field. The venue must be open for the activity's own hours. A venue closed for good withdraws the plan. | Log, 2026-10-22, 2026-11-04 |
| Venue quality comes from **popularity ranking and primary type only**. Ratings and "good for groups" are pricier fields and unused. | `docs/architecture.md`, "Known risks" |

### Walks

| Decision | Source |
|---|---|
| A **walk** is "a plan whose stops are all within walking distance of each other". Six activities are walking themes: city walk, seasonal viewing, café hop, food tour, temples, photography. | Glossary; `data/activities.json` (`walk_theme`) |
| A walk is an **itinerary**: a meeting place, 4–7 stops on the theme, and a place to finish, each with minutes there and a note. | Log, 2026-10-22 |
| **Walks are composed automatically and open without review** when they pass the checks. Manual review is the fallback if quality falls short. | Log, 2026-10-22 |
| **A model proposes, code checks.** Gemini, without grounding, names places from its own knowledge; Places and Routes then check each one exists, is the right kind, is near enough and that the legs are walkable. Nothing from Google goes to the model: Google's terms forbid creating content from Maps content, and grounded answers may only be shown to the person who asked. | Log, 2026-10-22 |
| **A place must match the model's name**, not just its kind and area. A doubtful match counts as not found: "leaving out a real place costs less than a wrong note." | Log, 2026-10-23 |
| The default limits: 2 attempts, every place within 3 km of the area's centre, legs of at most 20 minutes, 100 minutes of walking in total. A bad stop is dropped; a bad meeting place or finish rejects the walk; fewer than 3 usable stops rejects it. | [Walk eval in Da Nang–Hoi An](https://github.com/shrujancheruku/issho/issues/338); [Rides between stops in Vietnam](https://github.com/shrujancheruku/issho/issues/393) |
| **A walk that fails isn't opened**, and its area and activity rest for 7 days. | Log, 2026-10-22 |
| An admin can **edit a plan's stops** (3 to 10 places). The admin's order is kept and the result is saved even when it breaks the composer's rules, with warnings. This is what "route curation" came to mean. | Log, 2026-10-24 |
| A walk's quality bar is the **live eval: at least 70% of cases compose**. | [What the Pilot is](https://github.com/shrujancheruku/issho/issues/412) |

### Meeting places

A **meeting place** is "where an outing's group gathers when its first activity has no venue of its own". Each city sets its own order of preference in its pack; the default is a station within 1 km, then a park within 1.5 km (Glossary; Backend README).

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| Claude (Haiku) for tagging and plan text | Gemini for tagging and walks | Owner: same Google Cloud project, billing and client as Maps | Log, 2026-10-21 |
| 230 narrow activities | 66 broad ones, sports kept separate | Onboarding overload; pooling interest | Log, 2026-10-21 |
| A venue per group | Venues named by the plan | People sign up for a concrete outing | Log, 2026-10-22 |
| "A non-conversational activity gets a food follow-up" (a rule) | One or two activities of any kind, **by score**; only short activities are paired, with a small bonus | Owner wanted fuller outings preferred without forcing them. Simulated, pairing didn't fill more plans. | Log, 2026-10-26 |
| A pair's appeal = its better half | The time-weighted average of its parts | With "the better part", every plan became a pair (50 of 50 simulated) | Log, 2026-10-26 |
| Fixed opening-hours checks (at the start and 90 minutes in) | Open for the activity's own hours | A three-hour activity's venue could close mid-way | Log, 2026-11-04 |
| "Route curation" as overriding travel times | Editing a plan's stops | Stops and venues are what people see | Log, 2026-10-24 |
| **Meals are activities** ("Lunch or dinner out", "Brunch or breakfast") | **Meals and beverages are wind-downs**: optional, after the activities, chosen once the date is set, by the time of day, at a venue nearby that's open. Not something people sign up for, check in to or give feedback on. Food-centred outings (food tour, cooking class, barbecue, drinks, café walk) stay activities. | Food is "the social glue around an outing rather than its point"; nobody should be left out of a plan for not liking dinners | ADR 0002 (meals), 2026-11-05 |
| Walking only between stops | **Rides** allowed per city: on for Da Nang–Hoi An, off for Tokyo. Defaults: 15 minutes a ride, 45 minutes getting around in total; a leg of 10 minutes or less stays a walk. The plan is anchored by its meeting place, and stops may be in neighbouring areas. Legs read "8 min walk" or "12 min ride"; nothing about who drives. | People in Da Nang rarely walk far; walks too long sank 9 walks in the eval | [Rides between stops in Vietnam](https://github.com/shrujancheruku/issho/issues/393), 2026-11-05 |
| A walk benchmark (Gemini against Claude, blind judges) to pick the model | Benchmark marked for removal; the live eval stays | The Pilot doesn't need it | ADR 0002 (verdicts) |

## Left open, parked, cut or only planned

- **Walks don't work in the first city.** On 2026-11-05, 15 of 75 composed in Da Nang–Hoi An (20%): 10 of 15 wards composed none. Causes found: the model names chain-café branches as meeting places (28 failures); Vietnamese place names not found (đ doesn't fold to d; no Vietnamese generic words such as chùa, chợ, đình); the 3 km radius is too tight for wards merged in 2025; legs over 20 minutes. The fix was planned, not built ([Walks that work in Da Nang–Hoi An](https://github.com/shrujancheruku/issho/issues/452)).
- **Rides are decided and unbuilt.** No code allows a ride between stops. Which routing mode computes a ride was left "for the build"; Google's two-wheeler mode covers Vietnam at the Enterprise price ([Two-wheeler routing on Google Maps in Vietnam](https://github.com/shrujancheruku/issho/issues/394)).
- **Wind-downs are decided and unbuilt.** The detailed brief (kind by start time: before 11:00 brunch, to 15:00 lunch, to 17:30 a beverage, then dinner; nearest fitting venue within 800 m; none if nothing fits) was frozen and moved to Ideas ([meals and beverages become an optional wind-down](https://github.com/shrujancheruku/issho/issues/347)).
- **A curated list of activities that go well together** (a model drafts, the owner approves) instead of rules alone: "some pairs the rules allow read oddly (a fun park, then meditation)". About 380 pairs pass the rules. Frozen, in Ideas ([a curated list of activities that go well together](https://github.com/shrujancheruku/issho/issues/348)).
- **Plans that combine two popular activities** (the owner's example: pickleball then beer). Asked, never answered ([Plans that combine activities](https://github.com/shrujancheruku/issho/issues/408)).
- **Nine activities can't name a place**, because Google has no place type for them: cycling, bouldering, martial arts, archery, skiing, cooking class, anime, escape rooms, photography. Their plans get a meeting place only. A fix by text search was briefed and frozen ([activities with no venue type name a place](https://github.com/shrujancheruku/issho/issues/357)).
- **Model-written titles and descriptions** for ordinary (non-walk) plans: planned since 2026-10-22, never built.
- **Interest tagging** (free text to suggested activities) is built and parked for the Pilot.
- **No per-area switch for walks.** "Walks only in areas that pass the eval" can't be expressed; a pack can only disable an activity city-wide.
- **Plans people make**: v1 has none. Only the matcher and admins make plans.
- **A re-check of Google's caching terms** before launch was left to the owner ([Re-check Google Maps caching terms before launch](https://github.com/shrujancheruku/issho/issues/359)). The v2 research found v1's area assignment from Places coordinates and its 30-day travel-time cache break Google's current terms ([Terms and law on scraping and reuse](https://github.com/shrujancheruku/issho/issues/537)).

## Where the code and the documents disagree

- **Meals.** The Glossary and ADR 0002 (meals) say a meal is "only ever a wind-down, never an activity of its own". `data/activities.json` still has `dinner-meetup` and `brunch` as activities, people can pick them in onboarding, and matching pairs them.
- **Rides.** The Glossary defines **Ride** and says a plan with a ride is not a walk. No code knows about rides.
- **Meeting places.** The Glossary says a café is Da Nang's preference. The Da Nang–Hoi An pack sets no `meeting_places`, so it gets the default (a station within 1 km first, then a park). Listing cafés first was part of the unbuilt wind-down brief.
- **Follow-ups.** Backend README, matching step 2: "A non-conversational activity gets a food follow-up". The code pairs by score (Log, 2026-10-26).
- **Durations.** Backend README says "activities have no stored duration"; activities have had `hours` since the 2026-11-04 opening-hours change.
- **Walk limits inside the code.** The title message says "at most 60 characters" and the check is over 70; the leg limit is 20 minutes and the feedback text given to the model says "under 15 minutes" (`backend/internal/walks/walks.go`).
- **Tuned for Japan.** The walk prompt gives Tokyo examples and asks for "a station or a cafe right by a station"; place kinds include station, tea house and shrine (`backend/internal/walks/prompt.go`, `walks.go`).
- **The area.** `docs/architecture.md:97` calls an area "a neighbourhood or station"; the Glossary says a local government unit ([cities.md](cities.md)).
