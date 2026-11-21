# What varies by city

From [What varies by city](https://github.com/shrujancheruku/issho/issues/545), amended by [Routes worth walking](https://github.com/shrujancheruku/issho/issues/546), [Safety](https://github.com/shrujancheruku/issho/issues/553) and [Languages](https://github.com/shrujancheruku/issho/issues/556). The rules for changing a set outing, decided in the same ticket, are in [Outings](outings.md#changing-a-set-outing).

## The rule

Every local difference sits in one of three layers:

- **Same everywhere:** the loop, groups of 3 to 6, the bar, vouching and its clocks, conduct and reliability rules, the piece types, matching.
- **City settings:** a short list of facts about the city, decided when it opens.
- **Carried by pieces and plans:** anything a person has to have seen to know.

The test for a new difference: can someone know this without visiting? If no, it goes on a piece. If yes, it is a city setting only when a named city needs a different value; otherwise it is the same everywhere. Matching and the rules are never tuned per city.

**The city is the only unit that holds settings.** A city names its country, and currency, public holidays and legal obligations follow from it. There is no country-level layer until two cities in one country would repeat each other. Countries where Google's map is weak or absent (South Korea, China) are not designed for now.

## A city's settings

The whole list:

1. Name, and the country it is in
2. Time zone and currency
3. Its languages: the language most of its residents read, plus English always ([Languages](languages.md))
4. Its areas ([Cities](cities.md))
5. How people get around: own transport or public transport ([Routes](routes.md#how-people-get-around))
6. The clock hours of morning, afternoon and evening, and the weekend days
7. Its calendar
8. Its lead sources
9. Any override of the worldwide walk and ride limits, with its reason (heat, no pavements)
10. Its emergency numbers ([Safety](safety.md#emergencies))

The lead search's cadence is not a city setting ([Leads](leads.md)).

## The seams

| Seam | Answer |
| --- | --- |
| Walking against riding | How people get around is a city setting. The legs between stops are written on the plan, leg by leg, by its maker. The limits the accuracy check uses are one worldwide default a city may override with a reason. |
| Meeting points | No city setting. One rule everywhere (signed, one of a kind); the person who stood there supplies the local answer. |
| Sources of plan knowledge | Each city keeps a plain list of lead sources (name, link, what it is good for) that the lead search uses. No per-source code. Google is the identity layer in every city. |
| Activity types | One worldwide list ([The app's shape](app-shape.md)), with no per-city overrides. A city's activities are whatever has been vouched there. |
| Season and weather | The months a piece runs, and whether it needs dry weather or daylight, are recorded on the piece by whoever vouches; the plan inherits them as it does hours. |
| Times of day | The names are worldwide; the clock hours and the weekend days are the city's. |
| Units and money | Prices are always in the city's currency, never converted. Metric or imperial and the 12- or 24-hour clock follow the person's device. |

## The calendar

- An entry has dates, optionally the areas it covers, and one of two effects. **Closed:** no outing is scheduled on those dates (the days of Tết). **Caution:** an outing scheduled inside it needs a person's okay for that date (flood season, a festival that crowds the Old Town).
- Public holidays come from the country and are a caution by default. Something that affects one piece stays on the piece.
- The owner or an insider re-checks the calendar every 6 months. An attendee's "not as described" with a date-wide cause sends the calendar for a check as well as the piece. An overdue calendar is a warning to the owner, not a stop.

## The person against the city

- **Times:** the city's clock; when the person's device is in another time zone, their own time is shown beside it.
- **Money:** the city's currency.
- **Language:** the person's display language.
- **Distances and clock format:** the person's device.

## What a city is, and who changes it

- A **city** is its settings, areas, calendar, lead sources, leads list, pieces and plans.
- Settings and areas: the owner only, each change recorded with who and why. Calendar and lead sources: the owner and the city's insiders. Pieces and plans: as decided in [Pieces](pieces.md) and [Plans](plans.md).
- Settings apply to the whole city; only calendar entries and pieces can speak for part of it. A plan and a piece belong to exactly one city. Two places needing different settings is the sign they are two cities.
- A changed setting applies from each plan's next round; signups already made keep the meaning they had. A new **closed** date applies at once: an outing set on it is cancelled, and its people get the priority of people left waiting.

What a city supplies before it opens is in [Cities](cities.md#what-a-new-city-costs).
