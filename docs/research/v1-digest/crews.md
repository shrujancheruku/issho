# Crews

How people who got on keep meeting. Sources are cited as in the [README](README.md). The code is `backend/internal/crews` and `backend/internal/matching/crews.go`.

## What v1 decided

### What a crew is

A **crew** is "a small standing group offered to people who got on well across outings; it suggests a plan each week and has its own chat" (Glossary). The owner's design: "a group chat with weekly plan suggestions". The reasons recorded: a standing chat lets friends keep meeting without new matching, and the suggestions keep it about outings, not just chat (Log, 2026-10-23).

### Who is offered one

| Decision | Source |
|---|---|
| **Crews come from the relationship graph, not from one outing.** A friendship can span outings: A meets B, then C, and B and C meet too. | Log, 2026-10-23 (owner) |
| A crew is offered to a set of **at least 3** people where: everyone has **met** everyone else at some outing; **nobody minded anyone** in it (no report, no "rather not", no declining or leaving a crew together); each has a **mutual pick** inside it; and they **share an activity they like or a home area**. | Log, 2026-10-23; Backend README, "Matching" |
| Someone one member minded is **left out**, rather than blocking the crew. | Log, 2026-10-23 |
| There is **no maximum size**. | [join offers](https://github.com/shrujancheruku/issho/issues/356) (current behaviour) |
| **Of overlapping crews on the same basis, only one is offered**, and nobody is offered the same set of people twice. Two near-identical crews would split the same friends; re-offering what was declined would nag. | Log, 2026-10-23 |
| Suspended people aren't offered crews. | Glossary, "Suspended" |

### The offer

| Decision | Source |
|---|---|
| A crew is **offered to everyone in the set** and forms with **at least 3 who accept within 7 days**. | Log, 2026-10-23; `crews.OfferFor` |
| An answer can change until the offer closes. Once everyone has answered it is decided at once. | Backend README, `RespondToCrew` |
| Too few accepted: the crew **lapses**. A member who never answered is marked lapsed too, so an active crew doesn't list them as still deciding. | Log, 2026-11-02 |
| **Declining counts as minding** the others for future crews, "so nobody is pushed back together". | Log, 2026-10-23 |
| The offer shows members' first names and who has accepted. | Backend README, `ListMyCrews` |

### Life of a crew

| Decision | Source |
|---|---|
| A crew is **named after the shared activity or area**: "{name} crew". The area name is never machine-translated. | Log, 2026-10-23 |
| **"Crew" is an idea, not a word to translate.** Each language uses its own everyday word for a group of friends who keep meeting, avoiding gang, clan, clique, squad or team connotations. | Log, 2026-10-26 (owner) |
| A **plan is suggested** in the crew's chat a day after it forms, then weekly. It appears as a card in the chat; members sign up for it like any plan. | Backend README, step 0b; `crews.SuggestEvery` |
| The crew's **chat is always open** to its members ([chat.md](chat.md)). | Backend README, `SendMessage` |
| **Leaving is silent, always open, and asks why**: uncomfortable, not interested, inactive, too many messages, other, with optional details. The crew isn't told. "The owner wants to learn why crews fail; telling the crew would make leaving harder." | Log, 2026-10-23 |
| Leaving with someone counts as minding them for future crews. A crew left with **one member closes**. | Log, 2026-10-23; Backend README, `LeaveCrew` |
| A member can **report** another member, as in an outing. | Backend README, `ReportCrewMember` |
| A **suspended** person is taken out of their crews; lifting the suspension doesn't put them back. | Log, 2026-10-24 |
| Crew membership **doesn't feed matching**: matching doesn't try to group crew members in ordinary outings. | [the crew screen lists your past outings together](https://github.com/shrujancheruku/issho/issues/399) (out of scope) |

### Decided on tickets after the log froze

| Decision | Built? | Source |
|---|---|---|
| **For the Pilot, the owner proposes crews by hand.** The server works out candidates and lists them in the admin SPA; the owner picks the members and sends the offer. "The server never offers a crew itself." Answering, the quorum of 3, lapsing, weekly suggestions and leaving stay as built. | No. The worker still offers crews itself (`crews/pass.go`). | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417); [Crews the owner proposes](https://github.com/shrujancheruku/issho/issues/453) |
| A **join offer** lets someone new into an existing crew when at least two members picked them: they must be compatible with every member and share the crew's basis; the crew must have fewer than 8 members. Members get a **quiet veto** for 48 hours, and the newcomer never learns of a veto. | No (Ideas). The term is in the Glossary. | [join offers let someone new into an existing crew](https://github.com/shrujancheruku/issho/issues/356) |
| **Fellow crew members see who of them signed up** for a plan: "a narrow exception" to counts-never-names, because they already know each other by name. The signup sheet would say "Your crew will see you signed up", with no opt-out. | No (Ideas) | Owner, 2026-11-05; [fellow crew members see who of them signed up](https://github.com/shrujancheruku/issho/issues/398) |
| The crew screen lists **your past outings together**. | No (Ideas) | [the crew screen lists your past outings together](https://github.com/shrujancheruku/issho/issues/399) |
| A **becoming kept apart** takes the person reported or not wanted out of a shared crew, without telling them why. | No | Glossary, "Kept apart"; [a kept-apart pair comes back after the feedback is resubmitted](https://github.com/shrujancheruku/issho/issues/475) |
| A **crew offer gets its own walkthrough**, built when crews are first offered to real people. | No (Ideas) | Log, 2026-10-29 |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| Crews formed from one good outing (the roadmap's first idea) | From the relationship graph | Friendships span outings | Log, 2026-10-23 |
| The crews pass run inside the matching job, its errors logged as warnings | Its own job, queued after matching, retried on failure | A failure was never retried and one failing crew stopped the rest | Log, 2026-11-02 |
| Members who never answered stayed "still deciding" | They lapse | An active crew listed them forever | Log, 2026-11-02 |
| The app hard-coding 3 | The quorum sent by the API | The app and server could drift | Log, 2026-11-04 |
| Crews out of the Pilot | Crews in, proposed by hand | The logic was built | [What the Pilot is](https://github.com/shrujancheruku/issho/issues/412), amended |
| The server offers crews | The owner offers them (Pilot) | Owner's decision; not built | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |

## Left open, parked, cut or only planned

- **No crew has ever formed with real people.**
- **New people can't join an existing crew.** If two members later get on with someone new, no new crew is offered on the same basis (it would split the friends) and the newcomer isn't invited in.
- **Merging two crews**, and **members inviting someone by hand**, were ruled out of the join-offer brief and never decided.
- **What the candidates page shows**, and which meaning of "lapsed" keeps the word, were open questions on [Crews the owner proposes](https://github.com/shrujancheruku/issho/issues/453).
- **The crew search runs over the whole city graph every hour**: "cheap now, worth limiting to people with new feedback as cities grow."
- **A closed plan suggested in a crew chat still says "Signed up"** with a past date. An open, user-visible bug ([a closed plan suggested in a crew chat says why](https://github.com/shrujancheruku/issho/issues/323)).
- **Short stays work against crews**: the recruiting research noted nomads' short stays "work against the return-attendance metric and crews" ([Communities to recruit from](https://github.com/shrujancheruku/issho/issues/337)).

## Where the code and the documents disagree

- **Who offers.** The Glossary ("offered to people who got on well"), `docs/architecture.md` and the Backend README describe the server offering crews, which is what the code does. The Pilot decision says the owner offers them. The documents and code agree with each other and disagree with the latest decision.
- **Lapsed.** The Glossary defines it as "a crew offer that someone never answered before it was decided" (a member's answer). The code also uses `lapsed` as a crew's status: too few accepted in time.
- **"The four of you."** The app's copy says "a group chat for the four of you" and "suits the four of you"; a crew is any set of 3 or more (`apps/mobile/src/i18n/en.json`).
- **Join offer** is a Glossary term for something that doesn't exist in code.
- The mobile README's push section lists plan and group pushes only; crews send three kinds.
