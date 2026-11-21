# The build order

From [Build order and the first work parent](https://github.com/shrujancheruku/issho/issues/576). It says what is built before the Pilot opens, in what order, and where the owner's own work sits. The tickets are under the work parent, Build the Pilot.

The order is a plan, not a rule of the product. It is reviewed at each re-plan, and a change to it is an ordinary PR to this document.

## What the Pilot's build covers

Everything the spec puts in the Pilot. These wait for the soft launch or later, as their own documents say:

- Apple and Google sign-in ([People](people.md));
- the fee and everything about money ([The stages of launch](launch.md#money));
- waves and the wave proposal ([Running Issho](running-issho.md#the-numbers));
- insiders, and so the Making tab for anyone but the owner ([The stages of launch](launch.md#the-soft-launch));
- the lead search on a cadence ([Leads](leads.md));
- the second moderator ([Safety](safety.md));
- a native end-to-end tool, performance budgets and point-in-time recovery ([Quality](quality.md), [Technical operations](operations.md));
- a second city ([Cities](cities.md)).

## The day-one line

The Pilot opens before four things in its scope are built. They are the last milestone, built while the Pilot runs, in this order:

1. **Crews.** Nobody can be offered a crew before they have been on outings.
2. **The lead search.** Until it exists, leads are added to the leads list by hand; the twelve plans judged in the [prototype](plan-bar.md#what-the-prototype-showed) are the first.
3. **The chat's translate toggle.** A group already shares a language.
4. **City requests.**

Everything else is built before the first member is admitted, the cap on open plans included: at 10 seeds the cap is 4 plans, so it matters most on the first day.

## The milestones

Each is a GitHub Milestone with one integration PR. Each ends in something the owner can walk on dev from a short script, and is about 8 to 15 tickets.

| # | Milestone | Ends with |
|---|---|---|
| 1 | **Foundation** | v1 tagged and deleted; the empty app, admin, API and worker; CI's checks; `policy`, the city clock, the queue and the outside-service pattern; dev deploys on every merge; the bare app runs on the iOS simulator and the Android emulator. Its walk is "CI is green and dev deploys". |
| 2 | **The thin loop** | A person goes from a join link to feedback on a fixture plan. Journey 1 and the season begin. |
| 3 | **Plan-making** | Pieces, vouching, routes, the editor, the accuracy checks, the judge and its eval, plan translation, approval, and the leads list worked by hand. Production stands up. |
| 4 | **The outing in full** | Rounds, the seat order, group approval, the extra invitations, changes to a set outing, the forecast, cancels, attended and no-show, the whole of feedback, mutual picks, the cap on open plans. |
| 5 | **Notifications and chat** | Push, email, quiet hours and every message; rooms and system lines. |
| 6 | **Safety and privacy** | Reports, kept apart, holds, reliability points, standing, consent, deletion, the tombstone, retention, the sign-in log. Journeys 2 and 3. |
| 7 | **Running Issho** | The queue's items and their defaults, alerts, the scorecard, the second sign-in step, the break switch, the moderator's translation. |
| 8 | **The site** | The front page, the public plan pages, the other pages, the waitlist, "Get the app", the policy and terms. |
| 9 | **Release** | The Vietnamese read, the test tracks, the device checklist, Sentry, technical alerts, the restore test, the demo city and the store reviewers' way in, `2.0.0`. The Pilot can open. |
| 10 | **The tail** | The four things after the day-one line. |

- **The thin loop comes before plan-making.** It proves the stack end to end on fixture plans. Plan-making comes straight after because it opens the slowest gate, the 12 standing plans.
- **The thin loop is narrow.** In: one fixture city and two fixture plans, email-code sign-in, admission by join link, the profile at first signup, a signup with times, matching with only its hard rules (3 to 6, a shared time, a shared language, kept apart as an empty input), invitations and yes, going, check-in, and feedback's three questions. Out: notifications, the admin, group approval, rounds on the real clock (the test-only RPC moves time), chat, and any styling beyond the tokens.
- **Admin screens are built in the milestone that needs them**: plan review in 3, group approval in 4, reports in 6. Milestone 7 is what remains.
- **The privacy tests and the season are not milestones.** Each grows in the milestone that first builds its subject ([Quality](quality.md#privacy-as-tests)).
- **Native is checked in every milestone** on local builds ([Quality](quality.md#native-along-the-way)). Nothing goes to EAS or a store before Release.

## Production from milestone 3

Production stands up at the end of milestone 3 so the owner can vouch pieces and make the 12 plans while the rest is built. From that merge production is never reset, and every later migration runs against real pieces and plans. The nightly dump starts with it.

## Screen design

No screen is drawn in the spec ([The app's shape](app-shape.md) settles the tabs and where things live). Each milestone that builds screens has one design ticket for the owner, one milestone ahead: its screens are drawn and signed off before the build tickets that cite them are written. The thin loop is the exception. It is built plain from the app's shape and restyled once its design is signed off.

## The owner's work

Each is a `ready-for-human` ticket under the work parent, placed where it can start earliest.

| Work | Sits in | Why there |
|---|---|---|
| Take v1's API, web build and admin down; set up the Google Cloud project; wipe `issho-dev` | 1 | The dev deploy waits on it |
| The store test tracks | 1 | Store account steps are slow and outside our control |
| The support address answers | 1 | Needs nothing from the build |
| Name the outside person for reports about the owner | 1 | Needs nothing from the build |
| Choose the seeds | 1, due by 9 | Needs calendar time |
| Screen design | One milestone ahead of its screens; the first is in 2 | Above |
| Set up production | 3 | Vouching needs it |
| Vouch the 12 standing plans | Opens when 3 merges, due by 9 | The slowest gate ([Plan knowledge](plan-knowledge.md)) |
| Write the safety runbook | 6 | It describes the tools that milestone builds |
| Draft the policy and terms | 6, before 8's pages | The recipient list is final then |
| Engage the Vietnamese reader | 7 | Lead time before the read |
| The Vietnamese read; the device checklist on real phones | 9 | Both need the finished app |

These cover the gates before the Pilot in [Running Issho](running-issho.md#gates) and the 12 plans in [Plan knowledge](plan-knowledge.md).

## How the tickets are written

- One work parent holds the whole order. Tickets are written in full for the next two milestones only. Each later milestone's tickets are written at the re-plan when the milestone before it merges, so they are written from what the build has learnt.
- A bug found during the Pilot goes to the front, ahead of the tail (the Pilot-bug exception in `docs/agents/issue-tracker.md`).
