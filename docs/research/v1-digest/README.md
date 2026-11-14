# A digest of what v1 decided

Written for [A digest of what v1 decided](https://github.com/shrujancheruku/issho/issues/534), a ticket of [Wayfinder: Issho v2, from scratch](https://github.com/shrujancheruku/issho/issues/532). One document per product area on that map. Each gives a grilling ticket its defaults: v1's answer, where it is recorded, what changed along the way, what was left undone, and where the code and the documents disagree.

Facts only. Nothing here recommends keeping or changing anything.

**Snapshot:** `main` at `d35f3da` (2026-11-14). This branch (`research/v1-digest`) is never merged.

## The documents

| Area | Document | The v2 ticket it feeds |
|---|---|---|
| The core loop | [core-loop.md](core-loop.md) | The core loop |
| Plans and the catalogue, walks, rides and wind-downs | [plans-and-catalogue.md](plans-and-catalogue.md) | What makes a plan good, Plans people make, Where plan knowledge comes from, Building blocks, Routes worth walking |
| Matching | [matching.md](matching.md) | Matching |
| The outing | [outing.md](outing.md) | The outing |
| Crews | [crews.md](crews.md) | Crews |
| Chat | [chat.md](chat.md) | Chat |
| People | [people.md](people.md) | People |
| Safety | [safety.md](safety.md) | Safety |
| Notifications | [notifications.md](notifications.md) | Notifications |
| Cities | [cities.md](cities.md) | Cities, What varies by city |
| Languages | [languages.md](languages.md) | Languages |
| Running Issho | [running-issho.md](running-issho.md) | Running Issho |
| The site | [site.md](site.md) | The site |
| The stages of launch | [stages-of-launch.md](stages-of-launch.md) | The stages of launch |
| Privacy and legal obligations in the product | [privacy-and-legal.md](privacy-and-legal.md) | Privacy and legal obligations in the product |
| The app's shape | [app-shape.md](app-shape.md) | The app's shape |

## How to read them

Every document has the same four sections:

1. **What v1 decided**, in plain words, each with its source.
2. **Reversed or changed along the way**, and why.
3. **Left open, parked, cut or only planned.**
4. **Where the code and the documents disagree.**

## Sources and how they are cited

| Short form | Source |
|---|---|
| Glossary | [`GLOSSARY.md`](../../../GLOSSARY.md) |
| Log, *date* | The row of that date in the frozen [`docs/decisions-2026.md`](../../decisions-2026.md) |
| ADR *n* | [`docs/adr/`](../../adr/). Two files are numbered 0002: "meals" is `0002-meals-are-wind-downs.md` and "verdicts" is `0002-keep-renovate-rebuild-before-the-pilot.md` |
| Backend README | [`backend/README.md`](../../../backend/README.md) |
| A linked issue title | That issue's body or resolution comment |
| A file path | The code, where nothing else records the rule |

Earlier research this digest leans on, all on unmerged `research/*` branches: the feature and knowledge inventory (`research/feature-and-knowledge-inventory`), the tracker inventory (`research/tracker-inventory`) and the eight audits (`research/audit-*`), written on 2026-11-05 and 2026-11-06 against `main` at `4d13143`.

## The state of the tracker at the snapshot

Three things colour every document:

- **Decided is not the same as built.** After the decision log froze on 2026-11-04, decisions went into issues. Many were decided in detail and never built, because [Wayfinder: keep, renovate or rebuild before the Pilot](https://github.com/shrujancheruku/issho/issues/411) froze feature work on 2026-11-05. Each document says which of its decisions exist in code.
- **[Road to the Pilot](https://github.com/shrujancheruku/issho/issues/433) stopped at the start of its phase 1.** Phase 0 (groundwork) is done. The phase 1 cuts (remove the Tokyo pack, the walk benchmark and Cloud Translation; park tagging and the editors; English and Vietnamese only) are open tickets, so everything they would remove is still in the repo. Phases 2 to 4 hold decisions recorded on tickets, with nothing built.
- **Nobody has used v1.** There are no real users and no production data. Production was never cut over; `api.joinissho.com` runs against the `issho-dev` Supabase project.
