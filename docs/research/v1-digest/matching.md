# Matching

Who is put in a group with whom, and when. Sources are cited as in the [README](README.md). The fullest single description is the "Matching" section of the Backend README; the settings are `matching.DefaultConfig` in `backend/internal/matching/config.go`.

## What v1 decided

### The order of work

Matching runs **plan first, time second** (Log, 2026-10-21): suggest plans without a date, rank them for each person, then schedule a plan once its signups can make a good group. The engine does no input or output; the worker loads everything and stores the results. It runs for each city every hour: schedule first, then suggest, because scheduling frees room under the city's cap for new plans (Log, 2026-10-22).

### Suggesting plans

| Decision | Source |
|---|---|
| **Pools are per meeting area, not fixed clusters.** Every area is a candidate meeting place. A person joins each area they can reach within their travel limit, in each language they speak fluently. Clusters would split neighbours either side of a boundary. | Log, 2026-10-21 |
| Travel between areas uses the **fastest of the city's modes** (walk or drive). "Someone who'd walk 20 minutes won't drive 25." | Log, 2026-10-21 |
| A plan is suggested for an activity with a venue in the area and an **audience of at least 6** people who want it and are usually free at a part of the day it suits. | Backend README; `MinAudience` |
| Demand is weighted **love 2, like 1, open to try 0.5**, times 0.3 if the activity is over a person's budget. | Backend README; `PrefValues`, `OverBudget` |
| Suggestions are **concentrated**: at most 5 plans per pool, the same activity in a shared language at least 20 minutes apart, at most **one open plan per 10 people**. Without this the simulator made 297 plans for 500 people; signups spread thin and 30% of groups were pairs. With it: 50 plans, 7 pairs in 78 groups. | Log, 2026-10-21 |
| Only activities **in season** in the month the round starts are suggested. | Backend README |

### The feed

Every open plan in the person's city is shown, in **levels**: top (they love it), good (like), maybe (open to try), low (no preference or too far), not for you (they don't speak its language: greyed out, can't sign up). Within a level: interest, momentum (signups so far), fit with their usual availability, then distance. A map shows all nearby plans coloured by level (Log, 2026-10-21; Backend README).

A plan that people they get on with signed up for moves up a level, and says so **as a count**. Only mutual picks count: counting one-sided picks "would let someone follow another person's signups" (Log, 2026-10-23).

### Forming groups

| Decision | Source |
|---|---|
| **Group size targets** follow each person's preference: small about 4, medium about 6, large about 8. | `SizeTargets`; `proto/issho/v1/profile.proto` |
| A plan is **ready** when a group reaches its target size. Groups below the preferred size wait for the next round. At the **signup deadline** every group that can form goes ahead. | Log, 2026-10-21 |
| For each possible occasion in the window, groups are formed from the signups free then; the best occasion wins, ties to the earliest. Occasions start 2 days out. | Backend README |
| **Over-invite inside the group**: invite the target plus 2; confirm at the target. Inviting leftovers afterwards "almost never happened". | Log, 2026-10-21 |
| Outings start at a set time for their part of the day: **10:00, 14:00 or 18:30** local. A city can change these ("an 18:30 dinner in Tokyo, 21:00 in Spain"). | Log, 2026-11-04 |
| A person is **never double-booked**: an outing they're going to blocks its occasion in their other signups. | Log, 2026-10-21 |

### Rules that are never broken

- **At least 2 people.** A city can make 3 strict.
- **Kept-apart pairs are never grouped** ([safety.md](safety.md)).
- **Same-gender-only is respected** for those who ask for it.
- **Everyone speaks the plan's language** (its primary or its secondary). This is the only language rule (Log, 2026-10-21).

### Preferences, weighed in a score

"The engine avoids them when it can but would rather go ahead than leave someone out" (Backend README).

| Preference | Weight | Source |
|---|---|---|
| Interest in the activity | 3 | `Weights` |
| **Nobody the only woman or the only man.** Nonbinary and unspecified are exempt: "requiring two nonbinary people per group would leave most nonbinary users unmatched." | 2 | Log, 2026-10-21 |
| Mutual picks together. A link's strength is the **lower of the two people's pick return rates**, with a prior of two returned picks, so "someone who picks everyone can't buy extra time with anyone". | 2 | Log, 2026-10-23 |
| Clusters of friends | 1.5 | Log, 2026-10-23 |
| Groups of at least 3, or the activity's own minimum | 1 | Log, 2026-10-21 |
| Fit with size preference | 1 | `Weights` |
| Newcomers placed: only a newcomer *majority* is marked down | 1 | Log, 2026-10-23 |
| Reliability: check-ins over confirmed places at finished outings, with a prior of two | 1 | Backend README, `CheckIn` |
| Momentum | 0.5 | `Weights` |

