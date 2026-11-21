# Running Issho

From [Running Issho](https://github.com/shrujancheruku/issho/issues/557), amended by [Finding leads on a schedule](https://github.com/shrujancheruku/issho/issues/568), [Privacy and legal obligations in the product](https://github.com/shrujancheruku/issho/issues/559) and [The v2 glossary and domain model](https://github.com/shrujancheruku/issho/issues/569).

## Who runs it

- Four **staff roles**, each a grant on an ordinary account.
  - **Owner**: everything, including city settings, cities, waves and granting roles.
  - **Moderator**: reports, flags, holds, appeals and the person view. Nothing about plans.
  - **Insider**: pieces and plans in one city. No personal data.
  - **Reader**: the fluent person for one language. Sees app text and plan text only.
- "Admin" is only the tool's name.
- Only the owner grants and removes roles; removal takes effect at once.
- The admin needs a second sign-in step beyond the email code ([Technical operations](operations.md#the-second-sign-in-step)).
- Staff can never see the app as another member.
- **The staff log.** Every staff act is kept with who, when and what, including every opening of a person's view. A written reason is required for moderation decisions, group edits, waivers and switching a review back on. It is kept 24 months and read only by the owner ([Privacy](privacy.md)).
- A staff member never takes an item about themselves or about an outing they were on.

## Staff among members

- The owner goes on Pilot outings as an ordinary member, through the same matching, shown to the group with a "runs Issho" marker.
- A moderator who goes on outings carries the same marker. Readers and insiders carry none.
- In the Pilot a report about the owner goes by email to one named outside person the owner picks.
- The owner's own outings stay out of the scorecard's plan bar.

## The second moderator

- A named person the owner knows, paid, under a written agreement. Between the two moderators, Vietnamese and English are read.
- Each moderator hears appeals against the other.
- A new city needs one moderator and one reader for its local language before it opens.
- The Vietnamese reader and the second moderator can be the same person.

## The tools

- One web **admin** for the owner, moderators and readers. It talks only to the API. Insiders never get it.
- Vouching and re-checks happen in the member app on a phone, on the same screens for the owner and insiders.
- **One plan editor**, in the member app, used by the owner and insiders alike.
- The admin reviews a plan: the judge's score and reasons, the five accuracy checks, approve or send back with a note.
- The admin holds the owner-only acts: a like-for-like swap, retiring a piece, withdrawing a plan.
- In the admin a moderator sees a model translation beside the original for any report, message or feedback, marked as machine-translated.

## The queue

Every by-hand step arrives as an item in one **queue**, with a due time and the role that may take it. The admin's pages stay for looking things up; the queue is the only place work is found.

Every item has a written default at its due time, so the loop never waits on a person. Steps that add risk wait. Steps people are waiting on go ahead by rule.

| Item | Due | If nobody acts |
|---|---|---|
| Plan review, new or edited | 72 hours | Stays closed (an edited plan keeps running unedited) |
| Group approval (Pilot) | 24 hours | Invited as proposed |
| Crew approval | 7 days | Not offered; the item stays |
| Report | 24 hours | The hold lapses at 72 hours |
| Flag | 7 days | Nothing; the item stays |
| Appeal | 7 days | Nothing; the decision stands meanwhile |
| Accuracy report | 72 hours | Nothing; the item stays |
| Doubted piece | 14 days | Retired |
| Re-check due | 30 days after the 6 or 12 months | The piece becomes doubted |
| Weather decision (a set outing with rain forecast) | 6 hours before the start | The outing goes ahead; a severe-weather warning cancels by rule |
| Wave admission (soft launch) | Weekly, Monday | No wave |
| A suspended insider's plans | With the suspension | Stay closed |

Leads are not queue items ([Leads](leads.md)).

### Accuracy reports

One attendee answering "not as described" makes an item; the owner confirms (the piece becomes doubted) or dismisses. Two attendees of the same outing saying it doubts the piece at once, with no person involved.

### When the approvals fall away

Per city:

- Plan review narrows to judge-flagged plans once the judge and the owner agree on 27 of the last 30.
- Group approval ends after 20 groups in a row are approved unchanged, and belongs to the first city's Pilot ([Matching](matching.md#the-owner-approves-groups-in-the-pilot)).
- Crew approval ends after 10 crews in a row are approved unchanged, and does not return in a later city.
- The owner can switch any of them back on for a city, with a reason.

### Acting for a member

For a member who asks through support, staff can cancel their seat or waive a late cancel or no-show. Staff never sign up, answer an invitation, check in or give feedback for anyone.

## Alerts

Sent through the Issho app and by email.

- **Urgent**: a push at any hour, ignoring quiet hours. For a "felt unsafe" or harassment report, "report now" on the day, a hold 12 hours from lapsing, a weather decision, an outing within 24 hours that code finds broken, and a breach ([Privacy](privacy.md#a-breach)).
- **Everything else**: one digest email at 08:00 on the city's clock, listing the queue and, after a run of the lead search, the count of new leads.
- An insider gets only the digest: their own doubted pieces, re-checks due, plans of theirs changed or withdrawn, and new leads.

## The safety runbook

A written runbook for an urgent alert.

1. Acknowledge within 1 hour, at any hour.
2. Write to the reporter by email from the support address. Email from that address is the only way staff ever contact a member.
3. Take the reported person out of outings set in the next 72 hours while it is looked at.
4. Decide within 24 hours.
5. Record it.

Staff point people to the city's emergency numbers. They never contact police or anyone else on a member's behalf unless the member asks. The one exception: when someone may be in danger, the moderator may give the police the group's names, emails and the plan, but only after contacting the member first ([Privacy](privacy.md#giving-a-persons-data-to-someone-outside)).

## Cover

- A city can go on a **break**: no new rounds open and no groups form, outings already set go ahead, and reports still place holds. Members see "Issho is on a break in ⟨city⟩ until ⟨date⟩".
- A break is at most 14 days and exists only in the Pilot. The owner still takes urgent alerts during it.
- From the soft launch the second moderator covers and the break switch is removed.

## Support

- A "Contact Issho" action in the app and on the site sends an email to one address, answered within 24 hours in Vietnamese or English.
- No community chat group: it would undo "no direct messages" and kept apart, and expose phone numbers.

## The numbers

One weekly **scorecard** per city, holding only numbers that have a target.

| Number | Target |
|---|---|
| North star: share of people who attended who have had a second outing with someone from an earlier one | 30% |
| Got an outing: share of people who signed up and got one | 60% |
| Plan bar: outings at 4 or more on "send a friend" | 80% |
| Plan bar: outings with an accuracy problem | Under 5% |
| Went ahead: share of set outings that go ahead with 3 or more | 80% |
| Late cancels and no-shows | Read, no target yet |
| Reports looked at within 24 hours | All |
| The model judge's agreement with the owner | 27 of the last 30 |
| The lead search: of search leads vouched or dismissed in the last 3 months, the share vouched | 1 in 5, counted once 20 have been vouched or dismissed |
| By-hand load: queue items by item, how many went past due, the owner's hours (entered by hand) | No item overdue more than 1 time in 10 |

- No product analytics through the Pilot. The one thing recorded about arrival is the plan or join link a new account came by ([The site](site.md#measuring-and-reading)).
- **The wave proposal.** Each Monday in the soft launch the admin proposes a wave size: the largest that keeps the got-an-outing share at 60% over the last 4 rounds, and zero while the share or the plan bar is missed. The owner confirms, changes or skips it.

## Reading

- The owner reads English. One paid, named reader reads each other language (Vietnamese for the Pilot).
- App text is read whole, in place on every screen, before each stage. After that, only changed text before each release.
- Plans: every translation of the opening 12, then 1 in 5.

## Gates

- **Before the Pilot**: the safety runbook written; the outside person for reports about the owner named; the Vietnamese read done; the support address answering.
- **Before the soft launch**: the second moderator named, under agreement and walked through the runbook; the break switch removed; no queue item overdue more than 1 time in 10 over the last 4 weeks of the Pilot.
- **Before each wave**: nothing urgent overdue.
- **Before a new city opens**: a moderator and a reader for its local language.

## Promises that bind the stack

No outing fails because of a fault in Issho, and at most a day of data is lost. [Technical operations](operations.md) says how each is kept.
