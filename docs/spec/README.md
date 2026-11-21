# The v2 spec

What Issho v2 is, decided on the [v2 map](https://github.com/shrujancheruku/issho/issues/532) and gathered here by [Write the spec documents](https://github.com/shrujancheruku/issho/issues/577). Each document states the decisions of one product area, with every later amendment folded in, and links the tickets it came from. The words are those of the root [`GLOSSARY.md`](../../GLOSSARY.md).

The documents state decisions only. The reasoning, and what v1 did, is in the linked tickets.

## The documents

| Document | Covers |
|---|---|
| [Who Issho is for and what it promises](who-issho-is-for.md) | The person, the problem, the promise, who makes plans, what Issho is not |
| [The core loop](core-loop.md) | The eight steps from seeing a plan to going again, where the loop breaks, the north-star number |
| [The plan bar](plan-bar.md) | Accurate and compelling, who judges, rated poorly, how the bar is measured |
| [Plans](plans.md) | Who makes plans, suggestions, a plan's states and record, what a maker sets, edits, a maker leaving |
| [Plan knowledge](plan-knowledge.md) | Vouching, sources, renewal by people who went, re-checks, what a city needs to open |
| [Pieces](pieces.md) | The four piece types, plans from pieces, doubted and retired pieces, like-for-like swaps |
| [Routes](routes.md) | Stretches, legs, walks, rides, the limits, how people get around |
| [Leads](leads.md) | The lead search, outlines, the leads list, suggestions beside search leads |
| [What varies by city](city-settings.md) | The three layers, the ten city settings, the calendar |
| [Cities](cities.md) | Areas, what a city is, its states, requests, choosing and opening the next city, closing one |
| [Matching](matching.md) | The rules, the seat order, rounds, how many plans are open, the got-an-outing share |
| [Outings](outings.md) | States, seats and stages, invitations, check-in, changes, attended and no-show, feedback |
| [Crews](crews.md) | Crew offers, crew outings, leaving and closing, the name |
| [Chat](chat.md) | Rooms, what can be sent, system lines, translation, what a moderator reads, retention |
| [People](people.md) | Sign-in, the account and profile, join links, guests, finding plans, the code of conduct, standing |
| [Safety](safety.md) | Reports, kept apart, holds, the moderator, late cancels and no-shows, flags, the tombstone |
| [Notifications](notifications.md) | Push and email, message types, quiet hours, every moment that sends a message |
| [Languages](languages.md) | Display and spoken languages, a group's language, the app's text, plan text, readers |
| [Running Issho](running-issho.md) | Staff roles, the admin, the queue, alerts, the safety runbook, the scorecard, gates |
| [The site](site.md) | One address, the front page, ways in, the other pages |
| [Privacy and legal obligations in the product](privacy.md) | Consent, deletion, the tombstone's lifetimes, retention, the recipient list, the policy and terms |
| [The app's shape](app-shape.md) | The tabs, Together and "Needs you", the outing as the plan's page, Making |
| [The stages of launch](launch.md) | Pilot, soft launch, public launch, the fee, a second city |
| [Where v2 lives and what carries over](repo-and-carry-over.md) | The repo, v1's code and services, accounts, versions, porting |
| [Architecture and stack](architecture.md) | What v2 is built from, the modules and their seams, models, outside services |
| [Technical operations](operations.md) | Hosting, environments, backups, alerts, cost, releases, the reviewers' way in, the sign-in log, retention |
| [The quality bar and test strategy](quality.md) | The kinds of test, the season, model evals, privacy as tests, what CI requires, when a ticket is done, review |

## Amending the spec

From the merge of these documents, they are the spec.

- An amendment is a PR that edits the document, worked from an issue.
- It gets an ADR in [`docs/adr/`](../adr/) only when it is hard to reverse, surprising without context and a real trade-off.
- A change to a term edits `GLOSSARY.md` in the same PR.
- Closed wayfinder tickets are never edited again. They stay as the record of why.

## Research and prototypes

Linked, never merged. Each is on its own branch.

| Ticket | Where |
|---|---|
| [A digest of what v1 decided](https://github.com/shrujancheruku/issho/issues/534) | [`research/v1-digest`](https://github.com/shrujancheruku/issho/blob/research/v1-digest/docs/research/v1-digest/README.md) |
| [Comparable products](https://github.com/shrujancheruku/issho/issues/535) | Linked from the ticket |
| [Local recommendation sources](https://github.com/shrujancheruku/issho/issues/536) | Linked from the ticket |
| [Terms and law on scraping and reuse](https://github.com/shrujancheruku/issho/issues/537) | Linked from the ticket |
| [Route quality sources](https://github.com/shrujancheruku/issho/issues/538) | Linked from the ticket |
| [A prototype of good plans in Da Nang–Hoi An](https://github.com/shrujancheruku/issho/issues/542) | [`prototype/good-plans-da-nang-hoi-an`](https://github.com/shrujancheruku/issho/blob/prototype/good-plans-da-nang-hoi-an/docs/prototypes/good-plans-da-nang-hoi-an.md) |
| [The app's shape](https://github.com/shrujancheruku/issho/issues/560) | [`prototype/the-apps-shape`](https://github.com/shrujancheruku/issho/blob/prototype/the-apps-shape/docs/prototypes/the-apps-shape.md) |
| [What the v2 spec asks of the stack](https://github.com/shrujancheruku/issho/issues/571) | [`research/v2-stack-asks`](https://github.com/shrujancheruku/issho/blob/research/v2-stack-asks/docs/research/v2-stack-asks.md) |
| [The v2 glossary and domain model](https://github.com/shrujancheruku/issho/issues/569) | Landed as the root `GLOSSARY.md` |
| [Architecture and stack](https://github.com/shrujancheruku/issho/issues/572) | [`research/vertex-and-forecast`](https://github.com/shrujancheruku/issho/blob/research/vertex-and-forecast/docs/research/vertex-and-forecast.md), [`research/on-device-translation`](https://github.com/shrujancheruku/issho/blob/research/on-device-translation/docs/research/on-device-translation.md) |

## Not in the spec yet

The build order is still an open ticket on the map. Its "Not yet specified" section lists what no ticket has settled: per-screen design, money in detail, filling the waitlist, insiders as a role, photos of an outing, identity verification at the public launch, place names in another script, and the questions for the lawyer.

One gap opened while gathering: a guest now belongs to a member's signup to a plan ([People](people.md#guests)), which removed the route by which a crew outing's guest was voted into the crew. How an open crew gains a crew member is not yet specified.

## Readings made while gathering

Ten conflicts between resolutions were found while gathering and settled by the owner on 2026-11-21; they are listed, with the answers, in [Write the spec documents](https://github.com/shrujancheruku/issho/issues/577). The documents state the answers.

Separately, wording the documents had to settle to use the glossary's terms:

- Matching's "an outing is never set less than 3 days after its invitations go out" is written as the outing's date being at least 3 days after its invitations, since "set" now means 3 yeses.
- Crews used "minding", which is not a v2 term. "Nobody minded anyone" is written as nobody being kept apart; leaving with "made me uncomfortable" is written as the crew-exit flag in Safety.
- The rules for changing a set outing, decided in What varies by city, are in [Outings](outings.md#changing-a-set-outing).
