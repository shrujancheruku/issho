# People

Accounts, sign-in, the profile and what people see of each other. Safety is in [safety.md](safety.md). Sources are cited as in the [README](README.md). The code is `backend/internal/identity`, `backend/internal/profile` and `apps/mobile/src/features/profile`.

## What v1 decided

### Sign-in and the account

| Decision | Source |
|---|---|
| **Email code is the only way to sign in**: a 6-digit code, valid 10 minutes. Not a magic link, because links can't reliably open a native app. | Log, 2026-10-20 |
| Apple and Google sign-in were **deferred**: not needed to validate the product, and any social login on iOS makes Sign in with Apple mandatory. | Log, 2026-10-20 |
| **A first sign-in creates the account.** Anyone with an email can join; nothing checks an invitation. | `supabase/config.toml` (`enable_signup = true`); Backend README, `GetMe` |
| **Adults only.** Only the birth year is asked: born in or before this year minus 18, "so someone turning 18 later this year passes". Identity verification was meant to be "the real check". | Log, 2026-10-21 |
| The sign-in email is written in the person's language (19 languages, English for the rest). | Log, 2026-10-21 |
| **Deleting the account** is in the app and removes everything about the person. The auth user is deleted first, so a failure loses nothing and can be retried. | Log, 2026-10-20 |
| A suspended person can still sign out and delete their account. | Log, 2026-10-24 |
| Sessions are kept in the phone's secure store, never plain storage. | Log, 2026-10-20 |

### The profile

Onboarding is one pass, saved whole at the end: **languages, about you, area, availability, activities, kind of outing, code of conduct** (`apps/mobile/README.md`). The profile is always written whole (Log, 2026-10-21). What it holds (`proto/issho/v1/profile.proto`):

| Field | Rule | Used for |
|---|---|---|
| **First name or nickname** | 1–40 characters | Shown only to confirmed members of the person's group, and to crew members |
| **Birth year** | Must make them 18 this year | The age gate; shown to admins reviewing a group |
| **Gender** | Woman, man, nonbinary, or prefer not to say; an answer is required | The balance preference in matching; the same-gender-only rule |
| **Same gender only** | Needs a stated gender | A hard rule in matching |
| **Home area** | One area of a city | Travel time to plans; a crew's possible basis |
| **Longest trip** | 5–180 minutes | Which areas' pools they join |
| **Languages they speak** | At least one fluent; a learner gives a level (beginner, intermediate, advanced) | Matching uses fluent ones only |
| **Display languages** | A primary and an optional secondary | What the app is read in ([languages.md](languages.md)) |
| **Usual availability** | At least one slot: a weekday × morning, afternoon or evening | The default for every signup |
| **Activities** | **At least three**, each "open to try", "like" or "love" | The feed's order and which plans are suggested |
| **Group size** | Around 4, 6 or 8 | The group's target size |
| **Energy** | Calm, moderate, active | Collected; matching doesn't read it ([matching.md](matching.md)) |
| **Budget** | Free to expensive, four levels | Marks down activities that cost more |
| **Code-of-conduct version** | Must be the current one | See below |

Other decisions:

| Decision | Source |
|---|---|
| Onboarding **starts from the device's first two supported languages**; only the first is assumed to be spoken. | Log, 2026-10-21 |
| Interest levels are **colours shared by the legend and the tiles**, tapped to cycle. An animated example was tried and removed. | Log, 2026-10-21 |
| **Settings saves only when every section is complete.** Changing city clears the activities, because they belong to the city. | Log, 2026-11-02 |
| There is **no photo, no bio, and no public profile**. Nobody can look anyone up. | `proto/issho/v1/profile.proto`; Log, 2026-10-21 ("no profile browsing") |

### What people see of each other

- **Before confirming an outing:** counts only ("7 interested", how many confirmed).
- **After confirming:** the first names of everyone confirmed in their group.
- **In a crew offer or a crew:** members' first names and who accepted.
- **In feedback:** the first names of the others in their group.
- **Never:** age, gender, languages, home area, interests, or who picked whom.

(Backend README, `GetEvent`, `ListMyCrews`, `GetFeedback`; Log, 2026-10-21 and 2026-10-23.)

### The code of conduct

Six rules, accepted at the end of onboarding (`apps/mobile/src/i18n/en.json`, `conduct.*`): be kind and respectful; come with **platonic intentions** and respect boundaries; respect privacy (no sharing photos or details without asking); turn up when you've confirmed and cancel early if you can't; meet in the agreed public place and drink responsibly; report anyone who makes you or others uncomfortable.

