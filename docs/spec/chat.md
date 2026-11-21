# Chat

From [Chat](https://github.com/shrujancheruku/issho/issues/551), amended by [Notifications](https://github.com/shrujancheruku/issho/issues/554) and [Privacy and legal obligations in the product](https://github.com/shrujancheruku/issho/issues/559).

## Rooms

- There are exactly two types of **room**: an outing's room and a crew's room. No direct messages, no chat among a plan's signups before a group exists, and no chat with Issho.
- Direct messages are left out because they are where harassment happens and they would bypass the crew as the way to see people again. A signups chat would expose people who may never be grouped together.
- A room is open, read-only or gone.

## An outing's room

- It opens the moment the outing is set, for everyone who is going.
- Someone who takes an empty seat later sees the whole history.
- It becomes **read-only** 24 hours after the plan's end and stays readable until it is deleted. It is not extended to cover the 7-day feedback window: the crew is the way to keep talking.
- **Cancelled outing**: a system line says so, the room goes read-only 24 hours later, and it is deleted on the usual 30 days.
- **Changed outing** (place, time or day): the room carries on, a system line states the change, and the closing time follows the new date.

## A crew's room

- It is open as long as the crew is.
- A crew outing has no room of its own; it lives in the crew's room as a card.
- When the crew closes, the room goes read-only with a system line and stays readable for 30 days to those who were crew members that day.

## What can be sent

- Text only, 1 to 1,000 characters, shown under the sender's first name.
- **Reactions** from a small fixed set.
- A one-tap **"running late"** on the day, which posts an ordinary message with an optional 5, 10 or 15 minutes.
- Links are tappable and have no preview, so nothing is fetched by the device or the server.
- No photos, replies, edits or deletes. The room is the evidence behind a report.
- Typing dots, an unread count and last message per room, and a limit of 20 messages a minute per person.

## Issho's voice

- Issho speaks in a room only through **system lines**, which have no sender: who is going, who was added, who can't make it or has left, check-ins, changes to the outing, and plan cards in a crew's room.
- Each person sees a system line in their own display language ([Languages](languages.md)).
- It never sends prompts, icebreakers or marketing, and nobody can reply to Issho there.

## Translation

- Messages are shown as written and the server translates nothing. A group shares a language, and server translation would send private messages to a third party.
- Each room has an on-device **translate toggle**.
  - Per person and per room, off by default, remembered.
  - Each message not in the person's display language gets its translation underneath the original, which stays visible; the translation is marked as one.
  - It runs only on the device, using the system's own translation, so no message leaves the device. On Android the translation is credited to Google, as its terms ask ([Architecture](architecture.md#on-device-translation)).
  - Where the system can't do it (a device, a language pair, most browsers) the toggle isn't shown.
  - Nobody else sees who has it on.

## What a moderator can read

- A moderator reads a room only behind a report, while the report is open and for 30 days after it closes. Every read is in the staff log. Nobody at Issho can otherwise read a room.
- A report can be started from a message with a long-press. It still names the person, and the whole room is the evidence.
- The messages attached to a report may be machine-translated for the moderator. This is the one case where chat text goes to a translation service, and the privacy policy says so ([Privacy](privacy.md#who-receives-personal-data)).
- The app says plainly that a room can be read if someone reports it.
- Messages are moderated after posting; nothing filters them beforehand.

## How long messages are kept

- An outing's messages are deleted 30 days after the room goes read-only.
- A crew's messages are kept while it is open and deleted 30 days after it closes.
- An open report holds the room until 30 days after the report closes.
- A person can be reported from a room only while that room still exists, so a report can't arrive after its evidence is gone.

## When someone leaves

- Cancelling after the room opened, being removed or suspended, or leaving a crew ends access to the room at once, for reading and writing.
- A system line says they can't make it or have left, with no reason given.
- Their messages stay under their first name until the room is deleted.
- When an account is deleted, its messages stay until the room's own 30 days run out, shown as from "someone who left".

## Push

- A new message sends a push, in both types of room. Chat never goes by email.
- One notification per room, updated in place. It sounds for the first message, then stays silent until the person opens the room or 10 minutes pass with nothing new. In quiet hours chat pushes arrive silently.
- Each room can be muted, except an outing's room on the day itself.
