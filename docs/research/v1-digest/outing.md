# The outing

What happens between an invitation and the last piece of feedback. Sources are cited as in the [README](README.md). The rules for one person live in `plans.Membership` on the server (`backend/internal/plans/membership.go`) and in `outing(event, now)` in the app (`apps/mobile/src/features/events/outing.ts`).

## What v1 decided

### What an outing is

An **outing** is "a plan with a date and a group of people invited to it, from their invitation until the feedback window closes" (Glossary). One round of a plan makes one outing, which can have several groups; the groups share the plan's venues (Log, 2026-10-22). A person is in at most one group per outing.

### The stages

A **stage** is "where an outing stands for one person" (Glossary):

| Stage | What it means | What the person can do |
|---|---|---|
| **Invited** | A date is set: can you go? | Confirm or decline, until the start of the day before the outing (local time). A decline can be changed to a yes until then. |
| **Going** | They confirmed. | Cancel their place until the start. See the first names of everyone confirmed in their group. Chat from 48 hours before. See the meeting point from 24 hours before. Share the outing. |
| **On the day** | Until check-in closes. | Check in ("I'm here") from 30 minutes before the start to 3 hours after. |
| **After** | How was it? | Give feedback, from 3 hours after the start, for 14 days. |

An outing that is over, or that the person is out of, is **muted**; there is no separate "over" stage (Log, 2026-11-02). The app shows the stages as five knots on a gold thread on one outing screen whose content follows the stage (Log, 2026-10-26).

### Going ahead

