# Issho

Issho brings small groups of people together for outings in their city, across the languages they speak.

## Language

**Plan**:
An outing people can sign up for, before it has a date or a group: an activity (or a walk of stops) in an area.
_Avoid_: listing, activity (an activity is what a plan is about)

**Outing**:
A plan with a date and a group of people invited to it, from their invitation until the feedback window closes. The code and the API call it an event.
_Avoid_: meetup, event (in the app's copy)

**Area**:
The place a person calls home and a plan is held in: one of the local government's units in a city pack (Tokyo's wards and cities such as Shinjuku-ku and Musashino-shi, Da Nang's wards), with its official boundary and its official name in the city's language. A city's areas are chosen this way unless there is a very good reason not to. Matching and travel times work between areas.
_Avoid_: neighbourhood, district, station, region

**Stage**:
Where an outing stands for one person: *invited* (date set: can you go?), *going*, *on the day* (until check-in closes), and *after* (how was it). An outing that is over or that they're out of is *muted* rather than a stage.
_Avoid_: step, phase, numbered stages

**Display languages**:
The one or two languages a person reads the app in: a *primary* and an optional *secondary*. Separate from the languages they speak, which matching uses.
_Avoid_: app language, locale

**Shown language**:
Which of a person's display languages leads right now; the language switch flips it to the secondary, when they have one.
_Avoid_: current language, active language

**Crew**:
A small standing group offered to people who got on well across outings; it suggests a plan each week and has its own chat.
_Avoid_: club, circle

**Room**:
One chat: an outing's group chat or a crew's chat.
_Avoid_: channel, thread, conversation

**Attended**:
Someone attended an outing when they said they could go, their group went ahead, and they checked in or said in their feedback that they went. If their feedback says they didn't go, that wins over a check-in.
_Avoid_: went, showed up, turned up (as separate rules)

**Met**:
Two people have met when they both attended the same group at an outing.
_Avoid_: know each other, been grouped together

**Suspended**:
Someone is suspended while they have a suspension that hasn't been lifted. Suspended people aren't matched, offered crews or sent pushes.
_Avoid_: banned, blocked (that is kept apart)

**Kept apart**:
Two people who are never grouped or offered a crew together again: one reported the other, or said they'd rather not be in a group with them again. Whatever a moderator decides about the report, the pair stays apart.
_Avoid_: blocked (as a separate rule), blacklisted

**Lapsed**:
A crew offer that someone never answered before it was decided.
_Avoid_: expired, ignored

**Pending change**:
An edit to a city's pack or to a language's human translations, saved in the admin SPA, that waits for the owner's approval before it takes effect. A city or language has at most one pending change at a time; later saves add to it.
_Avoid_: proposal (a plan's group review), draft, suggestion

**Soft launch**:
The first opening of Issho to real users: one city pack, with people admitted through the waitlist.
_Avoid_: beta, public launch (that comes later)

**Unlock**:
An area opening to the people waiting in it, once enough of them have joined.
_Avoid_: go live, activate
