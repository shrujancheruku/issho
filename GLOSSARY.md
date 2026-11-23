# Issho (v2)

Issho plans outings in a city for a few people who can talk to each other, and gives them an easy way to see the same people again.

This is the v2 glossary and domain model, gathered from the resolutions on the [v2 map](https://github.com/shrujancheruku/issho/issues/532) by [The v2 glossary and domain model](https://github.com/shrujancheruku/issho/issues/569). The spec that uses these terms is in [`docs/spec/`](docs/spec/README.md). v1's glossary is readable at the `v1-final` tag.

Two rules hold throughout. A **type** is never used bare: it is an activity type, a message type or a piece type. A **state** belongs to its noun: a plan, a city, a crew and a calendar date can each be closed, so the noun is always said.

## Language

### People

**Person**:
Anyone with an account.
_Avoid_: user

**Member**:
A person who has been admitted to their city: any standing but waitlisted.
_Avoid_: user, participant; bare "member" for a crew member

**Account**:
A person's email, city and display language.

**Profile**:
The six answers asked at a person's first signup: name, date of birth, gender, spoken languages, residence and the code of conduct.

**Standing**:
A person's one state: *waitlisted*, *needs profile*, *needs agreement*, *ready*, *paused* or *suspended*.
_Avoid_: status; "standing plan", "standing group"

**Waitlisted**:
The standing of a person who has asked to join a city and hasn't been admitted.
_Avoid_: waiting (that is left waiting)

**Needs agreement**:
The standing of a person who has a new code of conduct or notice to agree to.
_Avoid_: needs conduct

**Ready**:
The standing of a person who can do everything.
_Avoid_: active (that is the 30-day measure)

**Paused**:
The timed standing that late cancels and no-shows lead to. Only a person is paused.
_Avoid_: paused for a plan (it is closed) or a city (that is a break)

**Suspended**:
The open-ended standing a moderator gives for conduct, lifted only by a moderator.
_Avoid_: banned, blocked

**Active**:
Signed up for a plan or went on an outing in the last 30 days.
_Avoid_: active as a standing (that is ready)

**Join**:
To make an account.
_Avoid_: register, sign up (that is a signup to a plan)

**Sign in**:
To prove who you are, with an email code.
_Avoid_: log in, register

**Join link**:
A one-use link from a member that admits whoever signs in with it, valid 14 days.
_Avoid_: invitation (that is an outing's), invite, referral

**Admission**:
The fact that an account has been let in: by a join link, or for the owner by the seed. In the Pilot a signup needs it; plans can be read without it. Issho records who brought in whom, which only a moderator sees.
_Avoid_: approval, invite

**Guest**:
A person a member adds to their own signup to a plan, by a link that shows the plan and lets them join and sign up. Admitting a guest who isn't a member uses one of the member's join links.
_Avoid_: vouch for, join offer; a guest of a crew

**Waitlist**:
The waitlisted people of a city.

**Wave**:
A batch of people admitted from the waitlist together.
_Avoid_: unlock, cohort

**Request**:
An email with a city and country, left by someone whose city isn't listed.

**Display language**:
The one language a person reads Issho in.
_Avoid_: display languages, shown language, app language, locale

**Spoken languages**:
The languages a person can comfortably hold a conversation in for a whole outing.
_Avoid_: level, learner

### Cities

**City**:
A set of adjoining areas a resident would cross for an outing, with one set of city settings, a calendar, lead sources, pieces and plans. It is *planned*, *open* or *closed*.
_Avoid_: city pack, patch, market

**Demo city**:
A city marked as demo: open, never listed or requestable, holding only scripted members, and left out of every scorecard. It is not a state.

**Area**:
One of the local government's units in a city, with its official name. A label on a piece and a filter on the plans list; a plan's area is its meeting point's area.
_Avoid_: neighbourhood, district, region

**City setting**:
One of the fixed facts that differ between cities, such as how people get around or the emergency numbers.
_Avoid_: city pack, config

**How people get around**:
The city setting that says whether people use their own transport or public transport.

**A city's languages**:
The languages a city's plans and the app are available in: its local language plus English.

**Calendar**:
A city's dated entries, each a *closed* date or a *caution* date.

**Break**:
A stop of up to 14 days on new rounds and groups in a city, in the Pilot only.
_Avoid_: pause, hold

**Pilot**:
The first stage of launch in a city: invite-only, about 60 residents.
_Avoid_: beta, test

**Soft launch**:
The second stage: people are admitted in waves from a city-wide waitlist.
_Avoid_: beta

**Public launch**:
The third stage: no waitlist.

### Plan knowledge

**Lead**:
A place or activity that a lead search, a list or a suggestion turned up and nobody has vouched for. It is *untaken*, *taken* or *dismissed*; once vouched it is a piece.
_Avoid_: candidate, prospect

**Suggestion**:
A place or activity a person puts forward for use in plans. It joins the leads list and is never a plan.
_Avoid_: suggest for what a crew does (that is propose)

**Lead search**:
The search that finds leads for a city, run by hand or on the city's cadence.
_Avoid_: engine, crawl, scout

**Lead source**:
A list or site worth searching for leads in a city.

**Leads list**:
A city's list of leads, from the lead search and from suggestions.
_Avoid_: catalogue

**Outline**:
A lead that sketches a whole plan around one activity lead. Not a plan.
_Avoid_: draft (that is a plan's state)

**Take**:
To claim a lead for a visit.

**Dismiss**:
To rule a lead out, with a reason.

**Vouch**:
An insider or the owner records that a place or activity is good for an Issho group, with its facts and the date. A visit in person is a full vouch; each independent local source is half a vouch, and a piece needs a full one.
_Avoid_: vet, verify, approve; vouch for a person (that is add a guest)

**Doubted**:
The state of a piece whose vouched facts a group or Google has contradicted, awaiting a re-check.

**Re-check**:
The act of confirming a vouch again: a visit, or an outing whose attendees all said "as described".
_Avoid_: re-confirm, re-vet

**Renew**:
The result of a re-check that passes: the vouch's date moves forward.
_Avoid_: re-confirm

### Pieces and plans

**Piece**:
A vouched thing a plan can be made from, owned by Issho and never shown to people as such. It is *vouched*, *doubted* or *retired*.
_Avoid_: building block, venue, listing

**Piece type**:
One of activity, food stop, meeting point or stretch.
_Avoid_: kind, bare "type"

**Activity**:
The piece a plan is about: something concrete the group does together. It belongs to one plan.
_Avoid_: event, experience

**Food stop**:
A piece where the group eats or drinks. An ordinary stop.
_Avoid_: meal, beverage, wind-down

**Meeting point**:
The piece where an outing's group gathers, vouched by someone who has stood there.
_Avoid_: meeting place, rendezvous

**Stretch**:
A piece that is a way on foot someone has walked and found pleasant.
_Avoid_: route, trail

**Walk**:
An activity made of one or more stretches walked continuously.
_Avoid_: a walk as a type of plan, tour

**Place**:
The physical location one or more pieces share.
_Avoid_: venue, POI

**Retired**:
Dropped for good and kept on record with the reason. Said of a piece or a plan.
_Avoid_: deleted, archived

**Plan**:
An outing people can sign up for, before it has a date or a group: stops around one activity, made by an insider or by Issho. Every plan is standing: it runs many times. It is *draft*, *in review*, *open*, *closed* or *retired*.
_Avoid_: listing, standing plan, event

**Open** (of a plan):
Taking signups in a round.

**Closed** (of a plan):
Not taking signups, always with a reason: between rounds, failed a check, a doubted piece, rated poorly or its maker suspended.
_Avoid_: paused

**Stop**:
A piece in its position in a plan, with its time and note. The word people see.

**Leg**:
The way from one stop to the next, with a mode (walk, ride or transit) and minutes stated by the maker.

**Ride**:
A leg by each person's own motorbike, car or hired ride.
_Avoid_: drive, transfer

**Activity type**:
One of Make, Move, Play, Walk, Learn or Watch: a filter on the plans list.
_Avoid_: kind, category, bare "type"

**Maker**:
The insider who made a plan. Issho's own plans have none.
_Avoid_: host, author

**Record**:
How many times a plan has run and how it was rated.
_Avoid_: record for staff acts (that is the staff log)

**Accurate**:
Everything a plan says is true on the day.

**Compelling**:
Someone who knows the city would send a friend on the plan as written.

**Rated poorly**:
The verdict of a plan's attendees that stops it running again unchanged.

**Model judge**:
A model that scores a plan against the bar before a person reviews it. It vouches for nothing.

### Matching

**Round**:
One 7-day opening of a plan, ending on its hear-by date.
_Avoid_: cycle, window

**Signup**:
One person's entry to one round of a plan, with the times they can go.
_Avoid_: registration, booking, RSVP

**Tick**:
One part of one real date a person can go, in a signup: "Saturday 10 January, evening". A signup holds a set of ticks, from 3 days ahead.
_Avoid_: slot (that is matching's own word for a tick it has read), availability

**Hear-by date**:
A round's last day, the same for everyone signed up: by then each has an invitation or a "no group this round".
_Avoid_: deadline, results day

**Matching**:
Forming groups and setting dates among the signups to plans.
_Avoid_: pool, audience, recommendation

**Seat order**:
The order in which people who fit a group are given its seats: left waiting, first-timers, mutual picks, the rest.
_Avoid_: score, weight, ranking

**Left waiting**:
Signed up to a round and given no outing from it.
_Avoid_: waitlisted (that is a standing)

### Outings

**Outing**:
One group on one date for a plan, from its invitations until feedback closes. It is *proposed*, *offered*, *set* or *over*, and ends early as *not set* or *cancelled*.
_Avoid_: meetup, event, session

**Group**:
The people of one outing. It has no life before or after the outing, and a crew is never called a group.
_Avoid_: party, table, cohort

**Proposed** (of an outing):
Formed by matching and not yet approved by the owner; nobody has been told.

**Offered** (of an outing):
Its invitations are out and fewer than 3 people have said yes.
_Avoid_: invited (that is a person's stage)

**Set** (of an outing):
At least 3 people have said yes.
_Avoid_: confirmed, locked

**Invitation**:
The offer of a seat in an outing, with its date, open 48 hours.
_Avoid_: invitation for joining Issho (that is a join link)

**Seat**:
One person's place in one outing. It holds their stage or how they left: *accepted* (a yes waiting for the outing to be set), *declined*, *unanswered*, *full*, *cancelled*, *late cancel* or *removed*.
_Avoid_: slot, spot, RSVP

**Accepted** (of a seat):
Said yes while the outing is not set yet: the yes waits for the third, which makes everyone accepted *going*. It is what a seat holds between *invited* and *going* ([Outings](docs/spec/outings.md#what-an-outing-is)).

**Stage**:
Where an outing stands for one person: *invited*, *going*, *on the day* or *after*.
_Avoid_: step, phase, status

**24-hour mark**:
24 hours before an outing's start: the end of free cancelling.

**Late cancel**:
A cancel inside the last 24 hours.

**No-show**:
Going at the start and did not attend.

**Change** (to a set outing):
A place, time or day altered for a cause outside Issho's control.
_Avoid_: reschedule, pending change

**Reminder**:
One of the three messages sent to people going before an outing.

**Attended**:
A person attended an outing when they were going, it went ahead, and their check-in or their own word says they were there, unless the group's word says otherwise.
_Avoid_: went, showed up

**Attendee**:
A person who attended an outing.

**Check-in**:
One tap ("I'm here") by a person going, with no proof of place, from 30 minutes before the start to the plan's stated end. Once checked in there is no cancel; the group sees who has checked in.
_Avoid_: arrive, confirm

**The group's word**:
What most of at least 2 others said about whether a person was there.

**Met**:
Two people have met when they both attended the same outing.
_Avoid_: know each other, matched

**Pick**:
A person's private answer, in feedback, that they'd like to see someone again. A mutual pick is two people who picked each other; neither is told.
_Avoid_: like, match

**Rather not**:
A person's private answer that they don't want to be grouped with someone again. It makes the pair kept apart.
_Avoid_: block, dislike

### Crews

**Crew**:
A set of 3 to 8 people who have all met and picked each other, which goes on outings of its own. It is *offered*, *open*, *resting* or *closed*.
_Avoid_: group, club, circle

**Crew member**:
A person in a crew.
_Avoid_: bare "member"

**Crew outing**:
An outing whose group came from a crew's yeses rather than from matching: its crew members and nobody else. Everything true of an outing holds unless Crews said otherwise.

**Propose**:
To put a plan to a crew, by a crew member or by Issho; 3 yeses make a crew outing.
_Avoid_: suggest (that is a suggestion)

**Resting**:
The state of a crew with no outing in 8 weeks.

### Chat and messages

**Room**:
One chat, belonging to an outing or a crew. It is *open*, *read-only* or *gone*.
_Avoid_: channel, thread, conversation, DM

**Read-only**:
The state of a room in which those who were in it can read and nobody can write.

**System line**:
A line in a room with no sender, written by Issho, stating a fact about that outing or crew.

**Message type**:
One of needs your answer, about your outing, about you, chat or nudge.
_Avoid_: kind, category, bare "type"

**Nudge**:
A message the person could do without; the one message type a single switch turns off.
_Avoid_: marketing

**Quiet hours**:
22:00 to 08:00 on the city's clock, when messages are held.

### Safety

**Report**:
A person telling Issho about another person's conduct, with a reason. It always makes the pair kept apart.

**Kept apart**:
Two people who are never grouped or offered a crew together again, for good: reached by a report, a "rather not" or the "Don't group me with them again" button.
_Avoid_: blocked, blacklisted

**Hold**:
No new groups or invitations for a reported person until a moderator has looked. Invisible, and not a standing.
_Avoid_: pause, break

**Warning**:
A moderator's written notice of a rule broken, kept on record.

**Flag**:
A signal that shows a person to a moderator and does nothing else.

**Tombstone**:
What a deleted account leaves against a hash of its email: suspensions, warnings, reports and kept-apart pairs.

**Notice**:
The versioned statement of what Issho collects and why, which the consent ticks refer to.
_Avoid_: notification (that is a message)

**Recipient list**:
The published list of outside services that receive personal data.

### Running Issho

**Staff role**:
A grant on an ordinary account that lets a person help run Issho: owner, moderator, insider or reader.
_Avoid_: admin (that is the tool)

**Owner**:
The staff role that runs Issho and decides everything not given to another role.

**Moderator**:
The staff role that reviews reports and flags.

**Insider**:
The staff role that makes and maintains plans and vouches for pieces in one city. No duty to attend, and no special place in a group.
_Avoid_: local, host, guide, curator, steward

**Reader**:
The staff role of a fluent person who reads the app and plans in one language before people see them.
_Avoid_: translator

**Admin**:
The web tool the owner, moderators and readers use. Not a role.

**Queue**:
The one list of by-hand steps, each an item with a due time, a role and a written default.

**Urgent**:
Said of an alert that pushes at any hour.

**Staff log**:
Who on staff did what and when, with a reason where one is required.
_Avoid_: record of staff acts, audit trail

**Scorecard**:
The weekly page of a city's numbers against their targets.
_Avoid_: dashboard, analytics

**Safety runbook**:
The fixed steps for an urgent alert.

**Fee**:
What a person pays Issho per outing, charged when they are going. The first outing is free.
_Avoid_: subscription, ticket, price (not yet decided)

### The app

**Together**, **Plans**, **You**, **Making**:
The app's tabs. Making is only for the owner and insiders.

**Needs you**:
The block at the top of Together holding everything that awaits the person's answer.

## Relationships

- A **city** has areas, city settings, a calendar, lead sources, a leads list, pieces and plans. A person's account names one city.
- A **lead** becomes a **piece** when vouched. Pieces at the same location share a **place**. A piece has one area.
- A **plan** is a sequence of **stops** joined by **legs**. Each stop is one piece; a plan has at least a meeting point and an activity. An activity belongs to one plan; other pieces can be in many.
- A plan has at most one **maker**, and a **record**.
- A plan opens in **rounds**. A person makes a **signup** to a round. **Matching** turns a round's signups into **outings**.
- An **outing** belongs to one plan and has 3 to 6 **seats**; the people in its seats are its **group**. Each seat is one person's, and carries their **stage**.
- An outing has one **room**, except a crew outing, which lives in the crew's room.
- A member can add one **guest** to a signup. The two count as a mutual pick.
- Two attendees of one outing have **met**. After an outing, each can **pick** or **rather not** each other person.
- A **crew** has 3 to 8 **crew members**, one room, and **crew outings**. A crew outing is an outing.
- A **report** or a rather not makes two people **kept apart**.
- A **staff role** is granted on an account. An insider's role is for one city.

## Dropped from v1

Beverage, meal, wind-down, meeting place, join offer, unlock, lapsed, pending change, shown language, display languages, city pack, venue, patch, pool, audience, level, and walk as a type of plan.
