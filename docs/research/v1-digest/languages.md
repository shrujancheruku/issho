# Languages

Which languages people speak and read, how text gets translated, and how a group's language is chosen. Sources are cited as in the [README](README.md). The code is `backend/internal/translate`, `backend/internal/uistrings` and `apps/mobile/src/i18n`.

## What v1 decided

### The principle

**Languages are symmetric.** "No default language; people speak and events run in any language." The reason given: "people who don't speak English must be fully served. A short-lived 'English-first' version was replaced before it shipped." (Log, 2026-10-20.) The Glossary's one-line description of Issho ends "across the languages they speak".

### Two kinds of language per person

| Decision | Source |
|---|---|
| **Spoken languages** are what a person can talk in at an outing: at least one fluent; a learner gives a level. **Matching uses only these**, and only the fluent ones. | Log, 2026-10-20, 2026-10-21 |
| **Display languages** are "the one or two languages a person reads the app in: a primary and an optional secondary". Kept separate because "what someone wants to read and what they can talk in at an event are different questions". | Glossary; Log, 2026-10-20 |
| **Two at a time.** Short text (names, titles) is shown side by side ("陶芸 · Cerámica"); long text uses a switch. The **shown language** is whichever leads right now. | Log, 2026-10-20; Glossary |
| A missing translation falls back to **the person's other display language first, English last**: "for someone who doesn't read English, showing only the language they do read beats a fallback they can't." | Log, 2026-10-20 |
| Any **ISO 639-1** language is accepted, so clients can't make the server translate into made-up codes. One exception: Traditional Chinese (`zh-Hant`) is a display language, not a spoken one. | Log, 2026-10-20, 2026-10-26 |
| **Layout direction** follows the primary display language (right-to-left is supported). | Log, 2026-10-26 |

### A plan's language

| Decision | Source |
|---|---|
| A plan has **a language, and optionally a second**. The **only language rule** in matching: everyone in the group speaks its primary or its secondary language fluently. | Log, 2026-10-21 |
| Someone who doesn't speak a plan's language sees it **greyed out and can't sign up**. | Log, 2026-10-21 |
| A plan's languages decide **who it suits, not how it's written**: everything is shown in the reader's own display languages. | Log, 2026-10-27; `docs/architecture.md` |
| A city pack lists the city's default languages (Da Nang–Hoi An: Vietnamese, English). | `data/citypacks/da-nang-hoi-an.json` |

### Where text comes from

| Decision | Source |
|---|---|
| **English is the only stored language; everything else is translated on demand and cached.** No translation files to keep in sync, any language works from day one, and everyone sees the same wording. | Log, 2026-10-20 |
| **Translation happens on the server**, not on the device: consistent, cached once for everyone, reviewable. | Log, 2026-10-20 |
| **Gemini is the translator**, with a note per ambiguous string and a translation guide. In a comparison it lost no placeholders (Google 35) and kept "Issho" every time. | Log, 2026-10-26 (owner) |
| **Google place names are never stored or translated**; they are fetched live in each displayed language. | Log, 2026-10-20 |
| **Area names are never machine-translated**: Spanish turned "The Domain" into "El dominio". | Log, 2026-10-21 |
| **Language names come from CLDR**, not a translator. | Log, 2026-10-21 |
| A translation that **loses a `{placeholder}` is dropped**, and English is shown. | Log, 2026-10-21 |
| **UI text: a plain English source, plus human versions of single strings.** "Plain, unambiguous English translates well but reads flat": machines translate the plain text, people who use English see the owner's livelier version, and any language can get a person's version string by string. A human version written for older English isn't shown. | Log, 2026-10-26 (owner) |
| Human versions **live in the repository** and reach people within minutes without an app release. | Log, 2026-10-26 |
| **No UI sentence is glued from pieces**, so translators can reorder and punctuate. A lint rule enforces it. | Log, 2026-11-04 |
| **The translation guide**: informal address where social apps use it, otherwise each language's usual app register; "Issho" always in Latin letters; Simplified Chinese for `zh`. "Issho talks like a friend." | Log, 2026-10-26 (owner) |
| **"Crew" is an idea, not a word to translate.** | Log, 2026-10-26 (owner) |
| The sign-in email comes in **19 languages**, generated and committed. | Log, 2026-10-21 |
| A city pack's languages are **bundled into the app**, so a fresh install doesn't flash English. | Log, 2026-10-26 |

