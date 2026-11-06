# Keep, renovate or rebuild before the Pilot

Before the Pilot in Da Nang–Hoi An, every part of the repo got one verdict. **Nothing is rebuilt.** Most parts are kept; the ones with the most debt (plans, the schema, walks, moderation, ops and docs) are renovated in place; the parts the Pilot doesn't use are parked or removed. The question came from [Wayfinder: keep, renovate or rebuild before the Pilot](https://github.com/shrujancheruku/issho/issues/411): fast, loosely planned building left enough drift that a fresh start was a real option, and it lost on cost in every area.

## How parts were judged

[Verdict criteria](https://github.com/shrujancheruku/issho/issues/413) set the rules:
- **keep** reaches the bar in ≤2 agent sessions with no red signal;
- **rebuild** needs red domain fit, or red tests and red structure together;
- **renovate** is everything else;
- **cut** is either **remove** (deleted) or **park** (dormant: it compiles and passes its tests, is exempt from the bar, and its RPCs are unregistered).

Each part was scored green, amber or red on tests, structure, domain fit, docs match, defects and refactor load, and costed both ways. The cheaper route wins; breaking changes are free before the Pilot. Audits recommended a verdict; the owner decided it.

A **frozen refactor** counts toward a part's cost only if it fixes a live defect or a Pilot gate needs it. Which frozen tickets survive is decided separately.

The bar is [The hygiene bar](https://github.com/shrujancheruku/issho/issues/414). Its Pilot gates:
- dead-code checks pass;
- handler tests for every in-scope RPC;
- render-and-interaction tests for core-loop screens;
- tests for the admin pages that run the Pilot;
- one end-to-end core-loop test;
- the inventory's doc contradictions fixed;
- no high or critical alerts and the Renovate backlog cleared;
- production secrets only in the secret store;
- the four tracker rules hold;
- a tested restore, Sentry alerts and a rollback path.

Every standard is checked by CI, a hook or the reviewer checklist. Scope came from [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417).

## Verdicts

Costs are agent sessions to reach the bar (renovate / rebuild). Each heading links its audit.

**[Plans and outings](https://github.com/shrujancheruku/issho/issues/421)**

| Part | Verdict | Why | Cost |
|---|---|---|---|
| `internal/plans` | Renovate | Red refactor load; no red fit | 8 / 16 |
| Mobile plans and outings | Renovate | Client rules drift from the server; 3 untested screens | 4 / 8 |
| Admin plan and group pages | Keep | No reds | 1 / 4 |

**[Matching and walks](https://github.com/shrujancheruku/issho/issues/422)**

| Part | Verdict | Why | Cost |
|---|---|---|---|
| Matching | Keep | #406 is a rename, so it doesn't count | ≤2 |
| Walks | Renovate | 20% pass in Da Nang–Hoi An against 70%. Fit is amber: the causes (Japan-tuned kinds and prompt, đ folding, radius and leg rules) are fixable. Needs Da Nang eval cases and a per-area switch | 3–4 / 5–6 |
| Maps | Keep | No reds | 1 / – |
| Geo | Renovate | #290 loses results on rate limits, a live defect | 2 / – |
| Simulator | Park | Swap its Tokyo fixture | – |
| Walk benchmark (`walks/bench`, `walks/claude`, `walk-bench`) | Remove | Drops `anthropic-sdk-go` | 0.5 |

**[People, safety and notifications](https://github.com/shrujancheruku/issho/issues/423)**

| Part | Verdict | Why | Cost |
|---|---|---|---|
| Identity, backend profile | Keep | No reds | 0.5 each |
| Mobile profile | Renovate | Red tests | 1.5 / 3.5 |
| Moderation | Renovate | Red refactor load; blocking is missing | 2.5 / 5 |
| Mobile safety and account screens | Renovate | No tests | 1 / 1.5 |
| Admin Reports, Flags, People, Person | Keep | No reds | 1 / 2 |
| Notifications, push | Keep | No reds | 1, 0.5 |
| Metrics | Renovate | The success bar isn't computable | 2.5 / 3 |
| Interest tagging | Park | – | – |

**[Crews and chat](https://github.com/shrujancheruku/issho/issues/424)**

| Part | Verdict | Why | Cost |
|---|---|---|---|
| Crews backend | Keep | No reds; includes the owner-proposes change | 2 / 4.5 |
| Chat, chat socket, mobile crew and chat screens, Together tab, CrewLeaves | Keep | No reds | 0.25–1 each |
| Admin crew candidates | Build | New with owner-proposes | 0.75 |

**[Catalogue, cities and languages](https://github.com/shrujancheruku/issho/issues/425)**

| Part | Verdict | Why | Cost |
|---|---|---|---|
| Catalogue, Venues curation | Keep | No reds | 1.5, 0.25 |
| `cities` | Renovate | #407; parked editor code inside it | 2.5 / 6 |
| City packs and the Cities page | Keep | Needs the `repoedit` read side | 1.5 / 3.5 |
| Da Nang–Hoi An and Austin packs | Keep | Austin is a test fixture | 0.5 |
| Translation (Gemini), UI strings, email templates, mobile i18n | Keep | No reds; Vietnamese is new Pilot work | 0.25–1.5 each |
| Tokyo pack | Remove | Replace its uses in tests, db-checks and the walk eval first | 1 |
| Cloud Translation, `translate-compare` | Remove | – | 0.5 |
| CityEditor, translation editor and translator role, other UI languages | Park | – | – |

**[Backend platform, schema and API contract](https://github.com/shrujancheruku/issho/issues/426)**

| Part | Verdict | Why | Cost |
|---|---|---|---|
| Platform core, worker, `cmd/api`, `cmd/tool`, admin jobs, the metrics package, `proto/`, API client | Keep | No reds | 0.25–1 each |
| `platform/github`, `repoedit` | Keep (read side) | The UI-string versions and "Apply from main" need them | 0.5 / 2 |
| Schema (migrations, views, queries) | Renovate | Red refactor load: #321, #331 (live defect), #401, #332 | 3 / 10 |

**[Mobile shell, admin shell and site](https://github.com/shrujancheruku/issho/issues/427)**

| Part | Verdict | Why | Cost |
|---|---|---|---|
| Mobile shell | Keep | No reds | 1.5 / 8 |
| Admin shell | Keep | No reds; role check moves off `GetTranslator` | 1 / 3 |
| Site | Renovate | No tests; Tokyo-first copy; legal pages and the waitlist form to come | 2.5 / 3.5 |

**[Infra, CI, docs and tracker](https://github.com/shrujancheruku/issho/issues/428)**

| Part | Verdict | Why | Cost |
|---|---|---|---|
| CI and hooks, Supabase config, scripts and Makefile | Keep | No reds | 0.5–1.5 each |
| Dependency upkeep | Renovate | 28 open alerts (11 high) | 2.5–3 / – |
| Deploy | Renovate | The production cutover (#341) and missing rollback paths count | >2 / 5 |
| Backups | Renovate | No restore of a current dump | 1 / 1.5 |
| Docs | Renovate | None of the inventory's contradictions fixed | 1.5–2 / 3–4 |
| Tracker | Renovate | Nothing checks the four rules | 1.5 / – |

No verdict overrules the rules. Two calls went the rules' way where an overrule was open:
- **The site** stays Renovate rather than Keep, so its form gets a smoke test.
- **Walks' domain fit** is amber, not red, so walks are renovated rather than rebuilt.

## Changes to the Pilot's scope

- `platform/github` and the read side of `repoedit` stay in scope; only the editing side is parked.
- The admin Cities page (apply and discover) is in. CityEditor stays parked.
- The two-activity pairing matching already does stays on. It pairs short activities that don't fill an outing alone. The broader design in [Plans that combine activities](https://github.com/shrujancheruku/issho/issues/408) stays out of the Pilot.

## Settled along the way

- **Parking:** parked code stays where it is. The dead-code check (`deadcode -test` at a pinned recent version, and knip with a config) exempts it by a named list.
- **Vietnamese:** the UI ships a machine-translated `vi` bundle. A Vietnamese reader reviews the core-loop strings into `human/vi.json` before the Pilot, as a Pilot gate. The server's language list offers only English and Vietnamese.
- **Blocking:** the Pilot gets the minimal version, a kept-apart pair plus a button. The full #404 stays frozen.
- **Restore:** the gate needs a restore of a current dump and the backup alert (#239) on. The pre-baseline drill doesn't count.
- **Admin:** the role comes from `GetMe` or a small caller RPC, not `GetTranslator`. Translations and CityEditor leave the nav. Admin errors reach Sentry, and the alert environment is `production`.
- **Web sessions:** #340 is not a Pilot gate.
- **Waitlist:** the bot check is Cloudflare Turnstile. Rate limiting lives in the Go API.
- **Admin test gate:** it covers the Plans and Groups list pages, NewPlan, Flags, Reports' resolve action and Metrics.
- **Tracker rules:**
  - Wayfinder-labelled issues and the Dependency Dashboard are exempt from the one-triage-label rule.
  - The ROADMAP rule covers open issues. "Moved from ROADMAP" provenance lines are allowed; ROADMAP acceptance boxes aren't.
- **Renovate PRs:** #391 (a benchmark-only dependency) and #389 (Postgres 18 backups against a 17 database) are closed.
