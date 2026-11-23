# Walk: the core loop, from a join link to feedback

The owner's script for journey 1 ([The quality bar and test strategy](../spec/quality.md#end-to-end), [#622](https://github.com/shrujancheruku/issho/issues/622)): sign in as, do, expect. It is the same journey `make e2e` drives in a browser (`e2e/tests/core-loop.spec.ts`), written for a person to walk on the **web**, the **iOS simulator** and the **Android emulator**, one platform per pass ([Native along the way](../spec/quality.md#native-along-the-way)). It was written, not run, on the simulator and the emulator.

## Before you start

- **Three people.** Use three email addresses you can read (call them **A**, **B** and **C**; plus-addresses of one mailbox do). Each is a different person with a different first name, so you can see who appears where: say **Zephyr**, **Marigold** and **Quillon**.
- **An admitted owner with 3 join links.** The owner is the account `make seed-dev ADMIT=<email>` admitted; sign in as the owner on the web, open the **You** tab and make three links (**Make a join link**), and copy each. A link is good for 14 days and one use.
- **Which build.** The web: the dev address in a browser at a phone width (about 390 wide). The simulator and the emulator: a local development build pointed at the same API (`apps/app/README.md`, "Native builds"), with its Metro up (`make native-metro`); stop it afterwards (`make native-stop`). Only one native worker at a time uses the simulator, the emulator and Metro.
- **Opening a join link.** Web: paste `https://<the dev address>/join/<code>` in the address bar. iOS simulator: `xcrun simctl openurl booted "issho://join/<code>"`. Android emulator: `adb shell am start -a android.intent.action.VIEW -d "issho://join/<code>"`. (The `https` link on a phone is the release milestone's.)
- **Where the code is.** Sign-in is a 6-digit code emailed to the address; it works once and lasts 10 minutes.

## Part 1: arriving, profile and signing up (walkable on dev today)

Do this for A, then B, then C, each in a fresh session (a private window, or an app with its data cleared / a reset simulator). Use the next join link each time.

| # | Sign in as | Do | Expect |
|---|---|---|---|
| 1 | A (signed out) | Open A's join link. | The **Sign in** screen with an **Email** field. The link's code is not in the address bar. |
| 2 | A | Type A's email, press **Send me a code**. | "We sent a 6-digit code to …", a **6-digit code** field, **Send a new code** (disabled for a minute). |
| 3 | A | Type the code from the email, press **Sign in**. | **You're in**, "Welcome to Issho. You can now sign up to plans.", and **See plans**. |
| 4 | A | Press **See plans**, then open **Hoi An old town at lantern time**. | The plan's page: stops, price, when, where to meet, and **Sign up**. No outing strip yet. |
| 5 | A | Press **Sign up**. | The profile questions, **Step 1 of 6**: the name you go by. |
| 6 | A | Name **Zephyr**; date of birth `1990-06-15`; gender **Man**; language **English**; lives in Da Nang–Hoi An **Yes**; tick **I agree to the code of conduct**; **Save**. (**Next** between steps.) | Back on the plan's page, with **Sign up**. |
| 7 | A | Press **Sign up**. | **When can you go?** with a list of dates, each with morning/afternoon/evening boxes (the plan runs in the evening). Nothing earlier than 3 days ahead. |
| 8 | A | Tick **the first evening** in the list and press **Sign up**. Remember which. | "You signed up. You will hear by …", with **Change times** and **Withdraw**. |
| 9 | B, then C | Repeat 1 to 8 with B's email and name (**Marigold**) and C's (**Quillon**), the same evening. | The same. On the **Together** tab each sees "You will hear about Hoi An old town at lantern time". |

Expect throughout: no screen shows another person's name or email.

## Part 2: forming to feedback (not walkable on dev yet)

These steps need the group to be formed and the clock to move, and neither is possible on dev: nothing schedules forming until rounds run on the real clock (milestone 4), and the clock RPC that moves time exists only in the end-to-end build, never in the image. Until then:

- **Web:** `make e2e` walks it for you. Its journey does steps 10 to 17 below, three browsers at phone width, and fails if any "Expect" is not so.
- **iOS simulator and Android emulator:** walk 10 to 17 once forming is scheduled and a round can be run on dev (or against a local end-to-end API with a development build pointed at it, which is not set up: see `e2e/README.md`, "Not done").

| # | Sign in as | Do | Expect |
|---|---|---|---|
| 10 | (the plan's round forms) | Form the plan's groups for the three. | A, B and C are each in one outing, invited for 48 hours. |
| 11 | A, B, C | Open **Together**. | **Needs you**: the plan, its date, "You share: English" and **Yes** and **No**. **No names**: not on Together, not on the plan's page, and not in what the API sends. |
| 12 | A | Press **Yes**. | "You said yes. It is set when 3 have." Still no names. Same for B. |
| 13 | C | Press **Yes**. | "You're going". The outing is **Set**. A and B, on opening the plan, are going too. |
| 14 | A, B, C | Open the plan's page. | **Your outing**: "You're going", "Stage: Set", and **Going with:** the other two first names. "Check-in opens …" and no **I'm here**. |
| 15 | A, B, C | Move the clock to 1 minute before check-in opens, then to the moment it does (30 minutes before the start). | Before: still no **I'm here**. At the moment: **I'm here** appears. |
| 16 | A, then B, then C | Press **I'm here**. | "You have checked in." A: "No one else has checked in yet." B: "Also here: Zephyr". C: "Also here: Zephyr, Marigold" (first names, any order). |
| 17 | A, B, C | Move the clock past the plan's stated end; open **Together**, then press **Give feedback** (the form is on the plan's page). | "How did it go? Feedback closes …"; **Did you go?** **Yes**; the others ticked under **Who else was there?**; pick one under **Who would you meet again?**; **Save my answers**: "You said you went. You can change your answers until feedback closes." Nobody is told who picked them. |

## What to write down

On each platform, any step where a screen was cut off, a button was under the keyboard or the home bar, a name showed that should not have, or the sign-in code took longer than a minute to arrive. A step that fails on a phone and passes on the web is the one this walk exists to find.
