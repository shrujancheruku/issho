# Running Issho

What the owner can see and do to run Issho: the admin SPA, the numbers, review and curation, and operations. Moderation's rules are in [safety.md](safety.md). Sources are cited as in the [README](README.md). The code is `apps/admin` and the admin-only services of the backend.

## What v1 decided

### Who runs it

| Decision | Source |
|---|---|
| **One person: the owner**, alone, building with agents. | [Wayfinder: soft launch in Da Nang and Hoi An](https://github.com/shrujancheruku/issho/issues/334) |
| The owner **operates Issho as an individual** until the public launch; a Texas LLC takes over before it. | ADR 0004 |
| **Admins are ordinary accounts with an admin flag**, managed where every other account is: "one place to manage people; no admin list to keep in sync". | Log, 2026-10-23 (owner) |
| Outside **translators** get a role limited to their languages, because admin covers reports and personal data. | Log, 2026-10-26 |
| The admin SPA is a separate web app that **only talks to the API**, never the database. No access layer in front of it. | Log, 2026-10-23, 2026-10-25 |

### What the admin SPA does

Pages, from `apps/admin/README.md`:

| Page | What the owner does there |
|---|---|
| **Plans**, plan detail, new plan, stops editor | Switch a city's **review** on or off; approve, edit, reject or withdraw the matcher's plans; **create a plan by hand**; pick a venue; edit a walk's stops. |
| **Groups**, group review | In a city under review: see the scheduler's proposed date and groups **before anyone is invited**; move or remove a person, change the start, set a meeting point; approve or reject. |
| **Venues** | Review discovered places, approve or reject, search and add a place by name; switch a city to approved venues only. |
| **Cities**, city editor | See packs and what's missing (travel times, venues); apply a merged pack; discover an area's venues; edit a pack through a pull request. |
| **Reports**, **Flags**, **People**, person view, **Crew leaves** | Moderation ([safety.md](safety.md)). |
| **Metrics** | The numbers, below. |
| **Translations** | Correct machine translations; write human versions of UI strings. |

### Decisions about running it

| Decision | Source |
|---|---|
| **The first city can be run by hand.** Review is a per-city switch so the owner controls what opens and who meets "without slowing cities that run on their own". Hand-curating the first city was the stated answer to "not enough nearby users". | Log, 2026-10-24; `docs/architecture.md`, "Known risks" |
| **Plans made by admins open at once**, even under review. | Log, 2026-10-24 |
| **An approved draft's round starts when approved**; a rejected draft rests like a plan that found no group; a rejected group proposal rests its plan for a day. | Log, 2026-10-24 |
| **Admins pick venues from those already found**, or add one by name after it's checked with Google. | Log, 2026-10-24 |
| **Irreversible actions ask first.** | Log, 2026-11-03 |
| **Editor pull requests never name who made them**: the repository is public. | Log, 2026-10-26 (owner) |
| Long admin operations run as **background jobs** the page polls. | Log, 2026-11-04 |

### The numbers

| Decision | Source |
|---|---|
| **The north star is `repeat_attendees`**: of the people who attended in a week, those with two or more outings so far. | Backend README, `MetricsService`; Log, 2026-10-23 |
| Weekly per city: people who went, first-timers, groups that went ahead, places invited / confirmed / attended, average ratings, meet-again picks and mutual pairs, new accounts, crews formed, reports; and the city now (people, open plans, signups waiting, outings to come, active crews). | Backend README |
| **Product analytics (PostHog) was deferred**, along with the consent decisions it needs. PostHog's error capture stays off: one error tracker (Sentry). | Log, 2026-10-20, 2026-10-26 |
| **The Pilot's success bar**: at least 60% of matched groups go ahead; at least 40% of people who attended attend again; no outing fails because of a bug; no safety incident goes unhandled. Checked after two turns of the loop and then every two. Missing the first two "triggers a rethink before more is built". | [What the Pilot is](https://github.com/shrujancheruku/issho/issues/412) |

### Operations

| Decision | Source |
|---|---|
| **The stack**: a Go API and worker on one VM, deployed with Kamal; Supabase for auth and Postgres; Cloudflare in front, hosting the three static sites; Sentry for errors and performance; a nightly database dump to R2, kept 30 days. | Log, 2026-10-20 to 2026-10-26; `docs/deploy.md` |
| **Free tiers until a trigger fires**, with a written table of upgrade points (Supabase, Resend, Sentry, R2), checked at each Pilot checkpoint. Production starts on Supabase Free: the nightly dump is the only backup, and "up to a day of lost data is acceptable for the Pilot". | [Production stack](https://github.com/shrujancheruku/issho/issues/341) (owner) |
| **Every vendor sits behind an interface**; open source preferred, paid or closed only when clearly better. | Log, 2026-10-20 |
| **One failure policy for the worker**: one bad plan, group, crew or notification is logged, reported and counted, and never stops the city. | Log, 2026-11-03 |
| **Production fails fast** on a missing key; a missing key never deletes data. | Log, 2026-11-03 |
| Releases follow Conventional Commits and SemVer through release-please; `main` is protected. | Log, 2026-10-20, 2026-11-03 |
| **Support during the Pilot is a chat group outside the app** (WhatsApp or Zalo). The owner may run it remotely. | [What the Pilot is](https://github.com/shrujancheruku/issho/issues/412) |

### Decided on tickets after the log froze

| Decision | Built? | Source |
|---|---|---|
| **The owner proposes crews** from a candidates page. | No | [crews.md](crews.md) |
| **Metrics shows the Pilot's success bar.** None of its measures can be read off Metrics today. | No | [Metrics computes the Pilot's success bar](https://github.com/shrujancheruku/issho/issues/454) |
| **A waitlist list** in the admin, with an "Invite" action; **invites** added, listed and removed on the People page. | No | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417); [only invited people can join the Pilot](https://github.com/shrujancheruku/issho/issues/477) |
| **The owner is alerted to each report** through Sentry. | No | [safety.md](safety.md) |
| **Budget alerts and quotas** for Gemini and Maps. | No (owner task) | [budget alerts and quotas for Gemini and Maps](https://github.com/shrujancheruku/issho/issues/488) |
| **The API can require a minimum app version**; no over-the-air update is published during the Pilot. | No | [the API can require a minimum app version](https://github.com/shrujancheruku/issho/issues/480) |
| **A tested restore, Sentry alerts and a rollback path** before the first invitee. | No | [Tested restore, Sentry alerts and rollback](https://github.com/shrujancheruku/issho/issues/465) |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| An admin list in config | The admin flag on the account | One place to manage people | Log, 2026-10-23 |
| City packs edited in the database | Through pull requests | The repository as the source of truth | Log, 2026-10-24 |
| Requiring venue approval before any use | A per-city approved-only switch, off by default | Thousands of discovered places would stop every plan | Log, 2026-10-24 |
| Cloudflare Pages; Caddy; Grafana | Workers; kamal-proxy; Sentry only | Redundancy review | Log, 2026-10-20, 2026-10-23 |
| Supabase Pro before real users | Supabase Free with upgrade triggers | Owner's call | [Production stack](https://github.com/shrujancheruku/issho/issues/341) |
| Tokyo as the hosting region | Singapore | Nearest to Vietnam | Same |
| The ROADMAP file as plan, status and decision log | GitHub issues, ADRs and a frozen log | It had become a second tracker | ADR 0001 |
| The in-app city editor and translation editor | Parked for the Pilot | Out of scope | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |
| The matcher's plans opening by themselves in the first city | The owner "seeding and approving plans" by hand in the Pilot | The Pilot is run by hand | [What the Pilot is](https://github.com/shrujancheruku/issho/issues/412) |

## Left open, parked, cut or only planned

- **How one person runs it beyond the Pilot**: moderation speed, support and its languages, on-call, and "what happens when the owner is unavailable" were asked and deferred ([Running the soft launch alone](https://github.com/shrujancheruku/issho/issues/345)).
- **Production was never cut over.** The deployed API runs against the development Supabase project.
- **Product analytics**: undecided.
- **Money**: no pricing, payments or costs per person are recorded anywhere beyond "Paid tier (per-event fee or membership)" in Ideas. Spending so far is free tiers plus billed Gemini and Maps calls (a walk eval of 75 cases cost about $19).
- **In-app support**: none.
- **Local people helping to run a city** (hosts, partner venues): the seeding question asked "how many hosts and partner venues" and was deferred unanswered ([Seeding plan for Da Nang–Hoi An](https://github.com/shrujancheruku/issho/issues/346)). v1 has no role between a member and an admin except translator.
- **The GitHub token the editors use expires on 2027-01-22.**

## Where the code and the documents disagree

- **Metrics and "went ahead".** Metrics counts only groups that went ahead, so it has no denominator for "share of matched groups that go ahead", and it counts a group stuck collecting answers at a finished outing as having gone ahead. Reports made in crews are never counted.
- The admin README says list paging is "not yet checked in a browser".
- `docs/architecture.md` describes the editor flow as how cities change; it is parked.
- The Backend README, admin README and several handlers cite the retired milestone "(M8)".
- The log says the deploy runs as `preview` "until a real production environment exists"; the production decision (Singapore, a separate Google Cloud project) is recorded only on its ticket and in `docs/supabase-hosted.md`.
