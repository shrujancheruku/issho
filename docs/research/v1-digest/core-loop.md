# The core loop

How a person goes from seeing something they could do to doing it again. The detail of each step is in the area documents; this one records the loop's shape and the decisions that shaped it. Sources are cited as in the [README](README.md).

## What v1 decided

### The loop

1. **Sign in and onboard.** An email code, then one onboarding pass: languages, about you, home area, availability, activities, kind of outing, the code of conduct ([people.md](people.md)).
2. **See plans.** The Plans tab lists every open plan in the person's city, best for them first. A **plan** is "an outing people can sign up for, before it has a date or a group" (Glossary).
3. **Sign up, with times.** One tap uses the profile's usual availability; a person can instead give weekly windows and blocked-out dates for that plan (Log, 2026-10-21). They can sign up for several plans. Signing up commits them to nothing.
4. **Wait.** The plan shows how many are interested, never who (Log, 2026-10-21). A signup means "waiting" and nothing else (Log, 2026-10-22).
5. **Matching sets a date and a group.** Hourly, the worker schedules a plan once its signups can form a good group, and at the signup deadline goes ahead with whatever groups can form, or closes the plan ([matching.md](matching.md)).
6. **Invited: can you go?** The person gets a push and answers by the start of the day before. The plan is now an **outing** for them (Glossary).
7. **Going.** A group is confirmed as soon as enough people say yes. Confirmed people see each other's first names, get the group chat from 48 hours before, and the meeting point from 24 hours before ([outing.md](outing.md)).
8. **On the day.** They check in ("I'm here") from 30 minutes before the start.
9. **After.** From 3 hours after the start, "How was it?": whether they went, ratings, who they'd meet again and who they'd rather not ([outing.md](outing.md)).
10. **Again.** The feed lifts plans that people they picked (and who picked them) signed up for. People who got on across outings are offered a **crew**, a standing chat that is suggested a plan each week ([crews.md](crews.md)).

### The decisions behind the shape

| Decision | Source |
|---|---|
| **Plan first, time second.** Plans are suggested without a date, signups are collected with availability, and a date is set once the signups can form a good group. The reason given: it pools interest over time instead of needing one week's slot to line up, which was seen as the main density risk. | Log, 2026-10-21 |
| **Issho makes the plans; nobody hosts.** The matcher suggests every plan from the city's catalogue, venues and people's stated interests. Admins can also create one by hand. | Log, 2026-10-21, 2026-10-24 |
| **No browsing of people.** Before confirming, a person sees counts only. After confirming, first names of their own group. | Log, 2026-10-21; Backend README, `GetEvent` |
| **A smaller outing beats no outing.** Two people is the hard minimum; three or more is a preference. A decline in a group of three doesn't cancel it. | Log, 2026-10-21 |
| **People who are left waiting are told, not moved.** When a plan closes without a group, the people waiting get a push that opens the feed; they are not signed up to a similar plan. | Log, 2026-10-22 |
| **Quality over count.** "A weak plan costs more trust than a missing one." A walk that fails its checks isn't opened. | Log, 2026-10-22 |
| **The north-star number is people coming back**: of the people who attended in a week, those with two or more outings so far. | Log, 2026-10-23; Backend README, `MetricsService` |
| **Good groups of regulars are the goal, not a problem.** No cap on friends in a group. | Log, 2026-10-23 |
| **The Pilot's version of the loop:** sign up for a plan → matched into a group → outing dated → attend → check in → feedback, and people coming back for a second outing. | [What the Pilot is](https://github.com/shrujancheruku/issho/issues/412) |

### What Issho does between the steps

One hourly pass per city, in a fixed order: settle groups whose answer deadline passed, schedule open plans, suggest new plans, then meeting places and routes, then the crews pass (Log, 2026-10-29; Backend README, "Plans, signups and events"). Pushes go out from an outbox every minute ([notifications.md](notifications.md)).

### Where v1 expected the loop to break

Recorded as standing risks in [`docs/architecture.md`](../../architecture.md), "Known risks":

- **Not enough nearby people**: "the main product risk". The answers were plan-first scheduling and hand-curating the first city.
- **Drop-off between signup and confirming**: signing up is cheap. The answers were per-plan availability and inviting two extra people.
- **Feedback depends on honesty**: "I went" counts as attending without proof.
- **Matching is untested on real behaviour**: the simulator's people are guesses. About 10% of simulated groups are pairs.

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| Weekly proposals: matching proposes an outing with a date, people accept or decline | Plan first, time second | Density: a week's slot rarely lined up | Log, 2026-10-21 |
| One proposal per person per matching run | Sign up for several plans; an outing they're going to blocks its occasion in their other signups | Superseded the same day by signups | Log, 2026-10-21 |
| A hard minimum of 3 | A hard minimum of 2; 3 is a preference | Owner: a smaller outing beats none | Log, 2026-10-21 |
| A group's venue chosen per group at scheduling | Each plan names its venues when it opens | Owner: people sign up for a concrete outing ("dinner at X"), not a kind of place | Log, 2026-10-22 |
| At most 2 people who know each other per group | No cap on friends | People coming back is the north star | Log, 2026-10-23 |
| Crews offered by the server | For the Pilot, the owner proposes crews by hand from a list of candidates | Decided, not built | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |
| "The Pilot proves the loop without crews" | Crews are in the Pilot | The logic was built | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |

## Left open, parked, cut or only planned

- **The loop has never run with real people.** No outing has happened through Issho.
- **An end-to-end test of the loop** was a Pilot gate and is unbuilt ([An end-to-end test of the core loop](https://github.com/shrujancheruku/issho/issues/463), [an API-level test of the core loop](https://github.com/shrujancheruku/issho/issues/490)).
- **How many plans a city gets** was left as the owner's call and never made. Simulated on 2026-10-26: one plan per 8 / 10 / 15 / 20 / 30 people gives 58 / 69 / 86 / 93 / 95% of plans filled before the deadline, and 55 / 52 / 46 / 42 / 34% of people with an outing. The default stayed at 10 ([Decide how many plans a city gets](https://github.com/shrujancheruku/issho/issues/349)).
- **A first-run tour** (a spotlight over the real app, shown once after onboarding) was decided on 2026-10-29 and is unbuilt ([a first-run spotlight tour](https://github.com/shrujancheruku/issho/issues/355)).
- **Reminders on the day** were decided on a ticket and are unbuilt ([notifications.md](notifications.md)).
- **Money** is not in the loop. A paid tier ("per-event fee or membership") is one line in [Ideas](https://github.com/shrujancheruku/issho/issues/360).
- **Who may make plans** beyond the matcher and admins was never asked in v1.

## Where the code and the documents disagree

- The Glossary calls step 6 onward an **outing**; the schema, the API and most code call it an **event**. The Glossary allows this ("The code and the API call it an event"). The rename is an open ticket ([an outing is called an outing in the schema, the API and the apps](https://github.com/shrujancheruku/issho/issues/496)).
- `backend/README.md:278` says "Not done yet: chat, push notifications, the travel-matrix refresh and AI-written plan text." Chat, push and the refresh exist.
- The marketing site tells the loop as six stages (plan, date set, going, on the day, how was it, crew), which matches the app. Its closing line is "Tokyo first, then your city." ([site.md](site.md)).
