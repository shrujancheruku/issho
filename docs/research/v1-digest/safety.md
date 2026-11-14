# Safety

Reports, keeping people apart, suspension and moderation. Sources are cited as in the [README](README.md). The code is `backend/internal/moderation`, the report and feedback parts of `backend/internal/plans`, and the admin SPA's Reports, Flags, People and Person pages.

## What v1 decided

### Built into the product's shape

| Decision | Source |
|---|---|
| **Groups, not pairs, and no browsing.** Nobody can look up, message or pick a person they haven't been grouped with. | Log, 2026-10-21 |
| **Adults only**, by stated birth year. | Log, 2026-10-21 |
| **Platonic by rule**: the code of conduct, with "here for dating" as a report reason. | Log, 2026-10-21 |
| **Same-gender-only groups** for anyone who asks: a hard rule. | Log, 2026-10-21 |
| **Nobody the only woman or the only man**: a preference, not a rule. | Log, 2026-10-21 |
| **Public venues**, picked by Issho from discovered places; the conduct says "meet in the agreed public place". | `conduct.rule.safe` |
| **The meeting point is shown only to confirmed people**, from 24 hours before. | Backend README |
| **Venue screening**: names suggesting a snack bar, hostess bar or tattoo studio are flagged for the admin. Japanese snack bars are typed `bar` by Google. | Log, 2026-10-24; `docs/architecture.md`, "Known risks" |

### Reports

| Decision | Source |
|---|---|
| A person can **report someone in their group** (or crew) with a reason: **dating, harassment, unsafe, no-show, other**, and optional details up to 1,000 characters. | Backend README, `ReportPerson` |
| Reports are open **until 30 days after the start**, to **anyone who confirmed**, including people who cancelled: "reports cover the chat before the event and things noticed later". | Log, 2026-10-23 |
| A second report about the same person and outing replaces the first. | Backend README |
| **A report keeps the pair apart for good, whatever the moderator decides.** "The reporter shouldn't meet them again either way." | Log, 2026-10-23 |
| A report is **kept if the reporter deletes their account**. | Log, 2026-10-23 |
| The person reported is **not told**: no push kind or screen exists for it. | `backend/internal/plans/events.go` (the push kinds) |

### Kept apart

**Kept apart**: "two people who are never grouped or offered a crew together again: one reported the other, or said they'd rather not be in a group with them again" (Glossary).

| Decision | Source |
|---|---|
| A **"rather not" pick** in feedback is private and keeps the pair apart like a report, with no moderator. | Log, 2026-10-23 |
| **It's for good**, "even if the person changes their feedback later". | Glossary; owner, 2026-11-06 |
| Becoming kept apart **takes the person reported or not wanted out of any upcoming outing or crew the two share, without telling them why**. Outings under way are left alone. | Glossary; owner, 2026-11-06 |
| **Suspended** and **kept apart** are different words for different things; "blocked" and "banned" are avoided. | Glossary |

### Suspension

| Decision | Source |
|---|---|
| **Suspended**: "someone is suspended while they have a suspension that hasn't been lifted. Suspended people aren't matched, offered crews or sent pushes." | Glossary |
| **Enforced by the API on every call**, not by the auth provider's ban, so it takes effect at once. The app shows "Your account is suspended" with sign-out and delete. | Log, 2026-10-24 |
| Suspending **takes the person out of upcoming outings and crews** as if they had cancelled or left. **Lifting doesn't put them back**: "putting them back later could land them in a group with the person who reported them." | Log, 2026-10-24 |
| An admin suspends with a reason and lifts with a note; an admin can't suspend themselves. | Backend README, `ModerationService` |
| **A suspended person who deletes their account leaves a tombstone**: a keyed hash of their email plus the suspension and its reasons. A later sign-up with that email starts suspended. Nothing is kept for someone not suspended. | ADR 0003 |

### Moderation

| Decision | Source |
|---|---|
| **A moderation queue** in the admin SPA: reports (open oldest first), resolved as actioned or dismissed with a note. Decisions on a report are **recorded, not enforced**; only suspension changes what a person can do. | Log, 2026-10-23 |
| A **person view** for admins: profile basics, report counts, flags, suspensions, the last 50 outings; search by name, email or id. | Backend README |
| **A gender-lean flag**: someone whose picks lean heavily to one gender (6 or more picks, 80% or more, p < 1%, against who they met) is flagged. "A statistical signal isn't proof: a moderator decides." | Log, 2026-10-23 |
| **Why people left crews** is listed for admins, with "made me uncomfortable" highlighted. | Log, 2026-10-23 |
| **Irreversible admin actions ask first**: suspending, lifting, approving or rejecting. | Log, 2026-11-03 |
| **Plan and group review** per city lets the owner control what opens and who meets. | Log, 2026-10-24 |

