# The app's shape

The app's overall structure: its tabs and screens, how it looks and moves, and where it runs. The name and visual identity stand untouched in v2 (the map's Notes) and are recorded here only as context. Sources are cited as in the [README](README.md). The code is `apps/mobile`; the design tokens are in [`docs/design-system.md`](../../design-system.md).

## What v1 decided

### Where it runs

| Decision | Source |
|---|---|
| **One codebase for Android, iOS and the web**: React Native with Expo. | Log, 2026-10-20 |
| **The web build is kept working** on every pull request, and for the Pilot is the **fallback** at `app.joinissho.com`. | Log, 2026-10-21; [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |
| All three platforms are **required** for the soft launch. | [Wayfinder: soft launch in Da Nang and Hoi An](https://github.com/shrujancheruku/issho/issues/334) |
| **No push on the web.** | Log, 2026-10-21 |
| **Maps are Google Maps everywhere**, because Google's policy requires Places results on a Google map. | Log, 2026-10-22 |
| A separate **admin web app** for the owner ([running-issho.md](running-issho.md)). | Log, 2026-10-23 |

### The structure

The app was redesigned on 2026-10-26 (owner's request). "The old feed and profile tabs mixed finding plans with following them. Grouping by what needs the person now, with one screen per outing whose content follows its stage, keeps the one thing to do on top." (Log, 2026-10-26.)

**Three tabs** (`apps/mobile/README.md`):

| Tab | What it holds |
|---|---|
| **Together** | What needs an answer ("Needs you"), your groups (outings and crews, each with its chat), and plans waiting for a date. |
| **Plans** | The feed as sections by level, with search, filters and a map of all nearby plans coloured by level. |
| **You** | Profile (each section editable), appearance, account (sign out, delete). |

**Screens beyond the tabs** (`apps/mobile/src/app/`):

| Screen | What it is |
|---|---|
| Sign-in | Email, then a 6-digit code. |
| Onboarding | One pass through seven steps ([people.md](people.md)). |
| Plan | One plan: its steps, map and route; "stage one of the outing once signed up". |
| Signup sheet | The times a person can do for that plan. |
| Outing | **One screen whose content follows the stage**, along a gold thread with five knots. |
| Feedback | "How was it?" |
| Report | Report someone in the group. |
| Crew | An offer to answer, or an active crew with its people, suggested plans and chat. |
| Leave a crew | The reason. |
| Chat | A room on its own. |
| Suspended | Only "Your account is suspended", with sign out and delete. |

### The thread

The **gold thread** is the app's central image: an outing is a thread with a knot at each stage (plan, date set, going, on the day, how was it), and a crew is the last knot. The site tells the same story the same way (Log, 2026-10-26). The name's meaning ("together") and the logo's thread tie into it.

### Look and motion

| Decision | Source |
|---|---|
| **Indigo ("Ai") ground and accent** on every surface, chosen after comparing five grounds; three alternatives' values are recorded so switching is a token swap. | Log, 2026-10-25 (owner) |
| **Gold leads the UI**; red is the logo's only (and error text); a vermilion marks "Love it". | Log, 2026-10-25 (owner) |
| **Dela Gothic One** for standout text, **Zen Kaku Gothic New** for body text. | Log, 2026-10-20, 2026-10-25 |
| **Light, Dark, or Match my phone**, stored on the device and applied before the first screen draws. | Log, 2026-10-26 |
| **Motion tokens**: press, change, move, draw (the thread growing), gather (knots tying, a crew starting), breathe ("the one live thing on a screen"), plus map pins dropping and a route drawing itself with a gold walker strolling it. Reduced motion is respected. | `docs/design-system.md` |
| **Text sizes and colours come only from the theme**, enforced by lint. | Log, 2026-11-04 |
| **Every screen shares one frame** and one loading ladder (loading, failed with retry, not found); a failed refresh keeps the data and says "Couldn't refresh". | Log, 2026-11-02, 2026-11-03, 2026-11-04 |

### How it behaves

| Decision | Source |
|---|---|
| **Two languages at once**: short text side by side, long text with a switch on every screen ([languages.md](languages.md)). | Log, 2026-10-20 |
| **The server decides, the app shows.** Rules about an outing, a crew or a plan live in one function each, and screens only render. | Log, 2026-11-02, 2026-11-04 |
| The app **refetches when it comes to the front and when a push arrives**; data is treated as fresh for 30 seconds. | Log, 2026-10-26, 2026-11-04 |
| Names of cities and activities are **kept on the device** and refreshed hourly, so the app opens with last time's names. | Log, 2026-10-26 |
| **Interest levels are colours** shared by the legend, the onboarding tiles, the feed's sections and the map's pins. | Log, 2026-10-21 |
| **Notification permission is asked after a first signup**, not at start. | Log, 2026-10-22 |
| **Server messages are never shown** to people: they're English and name fields. | Log, 2026-11-02 |
| "Designs that need data the API doesn't send yet **degrade** (no unread counts, no check-in list) rather than waiting for backend work." | Log, 2026-10-26 |

### Decided on tickets after the log froze

| Decision | Built? | Source |
|---|---|---|
| **A first-run tour**: a spotlight over the real app (the owner's choice over a card carousel), with example rows for what a new member can't see yet; shown once after onboarding; replayable from You. | No | Log, 2026-10-29; [a first-run spotlight tour](https://github.com/shrujancheruku/issho/issues/355) |
| **Screens for standing**: invite-only, consent needed, update required. | No | [people.md](people.md); [the API can require a minimum app version](https://github.com/shrujancheruku/issho/issues/480) |
| **An invite action on You** (up to 5 emails). | No | [only invited people can join the Pilot](https://github.com/shrujancheruku/issho/issues/477) |
| **The outing screen works offline on the day.** | No | [the outing screen works offline on the day](https://github.com/shrujancheruku/issho/issues/484) |
| **Block** buttons beside Report. | No | [safety.md](safety.md) |
| **The app gate as one pure decision**, and one device store for what the app keeps. | No | [the app gate is one pure decision](https://github.com/shrujancheruku/issho/issues/530), [one device store for what the app keeps](https://github.com/shrujancheruku/issho/issues/531) |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| The Expo template's tabs, then a feed tab and a Profile tab | Together / Plans / You | Finding plans was mixed with following them | Log, 2026-10-21, 2026-10-26 |
| Separate screens per stage of an outing | One outing screen along the thread | Keeps the one thing to do on top | Log, 2026-10-26 |
| Red and gold competing on every screen | Red in the logo only; gold leads | "Red meant both 'do this' and 'careful'" | Log, 2026-10-25 |
| A system body font | Zen Kaku Gothic New | Sets Japanese and Latin with the same rhythm | Log, 2026-10-25 |
| An animated example tile explaining interest levels | A colour legend | The animation was distracting | Log, 2026-10-21 |
| A card-carousel walkthrough | A spotlight over the real app | "It shows where things really are" | Log, 2026-10-29 |
| Each screen hand-writing its frame and loading states | One frame for every screen, tabs included | The copies had drifted: one sheet spun forever | Log, 2026-11-02, 2026-11-03 |
| Forty inline text sizes and six raw colours | Theme tokens only | They ignored dark mode | Log, 2026-11-04 |
| Any display language | English and Vietnamese (decided, unbuilt) | The Pilot's scope | [languages.md](languages.md) |

## Left open, parked, cut or only planned

- **The redesign has not been checked on phones** ([Check the redesign on phones](https://github.com/shrujancheruku/issho/issues/400), an owner task). It was built and checked in a browser.
- **Nobody outside the owner has used the app.** The only device test recorded led to cutting the activity list from 230 to 66.
- **Data the redesign was drawn around and the API doesn't send**: unread counts and last messages on Together; who has checked in; which crew members signed up; a crew's past outings ([chat.md](chat.md), [outing.md](outing.md), [crews.md](crews.md)).
- **The type scale** needs a revisit after sizes were snapped to tokens (Ideas).
- **Web session security** is undecided: the refresh token in `localStorage` with a strict content policy, or cookie sessions through an endpoint of our own. Also open on the web: Sentry, keyboard navigation, the map on desktop, check-in and location ([Web for launch](https://github.com/shrujancheruku/issho/issues/340)).
- **Vietnamese fonts** are unchecked on phones.
- **A crew offer's own walkthrough**: Ideas.
- **iOS has never been built for a device**: the dev Mac's macOS is too old for local builds, and there is no Apple account.
- **Notification settings, a help or support screen, an inbox**: none exist.
- **Almost no route screen has a test**: about 2,990 lines of routes, with tests only for the Plans tab and sign-in.

## Where the code and the documents disagree

- **The Glossary's stages are four** (invited, going, on the day, after) and the thread has **five knots** (it adds the plan before the date); the mobile README calls the plan screen "stage one of the outing once signed up".
- **"The one place the rules about an outing live"** (mobile README, of `outing.ts`) while the server holds the same rules and they differ ([outing.md](outing.md)).
- The mobile README says `src/app/` is "Routes only"; the crew route holds five components and their animation.
- The mobile README's structure table omits the report and leave-crew screens.
- `docs/design-system.md` says the marketing page is "dark only"; it has light and dark.
- The log's first-run tour rows (2026-10-29) read as built; the tour doesn't exist.
- The mobile README still says iOS builds are "deferred to M9".
