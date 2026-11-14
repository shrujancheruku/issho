# Privacy and legal obligations in the product

What the law and the app stores ask of the product, and what v1 does about it. The operator and the filings themselves stand untouched in v2 (the map's Notes); this document is about what they require the product to do. Sources are cited as in the [README](README.md). The research is on unmerged branches: `research/vietnam-social-app-rules`, `research/vietnam-data-localization`, `research/store-paths-vietnam`, `research/vietnam-phone-verification`. It is research, not legal advice, and several points are marked unverified there.

## What v1 decided

### What the product already does

| Decision | Source |
|---|---|
| **Account deletion in the app**, which removes everything about the person. Meets both stores' rules. | Log, 2026-10-20; [Store paths to real users in Vietnam](https://github.com/shrujancheruku/issho/issues/336) |
| **An 18+ gate** by birth year. | Log, 2026-10-21 |
| **A code of conduct** accepted in onboarding, with its version stored. | Log, 2026-10-21 |
| **Only Google place ids are stored**; names, addresses and locations are fetched live. Travel times are deleted after 30 days. Nothing from Google is given to a model. | Log, 2026-10-20, 2026-10-22 |
| **OpenStreetMap boundaries are attributed** under the ODbL. | Log, 2026-10-29 |
| **The public repository never names the admin** who made a change. | Log, 2026-10-26 |
| **Clients never read the database**; the data API is off. Found while preparing hosting: anonymous clients could write to a table through it. | Log, 2026-10-20 |
| **Retention**: chat 30 days, sent notifications 30 days, push tickets a day. | Backend README, "Retention" |
| **What people see of each other is minimal**: first names, to their own group ([people.md](people.md)). | Log, 2026-10-21 |
| **No product analytics, no tracking.** Sentry traces performance and errors; its HTTP tracer was avoided because it records request headers. | Log, 2026-10-26 |
| **Error text from outside services never reaches a client.** | Log, 2026-11-03 |

### What Vietnam's law asks (decided 2026-11-07)

From [Vietnam's rules for a social app](https://github.com/shrujancheruku/issho/issues/335) and [Legal operator and Vietnam filings](https://github.com/shrujancheruku/issho/issues/362):

| Obligation | v1's decision |
|---|---|
| **The Personal Data Protection Law applies from the first user.** Consent per purpose: never pre-ticked, recorded, provable. | In-app consent per purpose, recorded with the notice's version; a person missing a required consent can't use the app until they grant it. |
| **A published privacy policy** with ways to see, correct and delete data. | On the site, in Vietnamese and English, Vietnamese binding. |
| **Attendance, feedback, picks and chat are treated as sensitive data.** | "We don't rely on the small-business waiver." |
| **Two impact assessments** (processing, and cross-border transfer) filed within 60 days of the first Pilot invitee, updated every 6 months. | The owner files both. |
| **Breach reports within 72 hours.** | A runbook. |
| **A data-protection contact.** | The owner; `privacy@joinissho.com`. |
| **Hosting abroad is a cross-border transfer** wherever it is. Storing data in Vietnam is not triggered: it applies only after repeated unremedied Ministry requests. | Production in Singapore. |
| **Requests and takedowns from the Ministry within 24 hours.** | In the same runbook. |
| **A 12-month log of each account's sign-ins and sign-outs**, with time and IP address. Applies to foreign providers of any size. | A Pilot gate. |
| **Verifying accounts by Vietnamese phone number or ID number.** | A soft-launch gate. The Pilot runs on email codes "as an accepted risk". |
| **Social networks may not require photos of ID documents.** | Conflicts with the once-planned Stripe Identity. |
| **Tracking needs consent or an opt-out.** | Product analytics deferred. |
| Run by a **company in Vietnam**, a licence, a `.vn` domain, servers in Vietnam and phone verification would apply from day one. | Ruled out. |

### What the stores ask

From [Store paths to real users in Vietnam](https://github.com/shrujancheruku/issho/issues/336) and [Distribution for each platform](https://github.com/shrujancheruku/issho/issues/343):

| Requirement | v1's decision |
|---|---|
| **User blocking** | A minimal block: a kept-apart pair plus a button; no unblocking ([safety.md](safety.md)). |
| **Terms accepted before posting** | Terms on the site; the code of conduct in onboarding. |
| **A support contact in the app** | Named as a gate; nothing more recorded. |
| **A privacy policy** | On the site. |
| **A web page for requesting deletion** (Play) | On the site. |
| **A way for reviewers to sign in** | A password for reviewer accounts, behind a long press. |
| **Filtering objectionable content before posting, or an argument for moderating after** (Apple) | The argument: reporting, blocking, the owner told of every report, action within 24 hours. |
| **Age rating** | Each store's highest. |

### Data kept after deletion

| Decision | Source |
|---|---|
| **A report is kept if the reporter deletes their account.** | Log, 2026-10-23 |
| **A suspended person who deletes leaves a tombstone**: a keyed hash of the email plus the suspension and its reasons. "This keeps a small amount of personal data after the person has asked for erasure", judged justified for safety; the privacy policy discloses it; a lawyer checks it before the public launch. | ADR 0003 |
| **Moderators may read a reported chat**, and the privacy notice says so. | [moderators can read the chat behind a report](https://github.com/shrujancheruku/issho/issues/485) |
| **A person can export their own data**, "never what others said about them". | [in-app consent per purpose](https://github.com/shrujancheruku/issho/issues/479) |

### Accepted risks, as recorded

Email codes instead of phone verification during the Pilot (a fine of 30–50 million VND for an individual); the owner running the Pilot from Vietnam on a tourist visa; "provider from abroad" being weaker while the owner lives in Vietnam; personal liability until the LLC exists; no lawyer before the public launch (ADR 0004; [Legal operator and Vietnam filings](https://github.com/shrujancheruku/issho/issues/362)).

## Reversed or changed along the way

| From | To | Why | Source |
|---|---|---|---|
| The Supabase data API on | Off | Anonymous writes were possible | Log, 2026-10-20 |
| A 30-day travel-time cache judged within Google's terms | Flagged for a re-check; then found to break current terms by the v2 research | The terms state no window for durations | `docs/architecture.md`; [Terms and law on scraping and reuse](https://github.com/shrujancheruku/issho/issues/537) |
| The small-business waiver for the processing assessment | File both | Sensitive data loses the waiver; a sole owner isn't named in the relief | [Vietnam's rules for a social app](https://github.com/shrujancheruku/issho/issues/335), [Legal operator and Vietnam filings](https://github.com/shrujancheruku/issho/issues/362) |
| Phone verification "only above 100,000 visits a month" (Decree 147) | A duty at any size (Decree 333), made a soft-launch gate | Later research | [Legal operator and Vietnam filings](https://github.com/shrujancheruku/issho/issues/362) |
| Deletion erases everything | Everything except a suspended person's tombstone | Safety | ADR 0003 |
| Stripe Identity | In doubt | Vietnam's ID-photo rule | [Vietnam's rules for a social app](https://github.com/shrujancheruku/issho/issues/335) |

## Left open, parked, cut or only planned

**None of the legal floor is built.** As of the snapshot there is no privacy policy, no terms, no consent record, no data export, no sign-in log, no blocking, no deletion page, no breach runbook, no tombstone and no invite check. Each has a decided design on an open ticket.

Also open:

- **The purposes of consent** and the notice's text: "come from Legal operator and Vietnam filings", not yet written.
- **What withdrawing a required consent does** beyond locking the app.
- **A support contact in the app.**
- **Questions listed for a lawyer** before the public launch: whether an individual counts as an "enterprise"; email codes at the Pilot's size; the filing route for foreigners; the tombstone as an exception to erasure; the tourist visa.
- **Unverified in the research**: the filing forms, portal and fees; how "visits" are counted for an app; whether SMS codes alone satisfy phone verification; whether Twilio serves an individual for Vietnam.
- **Foreigners and phone verification**: visitors would need a local SIM registered with a passport; "without a SIM, they can browse only."
- **Laws of any country other than Vietnam** (and the stores' rules): nothing recorded. The owner is a US resident; no US or other rules were researched.
- **Children**: the 18+ gate is a stated birth year only.

## Where the code and the documents disagree

- **ADR 0003 describes a tombstone that doesn't exist.** Deleting an account cascades through suspensions and reports against the person.
- **The site says** "We only use your email to tell you when Issho opens near you" above a form that collects nothing; there is no policy behind the sentence.
- **`docs/architecture.md`** says travel times are deleted after 30 days in line with Google's terms; the v2 research disagrees about the terms.
- **The Glossary's Pilot** is invite-only; sign-up is open (`supabase/config.toml`).
- **The conduct version**: the log says raising it makes everyone accept again; no screen exists, and a test blocks the raise.
