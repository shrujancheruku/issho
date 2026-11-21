# The plan bar

From [What makes a plan good](https://github.com/shrujancheruku/issho/issues/540), amended by [A prototype of good plans in Da Nang–Hoi An](https://github.com/shrujancheruku/issho/issues/542), [Building blocks](https://github.com/shrujancheruku/issho/issues/544), [What varies by city](https://github.com/shrujancheruku/issho/issues/545), [The outing](https://github.com/shrujancheruku/issho/issues/550) and [Running Issho](https://github.com/shrujancheruku/issho/issues/557).

## The bar, in one sentence

**Someone who knows the city would send a friend on it as written, and everything it says is true on the day.**

One bar for every plan, whoever made it. Insiders skip no checks.

## Accurate

A plan is accurate when all of these hold:

1. Every place **exists, matches its name and is where the plan says**. A doubtful match counts as not found.
2. Every place is **open for the whole time the plan uses it**, on the outing's date and in that season.
3. It **suits a group of 3 to 6**: room, table, court or booking for that many; no single-person activity.
4. Every leg is **within the walking and ride limits** ([Routes](routes.md)), and the meeting point is a **signed, one-of-a-kind place**. An alley mouth or a stretch of beach is not one, even when it is unambiguous on a map.
5. **Cost and what to bring** are stated and right within a range.

The five checks run in code **when the plan opens** and **again at the 24-hour mark**. A piece that is doubted or past its re-check date fails them ([Pieces](pieces.md)).

## Compelling

1. **Worth the trip**: 3 to 4 hours around at least one thing the group does together that isn't eating or looking. Food supports it.
2. **The activity suits people who live here**, not only visitors.
3. **The activity comes before the meal**, as a rule of thumb: it breaks the ice and gives the meal a topic.
4. **Local knowledge**: not something a newcomer would find or assemble alone; no chain branch as a stop unless it is the point; no tourist trap.
5. **Fits together**: order, time of day and places make sense as one outing.
6. **Things of interest along a walk are marked as options.** Not a hard rule.
7. **A paid, bookable activity is allowed** when its cost is stated up front.
8. **The stranger's test**, the sentence above. The items before it are how a reviewer explains a "no".

## Who judges

### Before a plan opens

- The owner approves every plan. A **model judge** scores it first against this bar and gives reasons. Code alone checks accuracy.
- The judge's verdicts are logged against the owner's. Once they agree on 27 of the last 30, review narrows to the plans the judge flags. That agreement rate is the automatic evaluation of plan drafting.
- Trust in the model judge is earned separately in each city: a new city starts with the owner approving every plan.
- The owner can switch full review back on for a city, with a reason.

### After it runs

Feedback asks each attendee two plan questions, separate from the people questions ([Outings](outings.md)):

- **Was it as described?** Yes, or no plus which stop was off.
- **Would you send a friend on this plan?** 1 to 5.

A plan is **rated poorly** when one accuracy report is confirmed by the owner, or the group's median on "send a friend" is 3 or lower with at least 2 answers. Feedback has no group rating.

## Below the bar

- **Before it opens:** a plan that fails a check or review is never opened; it is fixed or dropped.
- **After people are going** (it fails the check at the 24-hour mark, or something closes): a like-for-like swap, or a change of time or day for a cause outside Issho's control, with the group told on every channel ([Outings](outings.md#changing-a-set-outing)). If neither is possible, the outing is cancelled and its people get the same priority as people left waiting.
- **Rated poorly:** the plan is closed until changed. Rated poorly again after a change, it is retired.

## How the bar is measured

- At least **80% of outings** have a group median of 4 or more on "send a friend".
- Under **5% of outings** have a confirmed accuracy problem.
- The pass rate of plan drafting is an engineering number, not the bar. Targets are tuned on Pilot data.

## What the prototype showed

Twelve real Da Nang–Hoi An plans were judged by the owner ([record](https://github.com/shrujancheruku/issho/blob/prototype/good-plans-da-nang-hoi-an/docs/prototypes/good-plans-da-nang-hoi-an.md) on the unmerged `prototype/good-plans-da-nang-hoi-an` branch).

- Local sources beat Google alone: a mean of 4.3 on "send a friend" against 2.3, with unvouched pieces at 2.7. Google's failures were tourist traps, which popularity cannot detect.
- Good pieces don't make a plan by themselves. With an activity at the centre, plans reached 5.
- Activities are the hard part, and need a person's visit more than the food and walks around them.
- Run by hand, the cost check failed on almost every plan, group fit was rarely confirmable from any source, and Google's hours were wrong for places that sell until they run out. These are the facts a vouch records ([Plan knowledge](plan-knowledge.md)).
- One local person's recommendation was not tested.
