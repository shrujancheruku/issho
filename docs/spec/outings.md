# Outings

From [The outing](https://github.com/shrujancheruku/issho/issues/550), amended by [Notifications](https://github.com/shrujancheruku/issho/issues/554) and [The v2 glossary and domain model](https://github.com/shrujancheruku/issho/issues/569), with the change rules from [What varies by city](https://github.com/shrujancheruku/issho/issues/545).

## What an outing is

An **outing** is one group on one date for a plan, from its invitations until feedback closes. A round of a plan can make several outings. The **group** is the people of one outing and has no life before or after it.

An outing is **proposed** (formed, awaiting the owner's approval in the Pilot), **offered** (its invitations are out and fewer than 3 have said yes), **set** (at least 3 have said yes) or **over**. It ends early as **not set** or **cancelled**.

A **seat** is one person's place in one outing. It holds their stage, or how they left: declined, unanswered, full, cancelled, late cancel or removed. The app shows an outing that is over, or that the person is out of, as muted.

## The stages

| Stage | What it means | What the person can do |
| --- | --- | --- |
| **Invited** | A date is set: can you go? | Say yes or no for 48 hours. |
| **Going** | They said yes and the outing is set. | Cancel; see the others' first names; see the outing. |
| **On the day** | From 02:00 on the city's clock until the plan's end. | Check in; see who else has. |
| **After** | From the plan's end, for 7 days. | Give feedback. |

## Invited

- An invitation is open for a **fixed 48 hours**, counted from delivery, so quiet hours never eat into it ([Notifications](notifications.md)).
- The outing is set as soon as 3 say yes. With fewer than 3 yeses at 48 hours it is not set, and those who said yes count as left waiting.
- A no can be changed to a yes inside the 48 hours while a seat is free.
- The person sees how many said yes, never who, and the languages the group shares.

## Seats stay open until the 24-hour mark

- An outing's capacity is the plan's 6 seats, not the size it was set at. Any empty seat, whether never filled or given up by a cancel, can be taken until the 24-hour mark.
- **Issho offers the seat; nobody picks a date.** The offer goes, in seat order, to people signed up to that plan who ticked that time and fit the group (its languages, kept-apart pairs, the "only women and non-binary people" setting). That includes someone who signs up to the plan's next round after the outing was set: they are invited at once.
- An offer is open for 24 hours from delivery or until the 24-hour mark, whichever comes first. Nobody is added inside the last 24 hours.
- There is no public "join this date" button: it would publish the date and let a person choose their group.
- People going see the first names update, and each person added is a system line in the outing's room.

## Going

- **The meeting point is public on the plan.** Only the date, the time and the group are private.
- The **24-hour mark** is the end of free cancelling, the second accuracy check, and the last moment a seat can be filled.
- **Cancelling** stays possible until the start. Inside the last 24 hours it is a **late cancel**; after the start, not coming is a **no-show**. Once checked in there is no cancel. The cancel button states the cost before the tap.
- **What the person sees**: the date and time on the city's clock (with the device's time beside it when different), the languages the group shares, the stops with their legs, the price, what to bring, the first names of the others going, and any change at the top ([The app's shape](app-shape.md)). It works offline on the day from its last copy; check-in needs a connection.
- **Sharing.** Only the plan can be shared, never an outing. There are no photos of an outing.

## On the day

- **Check-in** is one tap ("I'm here") with no proof of place, open from 30 minutes before the start to the plan's stated end.
- The group sees **who has checked in**.
- The screen shows the city's emergency numbers and a "report now" action ([Safety](safety.md#emergencies)).

## What the group sees of each other

| Stage | Seen |
| --- | --- |
| Invited | A count of yeses, and the languages the group shares |
| Going | First names of the others going |
| On the day | Who has checked in |
| After | The same first names, for feedback and reports |

Beside a first name the viewer may also see a private "you met at ⟨plan⟩, ⟨month⟩" marker ([People](people.md)) and a "runs Issho" marker ([Running Issho](running-issho.md)).

## When an outing doesn't happen as set

- **Too few.** A set outing goes ahead with 2. It is cancelled the moment only 1 person is going, at any point before the start; that person is told at once and gets the priority of people left waiting. If the others simply don't turn up, the one who came is told "we're sorry", gets the same priority, and the others are no-shows.
- **The forecast fails.** When the check at the 24-hour mark finds rain forecast for the hours of a plan that needs dry weather, the outing is cancelled unless the owner or an insider moves the day within 6 hours of the check. A moved day follows the change rules below. There is no wet-weather substitute: a different activity is a cancellation. A cancellation gives left-waiting priority and costs nobody reliability.

  > **Unresolved (5).** [Routes worth walking](https://github.com/shrujancheruku/issho/issues/546) said "no forecast feed for the Pilot", with weather handled by a person changing the outing. This ticket, later the same day, and [Running Issho](running-issho.md#the-queue) (the weather decision) assume a forecast is looked up. See the [list of conflicts](README.md#conflicts-for-the-owner).
- **Removed before the start.** A person suspended before the start is taken out as if they had cancelled, and so is the reported or not-wanted person when two people going become kept apart. The group is told only that it is one smaller. It costs the others nothing.

## Changing a set outing

A workshop is rescheduled, a restaurant opens late. The principle: nobody arrives at a place or time that was true yesterday.

- **What may change**, and only for a cause outside Issho's control: a place (the like-for-like swap in [Pieces](pieces.md#losing-standing)), a time within the same day, or the day. Anything larger, such as a different activity, is a cancellation. A change that makes a plan better, not truer, waits for the next round.
- **Who:** the owner or any insider of the city, with a reason the group sees, and no review beforehand. The plan's maker is told. Every change is recorded on the outing; a plan changed on two outings running has the piece that caused it sent for a re-check.
- **The people going:**
  - A swapped place, or a time moved by an hour or less: they stay going, and leaving is free even inside the last 24 hours.
  - A time moved by more than an hour, or a new day: everyone is asked **"still going?"** and nobody is counted in until they say yes. They have 24 hours from the notice, shortened so it always ends at least 3 hours before the earlier of the old and new start. The outing is set again as soon as 3 say yes; fewer than 3 at the deadline cancels it. No answer frees the seat without penalty.
  - No change ever costs a person reliability.
- **Being told:** a push and a system line in the outing's room, both naming what changed, from what to what, and why. The outing shows the change at the top with the old value struck through until it ends. Inside the last 24 hours the change also goes by email. Inside the last 3 hours, a new day or a move of more than an hour is not allowed and becomes a cancellation.

## Attended, met and no-show

- A person **attended** if they checked in, said so in feedback, or at least 2 others ticked them in "who else was there?".
- Their own "I didn't go" wins over all of that.
- **The group's word:** when at least 2 others answered and most of them unticked the person, that wins over the person's own claim and check-in. One untick alone never does. In an outing of 2 there is no group's word and each person's claim stands.
- Two people have **met** when both attended the same outing.
- A **no-show** is provisional from the plan's end and final when feedback closes at 7 days. The person is told when it is recorded and can answer "I was there" in feedback. It affects the seat order only once final, unless the person said "I didn't go" themselves. Nobody can see who unticked them.

## Feedback

Open from the plan's stated end for 7 days, to people who were going at the start. Answers can be changed until it closes.

1. **Did you go?** A "no" asks why and ends there.
2. **Who else was there?** Everyone who was going, ticked by default.
3. **Was it as described?** A "no" asks which stop, with an optional note.
4. **Would you send a friend on this plan?** 1 to 5.
5. **Who would you meet again?** Each answer is a pick.
6. **Anyone you'd rather not be grouped with?** Folded away. Each answer is a "rather not", and makes the pair kept apart.

Questions 1 to 4 are required; 5 and 6 are optional. After a crew outing with a guest, each crew member who was there is also asked the guest question ([Crews](crews.md#how-a-guest-becomes-a-crew-member)).

- **No group rating.** The measure of "a few people they can talk to" is the share of attendees who picked at least one person to meet again, per outing and overall. A "rather not" is counted separately. The Pilot sets the target.
- **Feedback nobody gives** costs the person nothing. Fewer than 3 "as described?" answers renews no vouch. A "send a friend" median needs at least 2 answers before it can make a plan rated poorly.
- Picks are private and neither person is ever told of a mutual pick.
