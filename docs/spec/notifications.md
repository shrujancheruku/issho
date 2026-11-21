# Notifications

From [Notifications](https://github.com/shrujancheruku/issho/issues/554), amended by [Languages](https://github.com/shrujancheruku/issho/issues/556) and [The v2 glossary and domain model](https://github.com/shrujancheruku/issho/issues/569). What staff receive is in [Running Issho](running-issho.md#alerts).

## Channels

- **Push and email only.** No SMS, Zalo or WhatsApp.
- **Push** is the default for everything.
- **Email** does what push can't: it reaches a person with no push, and carries anything a person may need to keep or reply to.
- **No inbox.** The app has no notifications list. Every message is about something with its own screen (an outing, a plan, a crew, the person's standing), and that screen shows the current state, so a missed push loses nothing. "Needs you" does the rest ([The app's shape](app-shape.md)).
- **No browser push** at any stage. A person who uses only the web gets email under the fallback rule and no chat notifications.

## When a push also goes by email

1. The person has no device registered for push (the web, permission refused, signed out): everything except chat goes by email instead.
2. The message needs an answer and is unanswered halfway through its window (an invitation at 24 of its 48 hours).
3. A change or cancellation inside the last 24 hours.

Chat never goes by email.

## Message types, and what a person can turn off

| Message type | Can it be turned off? |
|---|---|
| **Needs your answer** | No |
| **About your outing** | No |
| **About you** | No |
| **Chat** | Per room, except an outing's room on the day |
| **Nudge** | Yes, one switch for all of them |

The one switch and the room mutes are the whole of notification settings.

## Quiet hours

- Nothing arrives between **22:00 and 08:00 on the city's clock**; it is held until 08:00.
- Two exceptions: a change or cancellation to an outing starting before noon, and chat messages, which arrive silently.
- A window starts when the message is delivered, not when it was held, so quiet hours never eat into an invitation's 48 hours or an offered seat's 24.

## Chat pushes

One notification per room, updated in place. It sounds for the first message, then stays silent until the person opens the room or 10 minutes pass with nothing new.

## Language, dates and wording

- A message arrives in the person's one **display language**.
- The body carries the **date and time on the city's clock** ("Sat 28 Nov, 15:00"), with the person's own time added only when their time zone differs.
- The title is the plan's title or the crew's name. The body states what happened, the date and time, and the one thing to do.
- No guilt, no pressure, no exclamation marks, no emoji.
- Other people appear by first name only, and never in a message about conduct.
- **Lock screen.** Pushes about reports, warnings, pauses and no-shows say only "An update about your account"; the detail is inside the app. Every other push says everything, chat included (first name and the message).

## Permission, devices and expiry

- Push permission is asked right after the first signup. A refusal is never nagged: the person gets email, and one line on their first set outing says how to turn push on.
- Tapping a message opens the outing, plan, crew or room it is about. Every signed-in device gets the push; signing out stops them on that device; answering on one device withdraws the push from the others.
- A message that is no longer true is never sent (an invitation past its deadline, a reminder after check-in closes), and a delivered push that has gone stale is withdrawn where the platform allows.

## The moments

### Needs your answer

Push; email at the halfway point.

| Moment | Window |
|---|---|
| Invitation: a date is set, can you go? | 48 hours |
| Offered seat on a set outing | 24 hours, or until the 24-hour mark |
| "Still going?" after a change of day or of more than an hour | Up to 24 hours |
| "Keep waiting for the next round?", which is also the notice that no group formed | Until the next round forms groups |
| Crew offer | 7 days |
| A crew member proposes a plan | 7 days |
| A place in the crew, offered to a guest | 7 days |
| "Still in?" to a quiet crew member, and "keep this crew?" | 7 days |

The guest question ("would you want them in the crew?") is asked inside feedback and has no message of its own.

### About your outing

Push.

- **Set**: the date and time, to everyone who said yes.
- **Not set**: too few said yes; the person comes first next time.
- **Full**: they said yes after the last seat went.
- **Changed** and **cancelled**, with the reason; also by email inside the last 24 hours.
- **The only one who came**: "we're sorry".
- Someone added, someone cancelling and "one smaller" are system lines in the outing's room and reach people as chat pushes.

### Reminders

To people going.

1. **30 hours before**: the day and time, and when free cancelling ends. Skipped for someone who took a seat later than that.
2. **09:00 on the day**, or 20:00 the evening before when the start is before 10:00.
3. **When check-in opens**, 30 minutes before the start, to those not yet checked in.

### About you

| Moment | How |
|---|---|
| Recorded as a no-show: sent 24 hours after the plan's end if still counted absent, with "I was there" one tap away | Push |
| Paused, with the end date; and when the pause ends | Push and email |
| Warned or suspended | Email and in the app |
| Appeal answered; join links taken away | Email |
| Report received; report closed | Push |
| A plan the person signed up to was withdrawn | Push |

A late cancel gets no message: the cancel button states the cost before the tap.

### Nudges

Push; one switch; at most one a day.

- Feedback: the ask at the plan's stated end, and one reminder 3 days later to those who haven't answered. Nothing after that.
- Issho's weekly proposal to a crew.
- A crew has formed; a crew has closed.
- A suggestion of the person's was used in a plan.
- A friend has joined by the person's join link.
- **New plans this week**: one weekly message, only to an active person with no signup or outing in play, and only when there is a plan they haven't seen. It is the only message that isn't a reply to something the person did.

## No marketing

No newsletter, no "we miss you", no streaks, no counts of people waiting and no promotional email at any stage. Every email is about the person's own account or outing.

## The waitlist

Waitlisted people get exactly three emails: a confirmation on joining, "your place is open" when their wave is admitted, and one reminder 7 days later if they haven't signed in. They come in the language the person was reading when they joined. Each carries a "leave the waitlist" link ([Privacy](privacy.md#waitlist-emails)).

## Suspended and paused people

- A **suspended** person gets only the suspension notice and the appeal answer.
- A **paused** person still gets everything: they keep their crews, rooms and any outing already set.

## Email replies

Emails come from one Issho address. Replies are read only for a warning or suspension, where the reply is the appeal. Every other email says replies aren't read and points into the app.
