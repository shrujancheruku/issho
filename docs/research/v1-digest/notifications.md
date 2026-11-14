# Notifications

What Issho tells people, when and how. Sources are cited as in the [README](README.md). The code is `backend/internal/notifications`, `backend/internal/push` and `apps/mobile/src/lib/push.ts`.

## What v1 decided

### What is sent

Eight kinds of push exist (`backend/internal/plans/events.go`, `backend/internal/crews/notifications.go`; wording from `apps/mobile/src/i18n/en.json`, `push.*`):

| Kind | When | Body |
|---|---|---|
| Invited | A round the person signed up for has a date | "A date is set. Can you go? Open Issho to answer." |
| Group confirmed | Enough people confirmed, at quorum or at the deadline | "Enough people confirmed, so it is going ahead." |
| Group cancelled | Too few could go | "Not enough people could go, so it is cancelled." |
| Plan closed without a group | To whoever was still waiting; opens the feed | "Not enough people signed up this time, so it is closed. Open Issho to see other plans." |
| Feedback | 3 hours after the start, to everyone who went ahead with a group, once | "How was it? Tell us, and pick who you would meet again." |
| Crew offered | A crew is offered | "You got on well. Want to keep meeting as a crew?" |
| Crew formed | Enough people accepted | "Your crew has started. Say hello!" |
| Crew suggestion | A plan is suggested in the crew's chat | "A plan for your crew has been suggested." |

The title is the plan's title, or the crew's name. Bodies carry **no dates**, because the server can't format them per language (Backend README, "Push notifications").

### Rules

| Decision | Source |
|---|---|
| **Push is the only channel.** The only email Issho sends is the sign-in code. There is no in-app inbox and no SMS. | Backend README; `supabase/templates/` |
| **No push on the web.** | Log, 2026-10-21 (owner) |
| **Ask for permission right after a first plan signup**, not at app start: asking when the person has just said "tell me when there's a date" makes a yes far more likely. | Log, 2026-10-22 |
| Bodies are in the person's **primary display language**. | [reminders on the day and at check-in](https://github.com/shrujancheruku/issho/issues/482) (current behaviour) |
| **Suspended people aren't sent pushes.** | Glossary, "Suspended" |
| **No push for a chat message.** | The kinds above |
| **A person can't choose which pushes they get.** The app has no notification settings. | `apps/mobile/src/app/(tabs)/profile.tsx` |
| Tapping a push opens the outing, the feed or the crew it is about; the app also refreshes the affected screens when a push arrives while it's open. | Log, 2026-11-04 |

### How it works

| Decision | Source |
|---|---|
| **An outbox.** The notification is written in the same transaction as the change, so a rollback never notifies and a crash never loses one. A job sends them every minute. | Log, 2026-10-22 |
| Sent through **Expo Push**; Android goes through Firebase Cloud Messaging underneath. | Log, 2026-10-22 |
| **Nobody is pushed twice**: a partial send keeps what Expo accepted and retries only the rest; 5 attempts. | Log, 2026-11-02 |
| The push token is remembered **per account**, and every sign-out unregisters it. | Log, 2026-11-02 |
| Notifications are **one module** that plans and crews register their wording with. | Log, 2026-11-02 |
| Rows are deleted after 30 days. | Backend README, "Retention" |

### Decided on tickets after the log froze

| Decision | Built? | Source |
|---|---|---|
| **Two reminders** for confirmed people: at **09:00 city time on the day** (20:00 the evening before when the outing starts before 10:00), and **when check-in opens**, to those not yet checked in. "Nothing reminds people on the day or when check-in opens, which is when a no-show is decided." | No | [reminders on the day and at check-in; old pushes expire](https://github.com/shrujancheruku/issho/issues/482) |
| **Pushes expire**: an invitation at the answer deadline; confirmed or cancelled at the start plus 24 hours; reminders when check-in closes; feedback when its window closes; a crew offer when it closes; the rest after 7 days. An expired push is never sent. | No | Same |
| **A push for each new report to the owner** was considered and replaced by a Sentry alert. | — | [the owner hears about a new report straight away](https://github.com/shrujancheruku/issho/issues/478) |
| **A test that every push kind has its wording**: a renamed key silently sends an empty body today. | No | [pin that every push kind's catalogue key exists](https://github.com/shrujancheruku/issho/issues/325) |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| iOS push with the first release | Deferred twice, to the Launch stage | Needs the paid Apple Developer Program | Log, 2026-10-22, 2026-10-29 |
| One notifier inside `plans` that also wrote crews' wording | One notifications module | Crews depend on plans, not the reverse | Log, 2026-11-02 |
| A failed batch marks every notification failed | Keeps what was accepted | People got the earlier batches again | Log, 2026-11-02 |
| The token remembered per device | Per account | The next account on a phone got nothing | Log, 2026-11-02 |
| Wording for a deleted plan or crew sent blank or failed five times | Skipped | — | Log, 2026-11-03 |

## Left open, parked, cut or only planned

- **iOS push has never been sent**: there is no Apple account yet.
- **Reminders and expiry**: decided, unbuilt. Today a backlog can still send an old "please answer".
- **Push bodies in the secondary display language too**: Ideas.
- **Notification settings** for a person: never considered.
- **Email or another channel as a fallback** for someone without push (the web, or permission refused): never decided. A web user gets no notification of any kind.
- **A push when the meeting point is revealed**, **when someone joins or cancels**, or **when a message arrives**: none exist; none were decided against in writing either.
- **Quiet hours**: nothing recorded. The matching pass runs hourly round the clock, so an invitation can be sent at any hour.

## Where the code and the documents disagree

- `backend/README.md:278` lists push notifications under "Not done yet". They exist.
- The mobile README's push section and `lib/push.ts` list plan and group pushes only; crews send three more.
- The log says iOS push was "deferred to M9", then moved; the mobile README still says "deferred to M9".