Every setting can be overridden per city in its pack; unknown keys are rejected (Log, 2026-11-04).

### What matching never uses

- **Display languages.** Only spoken languages count (Log, 2026-10-20).
- **Age.** It is collected (birth year) and shown to admins reviewing a group, and no rule or weight reads it.
- **A person's energy preference** (calm, moderate, active). Onboarding asks for it; the engine's `User` has no such field. An activity's own energy is used only to order two activities in one plan (`backend/internal/matching/matching.go`).
- **Ratings.** Plan and group ratings are "stored but not used yet" (`docs/architecture.md`, "Known risks").
- **Learner languages.** A person can list a language they're learning; "learner data unused" (Log, 2026-10-21).

### Too few people

A plan that reaches its deadline without a group closes as `no_group`; the people waiting are told and not moved (Log, 2026-10-22). It rests for 14 days. In a city under review, the scheduler's groups are **proposals** an admin approves, edits (move or remove a person, change the start) or rejects before anyone is invited (Log, 2026-10-24).

### How it was checked

A **simulator** runs the whole flow on 500 synthetic people per city and fails on any rule violation. It doesn't model rounds, the waiting list, relations, suspensions, review or notifications (Log, 2026-11-04). Its people "are guesses, so its numbers show the engine works, not that the density or conversion is enough" (`docs/architecture.md`).

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| Weekly proposals with accept or decline | Plan first, time second | Density | Log, 2026-10-21 |
| A hard minimum of 3 | 2 hard, 3 preferred | Owner: a smaller outing beats none | Log, 2026-10-21 |
| "Nobody the only woman or man" as a rule | A preference | Owner: better to invite someone into an unbalanced group than leave them out | Log, 2026-10-21 |
| Fixed area clusters | Pools per meeting area | Clusters split neighbours | Log, 2026-10-21 |
| Invite leftovers as extras after assembly | Over-invite inside the group | The simulator showed 0 extras | Log, 2026-10-21 |
| At most 2 people who know each other | No cap; clusters of friends score highest | Regulars are the north star | Log, 2026-10-23 |
| Public transport in travel times | Walking and driving only | Google returns no transit routes in Japan | Log, 2026-10-20 |
| Block starts as constants | Per-city timing in the pack | Block starts are cultural | Log, 2026-11-04 |

## Left open, parked, cut or only planned

- **Two-language groups** are "not an MVP feature, only designed for" (Log, 2026-10-21): pools exist only for pairs a city lists (none by default), and are discounted by half.
- **How many plans a city gets** (`users_per_plan`): simulated, never chosen ([core-loop.md](core-loop.md)).
- **Learned weights**: one line in [Ideas](https://github.com/shrujancheruku/issho/issues/360).
- **Pairs.** About 10% of simulated groups are two people, "effectively 1:1 meetings, which the platonic rules try to avoid"; the note says to watch mixed pairs and tune (`docs/architecture.md`).
- **The simulator** is parked for the Pilot.
- **Identity verification** as "the real check" on age (Stripe Identity) was deferred to the public launch and conflicts with Vietnam's law ([privacy-and-legal.md](privacy-and-legal.md)).
- **One definition of who matching may place** (in the city, not deleted, not suspended) is an open refactor ([app.participants](https://github.com/shrujancheruku/issho/issues/401)).

## Where the code and the documents disagree

- The Glossary says **kept apart** and lists "blocked" under *Avoid*. Matching calls these pairs `Relations.Blocked` and its violation text says "blocked each other".
- Backend README says "A non-conversational activity gets a food follow-up"; the code pairs by score ([plans-and-catalogue.md](plans-and-catalogue.md)).
- The Pilot's scope lists "plans that combine activities" as out, while two-activity pairing is on by default and the Da Nang–Hoi An pack doesn't turn it off. ADR 0002 (verdicts) settled the reading: the existing pairing stays on.
- Backend README and code comments cite retired milestones (M4, M6, M7).
- The "met" rule: the Glossary and the `app.attendance` view say two people met when both attended the same group. The query behind feedback picks uses its own rule that ignores check-ins ([ListMetForPicks uses the shared attendance rule](https://github.com/shrujancheruku/issho/issues/321), open).
