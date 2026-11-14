# Chat

Group chat for an outing and for a crew. Sources are cited as in the [README](README.md). The code is `backend/internal/chat` and `backend/internal/chatws`, and `apps/mobile/src/features/chat`.

## What v1 decided

### Rooms

A **room** is "one chat: an outing's group chat or a crew's chat" (Glossary). There are exactly two kinds. There is no direct messaging between two people, no chat for a plan before it has a group, and no chat with Issho.

| Decision | Source |
|---|---|
| An **outing's room** is for the people **going** (confirmed) in one group. It isn't available in a cancelled group. | Backend README, `ListMessages` |
| It is open for writing **from 48 hours before the start to 24 hours after**. | Backend README, `SendMessage`; `chat.OpensBefore`, `chat.ClosesAfter` |
| A **crew's room** is for members of an active crew, and is open **whenever the member may read it**. | Backend README, `SendMessage` |
| Messages are **text only, 1 to 1,000 characters**. No photos, reactions, replies, edits or deletes exist. | Backend README, `SendMessage`; `proto/issho/v1/chat.proto` |
| Each message shows the sender's **first name**. | Backend README, `/ws/chat` |
| A crew's room also carries **suggested plans** as cards with no sender. | Backend README, `ListMessages` |
| **Typing dots**: shown to the others in the room, never stored, at most one notice every 2 seconds. | Log, 2026-10-26 |
| **Retention: 30 days.** An outing's messages are deleted 30 days after its date; a crew's 30 days after the crew closes or lapses. A member leaving an active crew deletes nothing. | Backend README, "Retention"; `chat.Keep` |
| A **suspended** person is refused at once, on every frame they send. Someone removed from a group while waiting can't post. | Log, 2026-11-04 |
| Support for people is **outside the app** (a WhatsApp or Zalo group) for the Pilot. | [What the Pilot is](https://github.com/shrujancheruku/issho/issues/412) |

### How it is shown

| Decision | Source |
|---|---|
| Messages are **shown as written**. Nothing translates a chat message. On-device translation "stays an option for user-written text such as chat". | Log, 2026-10-20 |
| The app's **Together** tab lists the person's groups (outings and crews), and the chat sits on the outing screen and the crew screen. | Log, 2026-10-26 |
| The app decides whether a room is open from the **server's** answer; the device clock only schedules asking again. | Log, 2026-11-02 |

### How it works

| Decision | Source |
|---|---|
| A **plain WebSocket**, not Connect streaming, because React Native's fetch can't stream. The token goes in the first frame, never the URL. | Log, 2026-10-22 |
| Postgres `NOTIFY` carries new messages between API instances; no Redis. | Log, 2026-10-22 |
| **One poster per room at a time** (a room lock), so messages commit in order and a reader can't skip one. | Log, 2026-11-02 |
| **One rooms table and one messages table** for both kinds. | Glossary, "Room"; merged 2026-11 as [one rooms table and one messages table](https://github.com/shrujancheruku/issho/issues/494) |
| No push is sent for a chat message. The push kinds are listed in [notifications.md](notifications.md). | `backend/internal/plans/events.go`, `backend/internal/crews/notifications.go` |

### Decided on tickets after the log froze

| Decision | Built? | Source |
|---|---|---|
| Rooms report an **unread count and the last message**, and opening a room marks it read. The Together tab was designed around both. | No | [chat rooms report their unread count and last message](https://github.com/shrujancheruku/issho/issues/396) |
| **Moderators can read the room behind a report**, while the report is open and for 30 days after it closes; each read is logged. Retention keeps a reported room until then. Reporting a single message is not in the Pilot: "a report names the person, and the room is the evidence." | No | [moderators can read the chat behind a report](https://github.com/shrujancheruku/issho/issues/485) |
| **Rate limit: 20 messages a minute** per person. Over it, the app shows "slow down" and keeps the draft. | No | [rate limits on chat, reports, activity suggestions and sign-up](https://github.com/shrujancheruku/issho/issues/481) |
| A **client message id**, so a retry on a weak network never posts twice. | No | [SendMessage takes a client message id](https://github.com/shrujancheruku/issho/issues/499) |
| **Moderating after posting is enough** for the stores (reporting, blocking, the owner told of every report, action within 24 hours). Filtering before posting becomes a gate only if Apple rejects this. | Position only | [Distribution for each platform](https://github.com/shrujancheruku/issho/issues/343) |
| **Blocking hides the blocked person's messages** from the blocker in shared rooms. | No (Ideas) | [Ideas](https://github.com/shrujancheruku/issho/issues/360) |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| Crews copying outing chat's listing and posting; a socket per component in the app | One chat module, one socket per room | Chat ids are assigned on insert but seen on commit, so a reader could skip a message for good | Log, 2026-11-02 |
| Two tables kept "because crew rooms carry suggested plans" | One rooms table and one messages table | Near-identical storage paths and two retention deletes | Log, 2026-11-02; [one rooms table and one messages table](https://github.com/shrujancheruku/issho/issues/494) |
| The app deciding "open" from the device clock | From the server | Drift | Log, 2026-11-02 |
| Access checked before taking the room lock | Checked under the lock | Someone removed while waiting could post | Log, 2026-11-04 |

## Left open, parked, cut or only planned

- **Nothing a moderator can read today.** There is no way to report a message or for a moderator to see a room. Outing chats are deleted 30 days after the date, the same 30 days as the report window, "so a late report can arrive after the evidence is gone".
- **Unread counts**: the rows on Together show only the room's name.
- **No push for new messages**, and nothing refreshes the chat list when one arrives.
- **Photos** were listed as an open item for the web and "not built on any platform yet" ([Web for launch](https://github.com/shrujancheruku/issho/issues/340)).
- **Translating chat messages** was never decided beyond "an option".
- **Chat before a group exists** (for a plan's signups) was never considered.
- **The chat retries only twice** in the app on a failed connection; backoff was decided with the offline ticket and not built.

## Where the code and the documents disagree

- `backend/README.md:278` lists chat under "Not done yet". It exists.
- `docs/architecture.md` describes "`rooms`, one per event group or crew, and `messages`", which matches the merged schema; the 2026-11-02 log row says "the two tables stay". The log is frozen and older.
- The Glossary's **Room** entry and the code agree.
