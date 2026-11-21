# Routes

From [Routes worth walking](https://github.com/shrujancheruku/issho/issues/546), with [Cities](https://github.com/shrujancheruku/issho/issues/555). Research behind it: [Route quality sources](https://github.com/shrujancheruku/issho/issues/538), whose recommended per-street score this overturns.

## The pleasant way

- **Issho computes no pleasant route and hosts no router.** The pleasant way exists only where a person walked it: a vouched **stretch**. A score can't see parked motorbikes on a pavement, and a model vouches for nothing.
- There is no formula for how much the pleasant way wins over the short way. The maker chose the stretch by walking it; the only cap is the limits below.

## Legs

- A **leg** joins one stop to the next. The maker states its mode (walk, ride or transit) and its minutes, having done it. That is Issho's own fact and is stored.
- The accuracy check compares the stated minutes with Google's live time when the plan opens and at the 24-hour mark, and flags a leg where Google's time is over 1.5 times the stated one. Google's figure is never stored.

## A walk

A **walk** is an activity, not a type of plan. There is one type of plan, with stops and legs.

- It is one or more stretches walked continuously, 45 to 120 minutes on foot, with optional stops marked along it.
- It may be longer than the leg limits.
- It runs only within its stretches' good months and hours.
- In an own-transport city it ends where it begins. In a public-transport city it starts and ends near a station or stop, which may differ.
- It can be a plan's only activity with no extra rule: it is vouched in person against the bar, and review decides. A stroll that doesn't meet the bar stays a stretch used as a leg.

## How people get around

A city setting with one value per city: **own transport** or **public transport**. The list of vehicles sits underneath it. There is no per-area value; a mixed city picks the one most people use to reach a plan. Da Nang–Hoi An is own transport.

| | Own transport | Public transport |
|---|---|---|
| Legs allowed | Walk, ride | Walk, transit (the maker states the line and minutes) |
| Where a plan ends | Within 10 minutes' walk of the meeting point, unless it has ride legs | Within 10 minutes' walk of a station or stop |
| A meeting-point vouch adds | Parking | The nearest station or stop |
| A walk | Ends where it begins | Starts and ends near a station or stop |

The setting switches nothing else for now. A rule that a plan ends before the last train is an opening task for the first public-transport city.

## Rides

- Everyone makes their own way: their own motorbike or car, or a hired ride. Issho books nothing, pays nothing and never suggests sharing a bike.
- A plan shows before signup that it has rides, and its total time getting around.
- The stop after a ride is vouched for parking and for being findable on arrival, because the group re-forms there.
- No ride leg follows a stop where drinking is the point.

## Limits

Worldwide defaults. A city overrides one with a reason; Da Nang–Hoi An starts with none.

| Limit | Value |
|---|---|
| Plain walking leg | 10 minutes at most |
| Walking leg along a vouched stretch | 20 minutes at most |
| Ride or transit leg | 15 minutes at most |
| Ride or transit legs in a plan | 2 at most |
| All legs together, not counting a walk that is the activity | 45 minutes at most |

## Vouching a stretch

An insider or the owner walks it end to end, in person, at a time it is meant to be used, and records:

- the line they walked, drawn by them (Issho's own data)
- its two ends and the minutes it took
- the months and hours it is good for
- timed rules, such as Hoi An Old Town's motorbike ban
- whether it is lit after dark
- steps or rough ground
- notes on things of interest along it

The accuracy check refuses to open a plan whose stretch or walk falls outside its good months and hours. The city's caution dates cover seasons such as the rains. A stretch has no Google place ID, so nothing flags it closed automatically; corrections and renewal keep it honest ([Plan knowledge](plan-knowledge.md#people-who-went)).

## On the day

- No turn-by-turn directions in the app.
- The plan lists its stops in order with the legs between them ("8 min walk", "12 min ride").
- Each stop has an "open in Maps" link by place ID.
- A stretch or walk shows Issho's own line on the map, with its notes and two ends.
- After a ride, the next stop shows its parking note.
- A plan gets one multi-stop Google Maps link only when every leg is a plain walk. Otherwise links are per leg, in that leg's mode.