| Decision | Source |
|---|---|
| A group is **confirmed as soon as enough people say yes** (the group's target size, its quorum). | Log, 2026-10-21 |
| At the answer deadline a group **goes ahead with at least 2** confirmed; otherwise it is cancelled. The people who had confirmed a cancelled group go back on the plan's waiting list. | Log, 2026-10-21; Backend README, step 0 |
| A **cancellation** that takes a confirmed group below 2 sends it back to collecting answers, or cancels it after the deadline. | Backend README, `RespondToEvent` |
| **Who said yes is hidden until you confirm.** Before that a person sees how many confirmed, never who. | Backend README, `GetEvent`; Log, 2026-10-27 |
| The **meeting point** is shown to confirmed people from 24 hours before. A cancelled outing's meeting point is never shown. | Backend README; Log, 2026-10-29 |
| A person **suspended** before the start is taken out as if they had declined or cancelled. | Log, 2026-10-24 |

### Attended and met

| Decision | Source |
|---|---|
| **Attended**: "they said they could go, their group went ahead, and they checked in or said in their feedback that they went. If their feedback says they didn't go, that wins over a check-in." | Glossary; Log, 2026-11-04 |
| **Met**: "two people have met when they both attended the same group at an outing." | Glossary |
| Both are defined once, as database views, because four queries each carried their own drifting copy. | Log, 2026-11-04 |
| Check-in needs no proof of place. "I went" in feedback counts the same as a check-in. | Backend README; `docs/architecture.md`, "Known risks" |
| Check-ins count towards **reliability**, which matching weighs. | Backend README, `CheckIn` |

### Feedback

| Decision | Source |
|---|---|
| Feedback asks: **did you go**; a rating of the **plan** and of the **group** (1–5, required if they went); **who they'd meet again**; and **who they'd rather not** be grouped with again. A person can't be in both lists. A new answer replaces the old. | Backend README, `SubmitFeedback` |
| It is open **only to people who went ahead with their group**, from 3 hours after the start, for 14 days. | Log, 2026-10-23 |
| Picks are **private**. A mutual pick is never announced; the feed's friend count is the only hint. | Log, 2026-10-23 |
| A **"rather not"** keeps the pair apart like a report, without a moderator. It exists because a group rating can't say who, and a report is too heavy for "we didn't click". | Log, 2026-10-23 |
| After feedback, a person's picks are checked for a **lean towards one gender** (6 or more picks, 80% or more to one gender, p < 1%, compared with who they met). It only raises a flag for a moderator. | Log, 2026-10-23 |
| **Reports** can be made from anyone who confirmed (even if they cancelled since), until 30 days after the start ([safety.md](safety.md)). | Log, 2026-10-23 |

### Decided on tickets after the log froze

| Decision | Built? | Source |
|---|---|---|
| **People who have checked in can't cancel.** | No. The app hides Cancel after check-in; the server still allows it. | Owner, 2026-11-04; [MyEvent carries what the caller can do when](https://github.com/shrujancheruku/issho/issues/329) |
| **"On the day" starts at 02:00 city time.** | No. The app decides it in the device's time zone. | Same |
| **No stage field from the server**: the API sends the windows for each action, and the app derives the stage. | No | Same |
| **"Goes ahead" = the group is confirmed and the outing isn't cancelled.** Groups still collecting answers when the outing finishes are cancelled. | No | Owner, 2026-11-04; [one definition of "goes ahead" and "feedback due"](https://github.com/shrujancheruku/issho/issues/331) |
| Times show in the device's time zone; **tapping shows the city's time**. | No (Ideas) | [an outing's time in the city's time zone on tap](https://github.com/shrujancheruku/issho/issues/333) |
| **Reminders** at 09:00 on the day and when check-in opens. | No | [notifications.md](notifications.md) |
| The outing screen **works offline on the day**: the cached outing, meeting place and group, with check-in saying it needs a connection. | No | [the outing screen works offline on the day](https://github.com/shrujancheruku/issho/issues/484) |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| Cancel a group when one of three declines | Confirm at quorum; go ahead at the deadline with 2 | "A decline in a group of 3 shouldn't cancel an event that can still happen with 2." | Log, 2026-10-21 |
| Outings shown in the outing's own languages | Shown in the reader's languages | Since plans got titles, every outing screen used the reader's languages anyway | Log, 2026-10-27 |
| The rules decided from different rows in four files; the app copying the 30-day report limit | One module on the server, and windows sent to the app | Drift | Log, 2026-10-29 |
| Numbered stages in the app | Named stages from one pure function | Comments counted the numbers differently | Log, 2026-11-02 |
| Four copies of "attended" and "met" | One view each | The newcomer count ignored whether the group went ahead; met pairs needed feedback from both | Log, 2026-11-04 |
| Checking in then cancelling allowed | Not allowed (decided, unbuilt) | Owner | [Architecture review, 2026-11-04](https://github.com/shrujancheruku/issho/issues/329) |

## Left open, parked, cut or only planned

- **On the day, the group sees who has checked in.** The redesign shows the group's seats filling as people arrive; the API only says whether the caller has. Briefed, frozen, in Ideas ([the group sees who has checked in](https://github.com/shrujancheruku/issho/issues/397)).
- **A group stuck collecting answers at a finished outing** counts as having gone ahead. Recorded as a live defect ([one definition of "goes ahead"](https://github.com/shrujancheruku/issho/issues/331), open).
- **A cancelled group still offers a share link** in the app; the server treats it as over. Closed as covered by the two tickets above.
- **Wind-downs** would add an optional last step to the outing ([plans-and-catalogue.md](plans-and-catalogue.md)).
- **Photos** of an outing were never built on any platform ([Web for launch](https://github.com/shrujancheruku/issho/issues/340)).
- **Check-in by location** was never built; check-in is a tap.
- **No-shows** have no consequence beyond the reliability weight and a `NO_SHOW` report reason. Nothing limits a person who repeatedly confirms and doesn't come.
- **What "lost to a bug" means** for the Pilot's bar ("no outing fails because of a bug") was shaped on a ticket and not built ([Metrics computes the Pilot's success bar](https://github.com/shrujancheruku/issho/issues/454)).

## Where the code and the documents disagree

- **Outing or event.** The Glossary says outing; the schema, API and code say event, with the Glossary's leave. The rename is an open ticket.
- **Two homes for the rules.** The mobile README calls `outing.ts` "the one place the rules about an outing live", while `plans.Membership` holds the same rules on the server. They disagree on cancelling after check-in, on sharing a cancelled group and on when "on the day" starts.
- **"Went ahead" is written more than once.** The view says the outing is done and the group wasn't cancelled; the feedback rule is written in three places (`membership.go`, `events.sql`, `feedback.sql`).
- **Feedback and reports are documented as "(M7)" and "(M8)"** in the Backend README, which are retired milestones.
