# The stages of launch

How Issho was meant to reach real people: the Pilot, the soft launch and the public launch. Sources are cited as in the [README](README.md). Most of this was decided on two maps, [Wayfinder: soft launch in Da Nang and Hoi An](https://github.com/shrujancheruku/issho/issues/334) and [Wayfinder: keep, renovate or rebuild before the Pilot](https://github.com/shrujancheruku/issho/issues/411), not in the decision log.

## What v1 decided

### Three stages

| Stage | What it is | Source |
|---|---|---|
| **Pilot** | "An invite-only trial in one city pack before the soft launch, with people the owner invites and the people they invite in turn. It runs until the owner ends it, and its participants stay on when the soft launch opens." | Glossary |
| **Soft launch** | "The first opening of Issho beyond the Pilot's invitees: one city pack, with people admitted through the waitlist." | Glossary |
| **Public launch** | Named, never defined. It is when the LLC takes over, a lawyer reviews everything, and identity verification was once planned. | ADR 0004; [Legal operator and Vietnam filings](https://github.com/shrujancheruku/issho/issues/362) |

### The Pilot

From [What the Pilot is](https://github.com/shrujancheruku/issho/issues/412), amended by [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417):

| Decision | Detail |
|---|---|
| **Purpose** | Prove one full turn of the loop with real people, and that people come back for a second outing. |
| **Who** | About **10 seeds** the owner invites, then the people each seed invites (one or two degrees out), "so groups include some strangers". Capped at about **40 active people**, grown in waves. |
| **Where** | Da Nang–Hoi An. The owner chooses which areas open. |
| **Plans** | Activity plans and walks, but walks only in opened areas that pass the 70% eval. |
| **Platforms** | All three open on the first invitee's day: Android through Play's closed testing, iOS through TestFlight's public link, the web as the fallback. |
| **Duration** | Until the owner ends it. |
| **Success bar** | At least 60% of matched groups go ahead; at least 40% of people who attended attend again; no outing fails because of a bug; no safety incident goes unhandled. Checked after two turns, then every two. Passing "makes the soft launch possible but doesn't start it; the owner decides". |
| **By hand** | The owner seeds and approves plans, proposes crews, moderates through the admin, and gives support through a chat group outside the app. |
| **Legal floor** | "The full floor from day one" ([privacy-and-legal.md](privacy-and-legal.md)). |
| **Environment** | The production stack. "Nothing is wiped at the soft launch": participants, attendance, met, kept-apart pairs and suspensions carry over. |
| **A hard gate** | Soft-launch work stays frozen until the Pilot passes. |
| **Sign-in** | Email code, unchanged. Invite-only is enforced by an allowlist; each member may invite 5. |

### The soft launch

| Decision | Source |
|---|---|
| **The city**: Da Nang–Hoi An, not Tokyo: the owner will be based there. Android, iOS and the web are all required. The owner runs it alone. | [Wayfinder: soft launch in Da Nang and Hoi An](https://github.com/shrujancheruku/issho/issues/334) |
| **Admission is through a waitlist**, and areas **unlock**: "an area opening to the people waiting in it, once enough of them have joined." | Glossary |
| **Stores**: Play production and an App Store release, with admission decided by the waitlist in the app. New gates: Play's production-access application (about 7 days, after 12 testers opted in for 14 days) and a full App Store review. | [Distribution for each platform](https://github.com/shrujancheruku/issho/issues/343) |
| **Verification by Vietnamese phone number** is a soft-launch gate. | [Legal operator and Vietnam filings](https://github.com/shrujancheruku/issho/issues/362) |
| A **person checks the legal texts' translation** before the soft launch. | Same |
| **Supabase Pro** before the soft launch opens beyond the Pilot's invitees. | [Production stack](https://github.com/shrujancheruku/issho/issues/341) |
| **Trails ready in Da Nang–Hoi An** was named as "one gate": rides between stops, matching Vietnamese place names, and meeting places that aren't chain branches, checked by the walk eval at 70%. | [Rides between stops in Vietnam](https://github.com/shrujancheruku/issho/issues/393) |

### Where the first people would come from

Research only, with nothing decided ([Communities to recruit from in Da Nang and Hoi An](https://github.com/shrujancheruku/issho/issues/337)): the largest pool is university students in Da Nang (80,000 or more across two universities); the fastest is English-language activity groups that already meet strangers in small groups (a Meetup group of about 1,500, pickleball on Reclub at about 2,000, Toastmasters); bridges between Vietnamese and English speakers are language exchanges and student tour clubs; coworking spaces reach ages 25–40 and Hoi An, "but short stays work against the return-attendance metric and crews".

### The store accounts

| Decision | Source |
|---|---|
| Both accounts are **personal**: the owner's existing Play account (so the 12-testers-for-14-days rule applies, and the Pilot meets it) and an individual Apple Developer account. They move to the LLC before the public launch. | [Distribution for each platform](https://github.com/shrujancheruku/issho/issues/343); ADR 0004 |
| **Age rating**: each store's highest (Apple 18+), matching the app's own gate. | Same |
| **Moderating after posting** is argued as enough for Apple. | Same |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| Milestones M1–M11 in a ROADMAP file, ending in a "Launch" stage | Wayfinder maps and issues | The ROADMAP became a second tracker | ADR 0001 |
| Tokyo as the launch city | Da Nang–Hoi An | The owner's location | 2026-11-04 |
| Straight to a waitlist soft launch | **A Pilot first**, as a hard gate | Decided on 2026-11-05 while judging whether to rebuild | [What the Pilot is](https://github.com/shrujancheruku/issho/issues/412) |
| The Pilot without crews or a waitlist | Crews in (by hand); the site's waitlist form goes live to collect first entries | The logic was built | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |
| Apple, iOS push and Supabase Pro in the first release | Moved to M10, then to "Launch", then: Apple now, Pro at the soft launch | Paid accounts only once real users are close | Log, 2026-10-26, 2026-10-29; [Distribution for each platform](https://github.com/shrujancheruku/issho/issues/343), [Production stack](https://github.com/shrujancheruku/issho/issues/341) |
| Stripe Identity for the public launch | Out of the soft launch; in doubt for Vietnam | The law bars requiring ID photos | [Vietnam's rules for a social app](https://github.com/shrujancheruku/issho/issues/335) |
| "Nothing is rebuilt" before the Pilot | v2 is written from scratch | The owner's decision of 2026-11-12 | ADR 0002 (verdicts); [Wayfinder: Issho v2, from scratch](https://github.com/shrujancheruku/issho/issues/532) |

## Left open, parked, cut or only planned

Every soft-launch question was closed unanswered on 2026-11-06, "to be re-asked after the Pilot":

- **Waitlist and unlock**: where people join, what someone waiting sees and can do, the threshold that unlocks an area and by what measure, who triggers it, what happens to people outside the pack's areas, what an unlocked area's first week looks like ([Waitlist and unlock](https://github.com/shrujancheruku/issho/issues/339)).
- **Product gates**: which unfinished product work must be done first ([Product gates for the soft launch](https://github.com/shrujancheruku/issho/issues/344)).
- **Seeding**: which communities in what order, "how many hosts and partner venues", whether the first plans are hand-curated ([Seeding plan for Da Nang–Hoi An](https://github.com/shrujancheruku/issho/issues/346)).
- **Running it alone** ([Running the soft launch alone](https://github.com/shrujancheruku/issho/issues/345)).
- **Analytics and consent** ([Analytics and consent](https://github.com/shrujancheruku/issho/issues/342)).
- **The web for launch**: session security, check-in and location on the web ([Web for launch](https://github.com/shrujancheruku/issho/issues/340)).
- **Go/no-go criteria** for the soft launch: the map's destination, never written.

Also open:

- **The Pilot never started.** No invitee, no store listing, no Apple account, no production stack.
- **What the public launch is**: no definition, date or criteria.
- **A second city**: nothing recorded.
- **Whether the first cohort is Vietnamese-led or English-led**, and whether to accept short-stay visitors: open questions from the recruiting research.
- **The owner's remaining Pilot steps**: the test tracks ([Pilot test tracks on Android and iOS](https://github.com/shrujancheruku/issho/issues/467)), the filings ([File the PDPL impact assessments within 60 days of the first invitee](https://github.com/shrujancheruku/issho/issues/503)), a Vietnamese reader's review ([A Vietnamese reader reviews the core-loop strings](https://github.com/shrujancheruku/issho/issues/468)).

## Where the code and the documents disagree

- The Glossary defines **Unlock**, **Soft launch** and a waitlist; no code has a waitlist, an unlock or an invitation.
- The Glossary's **Pilot** says "the people they invite in turn"; sign-up is open to anyone.
- The site says "Tokyo first, then your city. We're opening one city at a time."
- The mobile README still describes iOS and EAS builds as "deferred to M9".
- [Road to the Pilot](https://github.com/shrujancheruku/issho/issues/433) is open with its tickets, while the v2 map says it is paused.