### Decided on tickets after the log froze

| Decision | Built? | Source |
|---|---|---|
| **English and Vietnamese only for the Pilot.** Other UI languages are parked: kept in the catalogue, left out of the picker. On-demand content translation stays "so mixed groups can read each other". | No ([English and Vietnamese only](https://github.com/shrujancheruku/issho/issues/515), open) | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |
| **A Vietnamese reader reviews the core-loop strings** before the Pilot. | No | ADR 0002 (verdicts); [A Vietnamese reader reviews the core-loop strings](https://github.com/shrujancheruku/issho/issues/468) |
| **Legal texts in Vietnamese and English, Vietnamese binding**; machine-translated for the Pilot, checked by a person before the soft launch. | No | [Legal operator and Vietnam filings](https://github.com/shrujancheruku/issho/issues/362) |
| Machine translations take the **livelier English as a tone guide**. | No (Ideas) | [machine translations take the livelier English as a tone guide](https://github.com/shrujancheruku/issho/issues/350) |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| English first | Symmetric languages | Owner: people who don't speak English must be fully served | Log, 2026-10-20 |
| Hand-written Japanese, Spanish and French catalogue names | English stored, the rest translated on demand | Nothing to keep in sync | Log, 2026-10-20 |
| Claude Haiku for translation | Google Cloud Translation (Basic) | Owner: one vendor, one bill | Log, 2026-10-20 |
| Google Cloud Translation | Gemini | The comparison: placeholders, the product name, UI meaning from context | Log, 2026-10-26 |
| Outings shown in the outing's own languages | In the reader's languages | Every screen already did | Log, 2026-10-27 |
| About 20 screens each choosing the shown language | One function on each side | "Nothing should re-implement the language rule" | Log, 2026-11-02 |
| A translation editor in the admin SPA with a translator role | Parked for the Pilot | Vietnamese reviews go through pull requests | [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |
| Any language in the picker | English and Vietnamese (decided, unbuilt) | The Pilot's scope | [English and Vietnamese only](https://github.com/shrujancheruku/issho/issues/515) |

## Left open, parked, cut or only planned

- **Two-language outings and language exchange** are "not an MVP feature, only designed for". No city lists a language pair, so no two-language plan is ever suggested. Learner levels are collected and unused. The catalogue does have a "language exchange" activity (renamed "Vietnamese–English exchange" in Da Nang).
- **A mixed group's shared language in practice**: with single-language plans only, a Vietnamese speaker and an English speaker meet only if one is fluent in the other's language.
- **Chat messages aren't translated** ([chat.md](chat.md)).
- **Vietnamese has no app bundle and no human review yet.** The bundles are English, Japanese and Spanish. Whether the fonts render Vietnamese diacritics on phones is an unchecked owner task ([Check Vietnamese fonts on phones](https://github.com/shrujancheruku/issho/issues/352)).
- **A spot-check of translations per launch language** was deferred and never done.
- **Push bodies** use the primary display language only.
- **The walk composer can't match Vietnamese names** ([plans-and-catalogue.md](plans-and-catalogue.md)).
- **Moderator search can't find Vietnamese names without accents** ([safety.md](safety.md)).

## Where the code and the documents disagree

- `docs/architecture.md:127` says "Google Cloud Translation translates it now"; `docs/google-maps.md:88` says the API server calls Cloud Translation. Gemini has been the default since 2026-10-26. The Cloud Translation client is still in the repo, marked for removal.
- `docs/architecture.md` says events "are stored with English plus each of their own languages prefilled"; there is no free text on an event to prefill, since plan descriptions were never written.
- The app's code is built for any language (about 180 codes, right-to-left, per-language fonts) while the decision is English and Vietnamese only.
- The mobile README names the Tokyo pack and the Japanese and Spanish bundles.
- The Glossary's **Display languages** and **Shown language** match the code.
