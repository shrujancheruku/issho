# The stages of launch

From [The stages of launch](https://github.com/shrujancheruku/issho/issues/548), amended by [The v2 glossary and domain model](https://github.com/shrujancheruku/issho/issues/569), with [Cities](https://github.com/shrujancheruku/issho/issues/555), [People](https://github.com/shrujancheruku/issho/issues/552) and [Running Issho](https://github.com/shrujancheruku/issho/issues/557). Research behind it: [Comparable products](https://github.com/shrujancheruku/issho/issues/535).

## The stages

Three stages, all in Da Nang–Hoi An, each defined by who can get in. Stages never reverse.

1. **Pilot**: by join link only.
2. **Soft launch**: anyone can ask to join; people are admitted from a waitlist as fast as plans and groups can absorb them.
3. **Public launch**: no waitlist. The LLC takes over from the owner ([ADR 0004](../adr/0004-the-owner-operates-issho-until-the-public-launch.md)).

A second city is not a stage; it has its own gate, below.

## The Pilot

**What it proves**, in this order, and nothing else:

1. The loop turns: people who sign up get an outing within the 7-day wait, and set outings go ahead.
2. Plans meet [the bar](plan-bar.md).
3. The promise holds: people's second outing includes someone from an earlier one.

It does not try to prove that people will pay, that strangers can be recruited at scale, that insiders work, or that a city can open without the owner.

**Who.** About 10 seeds the owner admits, then the people they bring in, grown in waves.

- The cap is about **60 active people**.
- The seeds live in Da Nang–Hoi An (staying three months or more). The people they bring in are welcome wherever they live.
- Seeds are chosen so the Pilot has both Vietnamese and foreign residents, with no fixed ratio.
- Each member gets **3 join links** ([People](people.md#join-links)).

**Seeing plans.** Plans are visible to anyone. A signup needs admission. A person without a join link is offered the waitlist, so the soft launch's waitlist starts filling during the Pilot.

**Insiders.** None. Every Pilot plan is Issho's, made by the owner. Suggestions are open, so the candidate pool builds from the first day.

**Conditions.** It runs on the production system and nothing is wiped; members stay on into the soft launch with their history. Android, iOS and the web are open from the first member's day. The owner makes and approves every plan, approves groups and crew offers, moderates, and gives support by email only ([Running Issho](running-issho.md#support)).

**The success bar**, read once the Pilot has run at least 20 outings over at least 6 weeks.

| | Bar |
|---|---|
| The loop | At least 60% of people who sign up in a week get an outing within the 7-day wait; at least 80% of set outings go ahead with 3 or more |
| The plan bar | 80% of outings at 4 or more on "send a friend"; under 5% with an accuracy problem |
| The promise | At least 30% of people who attended have had a second outing with someone from an earlier one |
| The floor | No outing fails because of a bug; no safety incident goes unhandled |

Passing makes the soft launch possible; the owner decides whether to start it. The Pilot has no end date. A miss means changing the product and reading the bar again. The gates [Running Issho](running-issho.md#gates) adds before the Pilot and the soft launch also apply.

## The soft launch

**The waitlist.** People are admitted city-wide in weekly **waves**, oldest first. Each Monday the admin proposes a wave size and the owner confirms, changes or skips it ([Running Issho](running-issho.md#the-numbers)). A wave is only as large as keeps the got-an-outing share at 60% or more. Members keep their 3 join links, and a person who arrives by one skips the queue. A waitlisted person can see plans and nothing else. There are no areas "unlocking": the city is the only unit, and matching draws on all of it.

**When the numbers slip.** Waves stop while the got-an-outing share is under 60% or the plan bar is missed.

**Filling the waitlist.** By members sharing plans, and by the owner going in person to groups that already meet in small numbers. No paid ads before the public launch.

**Insiders.** The first are brought on during the soft launch, from people whose suggestions keep being used.

**Sign-in and verification.** Apple and Google sign-in are added ([People](people.md)). No phone number is asked for ([Privacy](privacy.md#no-phone-number-and-the-sign-in-log)).

**What it proves.** What the Pilot didn't: strangers from a waitlist, payment, and the owner running it alone.

## Money

- **The model.** People pay Issho a **fee** per outing, charged when they are going, and refunded if they cancel more than 24 hours before the start or the outing doesn't happen. Their own food and activity costs are paid on the spot. Not a membership: in a thin city a membership bills people for weeks with no outing.
- **Nothing is charged in the Pilot.**
- **The trigger**, inside the soft launch: the fee turns on once the city has had 50 or more people going on outings each week for four weeks running, and has 30 plans. Turning it on is a notice change that needs a new consent tick ([Privacy](privacy.md)).
- **A person's first outing is always free.**
- **Insiders** earn a share of the fees from each outing of their plan that ran, once the fee is on. An insider's plan never costs more than Issho's own, and insiders never pay Issho. Insiders are a cost that buys plan quality, not a revenue line.
- Venues never pay.

## The public launch

It needs all of:

- the Pilot's bar still held at soft-launch size, with people from the waitlist;
- the fee charged for at least 8 weeks with the bar still holding;
- a week's operations fitting in the time the owner can give it alone;
- the LLC, the lawyer's review and the store accounts moved, as ADR 0004 requires.

After it, a city's waitlist can come back if the got-an-outing share falls under 60% or the plan bar is missed. The waitlist is a standing control on a city's density, not only a stage.

## A second city

It waits until the first city has reached its public launch, and is chosen then ([Cities](cities.md#choosing-the-next-city)). It opens at the waitlist stage, with no invite-only Pilot of its own, once it has its 12 plans and **150 people on its waitlist who live there**. The 150 assumes a third of those admitted go out in a given week; it is revised with the first city's real ratio.