### Decided on tickets after the log froze

| Decision | Built? | Source |
|---|---|---|
| **Blocking**: both app stores require it. A block is "a report with no moderator queue": the same kept-apart pair, plus a button. **No unblocking in the Pilot.** | No | [block someone](https://github.com/shrujancheruku/issho/issues/404) |
| **The owner hears about a new report at once**, through a Sentry alert that carries only the report's id. | No | [the owner hears about a new report straight away](https://github.com/shrujancheruku/issho/issues/478) |
| **Moderators can read the chat behind a report** ([chat.md](chat.md)). | No | [moderators can read the chat behind a report](https://github.com/shrujancheruku/issho/issues/485) |
| **Action within 24 hours** on a report is the position argued to the stores. | Position | [Distribution for each platform](https://github.com/shrujancheruku/issho/issues/343) |
| **Rate limits on reports**: 5 an hour. | No | [rate limits on chat, reports, activity suggestions and sign-up](https://github.com/shrujancheruku/issho/issues/481) |
| **A runbook** for a safety incident and a data breach. | No | [a runbook for a safety incident and a data breach](https://github.com/shrujancheruku/issho/issues/487) |
| **"No safety incident goes unhandled"** is one of the Pilot's four success measures. | — | [What the Pilot is](https://github.com/shrujancheruku/issho/issues/412) |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| Moderation decisions recorded only | Suspension added a day later | "Suspension touches sign-in, matching and chats and deserves its own change" | Log, 2026-10-23, 2026-10-24 |
| The auth provider's ban | A gate in the API | A ban leaves an issued token working for up to an hour | Log, 2026-10-24 |
| Suspend, then clean up in separate steps | Atomic: the suspension and its cleanup job in one transaction | A failure left people half-removed | Log, 2026-11-04 |
| Kept apart derived from the current reports and picks (so changing feedback undid it) | For good: a record that only grows (decided, unbuilt) | A pair came back after feedback was resubmitted | [a kept-apart pair comes back after the feedback is resubmitted](https://github.com/shrujancheruku/issho/issues/475) |
| Deleting an account erases the suspension | A tombstone (decided, unbuilt) | A suspended person could delete and sign up again | ADR 0003 |
| No blocking | Minimal blocking for the Pilot | Store rules | ADR 0002 (verdicts) |
| Cloudflare Access in front of the admin site | None | The page is a static shell; the API checks the role on every call | Log, 2026-10-25 |

## Left open, parked, cut or only planned

- **Identity verification**: none ([people.md](people.md)).
- **Nothing happens to a person who is reported**, unless an admin suspends them. There are no warnings, strikes or automatic thresholds.
- **No-shows**: a report reason and a matching weight, nothing else.
- **A pair kept apart can come back** today if the feedback is changed; and **deleting an account erases suspensions and reports against it**. Both are open bugs with decided fixes.
- **Hiding a blocked person's messages**, and **unblocking**: Ideas.
- **Reporting a message**, or reporting someone from outside a shared outing or crew: out of every brief.
- **Emergencies during an outing** (who to call, what Issho does): nothing recorded.
- **Filtering text before it's posted**: argued unnecessary; would become a gate if Apple disagreed.
- **Moderator search can't find Vietnamese names typed without accents** (open bug, [moderator people search finds Vietnamese names without accents](https://github.com/shrujancheruku/issho/issues/483)).
- **Crew reports aren't counted** in the weekly metrics, because the count joins through outings.
- **What happens when the owner is unavailable** was asked and deferred ([Running the soft launch alone](https://github.com/shrujancheruku/issho/issues/345)).

## Where the code and the documents disagree

- **Kept apart is "for good" in the Glossary** and in the owner's decision; in code it is a view over current reports and "rather not" picks, so it can lapse.
- **Removal on becoming kept apart** (Glossary) isn't built.
- **The tombstone** (ADR 0003) isn't built: deletion cascades through suspensions and reports.
- **"Blocked."** The Glossary avoids the word for kept apart; matching's code and the Backend README use `Relations.Blocked` and "blocked pairs". Blocking as a feature will reuse the word for a button.
- **Kept apart lives in the wrong module**: it is a Glossary rule implemented inside `plans` ([a reports module](https://github.com/shrujancheruku/issho/issues/403), open).
- The Backend README and `moderation/handler.go` cite the retired milestone "(M8)".
