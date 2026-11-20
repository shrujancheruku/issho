# Prototype: the app's shape

Issue [#560](https://github.com/shrujancheruku/issho/issues/560), part of the v2 map [#532](https://github.com/shrujancheruku/issho/issues/532). Built and judged 2026-11-20. Throwaway: this branch is never merged.

The page the owner judged is a private artifact: https://claude.ai/artifact/988txzxq3ba2YYyyopgoP3. Its source is [the-apps-shape/index.html](the-apps-shape/index.html) (open it in a browser; `?variant=A|B|C|D`, `?theme=light|dark`). This file is the durable record. Plans, names and prices on the page are invented examples, not vouched.

## Question

How is the app laid out so the core loop is obvious: the top-level sections, what someone sees first, and where plans, outings, crews and chat live?

## Method

Four layouts that disagree about structure, each drawn as phone frames through the same five moments of one person's week (first open with no account, home with things in play, a plan, the outing on the day, a crew), then three browser views. One further row was the same in every variant: the signup sheet, the push ask, filters, settings, the making screen, and the one-line states.

| | Shape | Where a person's things live | Where it strained |
|---|---|---|---|
| A | Three tabs: Together, Plans, You (v1's, corrected to v2) | Together, grouped by kind | Tonight's outing can sit below a crew proposal due next week; a signed-up plan is drawn in two places |
| B | No tabs: the plans list is the app | A dock above the list; the outing is the plan page plus a private strip | Crews have no home except the dock's sheet; the dock hides all but the next thing |
| C | Week, Plans, Crews | A dated thread of the week; crews get a tab | Thin weeks look empty; Crews is an empty tab until a first crew |
| D | Plans, Chats, You (a messenger) | The room is the outing | No room exists before an outing is set, so invitations and waiting need a second home and the loop splits across two tabs |

## Verdict

The owner chose **A, with two parts taken from the others**, and settled the open points:

- **Tabs:** Together, Plans, You. The owner and insiders get a fourth, **Making**.
- **Together is ordered by time** (from C): a "Needs you" block on top, then everything else of the person's by date, not by kind.
- **The outing is the plan's page plus a private strip** (from B): the date, the group, check-in. The plan is never drawn on a second screen.
- **Chat is a pane within that screen**, and within the crew screen. There is no chats list.
- **Crews** are part of Together, with no tab of their own.
- **Opens on** Plans when nothing is in play, otherwise Together.
- **Kinds of activity:** Make, Move, Play, Walk, Learn, Watch.

The shared row was accepted as drawn. The chosen combination itself was not drawn; per-screen design is still in the map's fog.

Rejected: B as a whole (crews homeless), C's Crews tab (empty for most people for weeks), D (works against Chat's rule that a room opens only when an outing is set, and puts talking at the centre).
