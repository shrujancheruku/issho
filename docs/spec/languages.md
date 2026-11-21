# Languages

From [Languages](https://github.com/shrujancheruku/issho/issues/556), with the reading rules from [Running Issho](https://github.com/shrujancheruku/issho/issues/557).

## The languages the app is shown in

- The app's languages are a **short fixed list: the languages of the open cities**. For Da Nang–Hoi An that is Vietnamese and English, at every stage. A language is added only when a city that needs it opens.
- A device set to a language not on the list gets English.
- A person has **one display language**. There is no second display language, side-by-side text or per-screen switch. Push and email use it.
- A display language need not be one the person speaks.

## The languages people speak

- The profile asks one question: **"Which languages can you comfortably hold a conversation in for a whole outing?"** Self-declared, at least one.
- No levels and no learner languages.
- A person can list **any language** (names from CLDR), including one the app isn't shown in.

## A group's language

- A group can form in **any language all of its people share**. Four Russian speakers in Da Nang can be a Russian-speaking group, each reading the plan in English or Vietnamese.
- When they share more than one language, **the invitation names every shared language** ("this group can speak Vietnamese and English"). Issho doesn't choose one.
- Matching requires one shared language and prefers no language over another. It never builds a two-language group. A language exchange can still be the activity in a plan.

## The app's own text

- The owner writes **one English**.
- Each other language is **drafted by a model** from the English and the translation guide, and kept as fixed text. Nothing in the app's own text is translated at run time.
- The translation guide: Issho talks like a friend; informal address where social apps use it; "Issho" always in Latin letters; "crew" is each language's everyday word for a group of friends who keep meeting.
- **A reader reads the whole language before the Pilot and again before each later stage**, in place on every screen, the site and its pages included. After that, only changed text is read, before each release.
- Fixed wording Issho sends (push, email, system lines in a room) follows the same rule. Each person gets it in their own display language, so two people in one room can see the same system line in different languages.

## Plan text

- The maker writes a plan in **any one of the city's languages**.
- Everything that must be true on the day (times, price, address, meeting point, each leg's mode and minutes) is a **field, not prose**, so a translation can't change it.
- When the plan is approved, a model translates the prose into the city's other languages and **the result is stored** (it is Issho's own content). An edit between rounds is translated again.
- A reader checks every translation of a city's **opening 12 plans**, then 1 in 5. The model judge checks that each translation says what the original says, and attendees' "as described?" catches the rest.
- A person sees the plan in their display language with a note, **"translated from Vietnamese"**, and can switch to the original.
- A reviewer who can't read a plan's language **may review it through the model's translation**; the model judge scores the original. Insiders are never required to write in English.

## Names and formats

- A place, a piece and an area keep their real local name and are never translated.
- City names use each language's usual name.
- Language names come from CLDR.
- Dates, numbers and money follow the display language's conventions, on the city's clock and in the city's currency.
- A crew's name is shown as its crew members wrote it.

## Text people write

- Chat is shown as written, with an on-device translate toggle ([Chat](chat.md#translation)).
- Suggestions, feedback comments, reports and appeals are written in **any language**. The owner or a moderator reads them beside a model translation, marked as machine-translated. None of it is shown to other people, so none of it is translated for them.
- **The messages attached to a report may be machine-translated for the moderator.** This is the one case where chat text goes to a translation service, and the privacy policy says so.

## Before an account

- A visitor with no account gets their browser's or device's language if it is on the list, English otherwise, with a visible switch.
- Waitlist and sign-in emails come in the language the person was reading when they joined.
- That language becomes the account's display language, changed in settings and never asked.

## A city's languages

- A city's languages are **the language most of its residents read, plus English always**. English is the source the owner writes and the fallback for every unsupported device; the local language is there because people who don't read English must be fully served.
- The language items on a city's opening list:
  - the app's text read by a reader in each of the city's languages;
  - all 12 opening plans in each language, checked by a person;
  - the emails and legal texts in each.

## Not built until a city needs it

Any other display language, right-to-left layout and per-language fonts. There is no translation editor in the admin.
