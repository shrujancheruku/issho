# Privacy and legal obligations in the product

From [Privacy and legal obligations in the product](https://github.com/shrujancheruku/issho/issues/559), amended by [The v2 glossary and domain model](https://github.com/shrujancheruku/issho/issues/569), with facts from [What the v2 spec asks of the stack](https://github.com/shrujancheruku/issho/issues/571). The legal parts are research, not legal advice.

## The floor

- Vietnam's Personal Data Protection Law and social-network decrees, plus both stores' rules. Members are residents of one Vietnamese city.
- Another country's law is researched when a city there is planned.
- A lawyer checks before the public launch ([ADR 0004](../adr/0004-the-owner-operates-issho-until-the-public-launch.md)), not before.

## Consent

- Asked at two moments, each for what is being collected then.
  - **At sign-in:** keeping the email and city to hold a place and send the three waitlist emails; storing it outside Vietnam.
  - **At the first signup, with the profile:** using the profile's answers to form groups; showing a first name to the group; keeping attendance, feedback, picks and chat, which are treated as sensitive.
- Every tick starts unticked and is recorded with the notice's version.
- Every tick is required for its step. There are no optional purposes, because there is no analytics, marketing or tracking.
- **Withdrawing consent is deleting the account.** The account screen has one action, "Delete my account", and the privacy policy says so.

## When the notice changes

- **A new purpose or a new type of recipient** (the fee and a payment processor, the phone number at the soft launch): the person ticks again and can't sign up or answer until they do. Outings already set go ahead.
- **Any other edit:** an email 14 days before it takes effect, no tick.
- The standing **needs agreement** covers a new code of conduct and a new notice.

## Phone verification and the sign-in log

- From the soft launch a verified +84 number is a seventh profile answer, shown to nobody and never used to sign in or to contact anyone.
- The Pilot runs on email codes as an accepted risk.
- It costs about US$0.34 per person through Twilio Verify. Delivery to Vietnamese carriers is tested with real SIMs before the soft launch.
- Each account's sign-ins and sign-outs are logged with time and IP address from the Pilot's first account and deleted at 12 months. Nobody can read the log in the admin; it is produced only for a lawful request. It is the only place Issho keeps an IP address.

## Age

- A stated date of birth is enough until identity verification is decided for the public launch.
- Issho checks the date and keeps only "said they were 18 or over, on this day".
- "Seems under 18" is a report reason, and the moderator suspends on it.

## Deleting an account

- At once, from the app or the site's page (sign in, then delete). No grace period.
- A set outing loses the person as a cancel with no points.
- Their messages stay until the room's own 30 days run out, shown as from "someone who left".
- Their feedback on plans stays without their identity. Their picks, signups and profile go.
- Their reports about others stay, without their name. Plans and vouches they made stay Issho's.

## The tombstone

[Safety](safety.md#deleting-an-account) says what it holds. Each part has a lifetime:

| Part | Kept |
|---|---|
| A warning, or a suspension that was lifted | 12 months from the act |
| A suspension still in force | While it is in force |
| A report about them | 12 months after it was closed |
| A kept-apart pair | While the other person has an account |

The tombstone disappears when it holds nothing. The privacy policy states all of this.

## Export and correction

- A person downloads one file of what they gave Issho: account, profile, signups, outings, the feedback and picks they gave, crews, the messages they sent, the reports they made. Never what others said about them. Available while suspended.
- Correction is editing the profile in the app.
- Anything else is a "Contact Issho" request, answered within 24 hours and completed within 7 days.

## What is kept and for how long

| Record | Kept |
|---|---|
| Chat | 30 days after the room goes read-only ([Chat](chat.md#how-long-messages-are-kept)) |
| Signups, outings, attendance, feedback, picks | While the account exists |
| Closed reports | 12 months |
| Conduct points | 90 days |
| The staff log | 24 months, read only by the owner |
| Sent notifications | 30 days |
| The sign-in log | 12 months |
| A waitlisted account never given a place, or any account with no sign-in for 24 months | Deleted after one warning email 30 days before |

## Waitlist emails

- The three emails are the service the person asked for, not marketing.
- Each carries one "leave the waitlist" link, which deletes the account after one confirmation tap. There is no separate unsubscribe.

## Who receives personal data

- A short published **recipient list**, and nothing goes to anyone off it: hosting and database, email and push delivery, error tracking (no request bodies or headers), the phone-code sender, and one machine-translation service for the moderator's view of reports, messages and feedback, under terms that forbid training on it.
- No personal data goes to a model for any other reason. The model judge, plan translation and the lead search see no personal data.
- Fonts are served from `joinissho.com`, not from Google.
- Every candidate service holding personal data is outside Vietnam, which is a cross-border transfer: a transfer impact assessment is owed within 60 days of the first transfer ([What the v2 spec asks of the stack](https://github.com/shrujancheruku/issho/issues/571)).

## Giving a person's data to someone outside

- **A lawful request from an authority:** only the owner answers, it is written in the staff log, and the person is told unless the law forbids it.
- **Someone may be in danger:** the moderator may give the police the group's names, emails and the plan without waiting for a request. (See the unresolved note in [Running Issho](running-issho.md#the-safety-runbook).)
- **Another member asks:** never.

## A breach

- The authority is told within 72 hours.
- The people affected get an email within 72 hours of the owner knowing: what was exposed and what to do. Every session is signed out when sign-in data is involved.
- It is in the same written runbook as safety and is an urgent alert at any hour.

## What people write

- No filter before posting. The argument to the stores: reporting, kept apart, the moderator told of every report, action within 24 hours.
- Suggestions and plans are licensed to Issho for good.
- A takedown request from anyone (a venue, a rights holder, the Ministry) goes to "Contact Issho" and is answered within 24 hours.

## The privacy policy and the terms

- Both in every city language, the local-language version binding, read by the paid reader.
- Drafted from this document, read by the owner, checked by the lawyer before the public launch.
- Each carries a version and a date; old versions stay readable on the site.
- One contact, the support address. `privacy@joinissho.com` delivers to it.
- At sign-in both are linked beside the consent ticks, and continuing is agreeing. The code of conduct stays its own answer in the profile.

The terms say:

- For residents of an open city aged 18 or over; one account per person.
- The code of conduct is part of the terms, and breaking it can end the account.
- Issho plans outings and forms groups. It doesn't run the venues or activities, and it doesn't vet members beyond what [Safety](safety.md) describes. Each person goes at their own risk and pays their own costs.
- What people write is licensed to Issho.
- Vietnamese law governs. The fee's terms are added when the fee turns on, as a notice change that needs a new tick.

## The stores

Each store's highest age rating.