| Decision | Source |
|---|---|
| The conduct asks for platonic intentions and respect for boundaries; **it doesn't ban romantic interest**. "Feelings can't be prohibited, and the rule should be one people can keep." Unwanted pursuit is reportable. | Log, 2026-10-21 (owner) |
| The accepted **version is stored with the profile**; raising the version makes everyone accept again. | Log, 2026-10-21 |

### Roles

| Role | How it's given | Source |
|---|---|---|
| **Admin** | A flag on the person's Supabase account that only the service role can set; carried in the access token; set with a command-line tool. A revoked admin keeps access for up to an hour. | Log, 2026-10-23 |
| **Translator** | A list of languages on the account; sees only the translation editor. | Log, 2026-10-26 |

### Decided on tickets after the log froze

| Decision | Built? | Source |
|---|---|---|
| **Invite-only for the Pilot.** An email allowlist checked by the API. Each invited person may invite up to 5 emails from the You tab. Someone not invited sees an invite-only screen. | No | [only invited people can join the Pilot](https://github.com/shrujancheruku/issho/issues/477) |
| **One Standing per person** replacing the scattered checks: needs profile, active, suspended, not invited, needs consent (waiting and needs-conduct reserved). | No | [one Standing in identity](https://github.com/shrujancheruku/issho/issues/402) |
| **Re-accepting a new code of conduct** on a blocking screen. Until they accept, a person can't take on anything new but keeps what they already confirmed. | No (Ideas) | [re-accept a new code of conduct](https://github.com/shrujancheruku/issho/issues/358) |
| **A person can export their data** as a JSON document: what is stored about them, "never what others said about them". | No | [in-app consent per purpose](https://github.com/shrujancheruku/issho/issues/479) |
| **Store reviewers sign in with a password**, offered after a long press on the logo. | No | [a way for app store reviewers to sign in](https://github.com/shrujancheruku/issho/issues/486) |
| **Verification by Vietnamese phone number** is a gate for the soft launch, not the Pilot. Recommended: keep the email code for sign-in and add a verified +84 number as a second attribute (about US$0.34 each by SMS). Visitors without a local SIM could browse only. | No | [Verifying accounts by Vietnamese phone number](https://github.com/shrujancheruku/issho/issues/505) |

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| Magic links | Email codes | Links can't open a native app reliably | Log, 2026-10-20 |
| Clerk + Neon | Supabase Auth and Postgres | People live in our own database; one vendor | Log, 2026-10-20 |
| Free-text interests tagged by a model (Claude, then Gemini), confirmed by the person | Parked for the Pilot: people pick activities directly | Out of the Pilot's scope | Log, 2026-10-21; [The Pilot's scope](https://github.com/shrujancheruku/issho/issues/417) |
| Per-device memory of the push token | Per account, and every sign-out unregisters | The next account on a phone got no pushes while the old one kept getting them | Log, 2026-11-02 |
| Stripe Identity for the release | Deferred to the public launch, then questioned: Vietnam's law bars social networks from requiring photos of ID documents | Paid account; law | Log, 2026-10-26; [Vietnam's rules for a social app](https://github.com/shrujancheruku/issho/issues/335) |
| Open sign-up | Invite-only for the Pilot (decided, unbuilt) | The Pilot is invite-only and nothing enforced it | [only invited people can join the Pilot](https://github.com/shrujancheruku/issho/issues/477) |

## Left open, parked, cut or only planned

- **Identity verification** of any kind: none exists. Email is the only proof of anything.
- **The same person signing up twice** with another email is not prevented; invite-only was named as the control.
- **Interest tagging** ("tell us what you like" in free text): built, parked.
- **Moving city.** A profile has one home area in one city. Changing city clears the activities and is otherwise unhandled.
- **Travellers and short stays** were never decided: an open question from the recruiting research was "whether to accept short-stay nomads or prioritise residents".
- **Accessibility and other needs** are not asked.
- **What onboarding should ask** was never challenged after the first build; only the activity list was cut down after a device test.

## Where the code and the documents disagree

- **Energy** is asked in onboarding and stored, and matching doesn't read it.
- **A person's standing is decided in five places** by three mechanisms; `GetMe` reads suspension from the database while the gate reads a cache up to 15 seconds old, so the two can disagree briefly.
- **Re-acceptance.** The log says raising the conduct version "makes everyone accept again". No screen asks; a test (`TestConductVersionIsOne`) stops the version being raised.
- `ListLanguages` returns every ISO 639-1 language (Backend README), while the Pilot decision is English and Vietnamese only.
- Comments in `profile/validate.go` cite the retired milestone "(M9)" for identity verification.
