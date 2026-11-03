import { InviteResponse, type Crew, type FeedPlan, type MyEvent } from '@issho/api-client';

import { crewView } from '@/features/crews/crew-view';
import { outing } from '@/features/events/outing';

/** Something waiting for the person's answer, most urgent kinds first. */
export type NeedsYou =
  | { kind: 'invite'; event: MyEvent }
  | { kind: 'checkIn'; event: MyEvent }
  | { kind: 'crewOffer'; crew: Crew }
  | { kind: 'feedback'; event: MyEvent };

/** A group chat the person is in: a crew, or an outing they're going on. */
type Group = { kind: 'crew'; crew: Crew } | { kind: 'outing'; event: MyEvent };

type Together = {
  needsYou: NeedsYou[];
  groups: Group[];
  /** Plans they signed up for that don't have a date yet. */
  waiting: FeedPlan[];
};

const order: Record<NeedsYou['kind'], number> = { invite: 0, checkIn: 1, crewOffer: 2, feedback: 3 };

/**
 * Sorts the person's events, crews and signups into Together's sections,
 * following EventService's rules (features/events/outing.ts). Pure, so the
 * rules are unit-tested.
 */
export function sortTogether(events: MyEvent[], crews: Crew[], feed: FeedPlan[], now: number): Together {
  const needsYou: NeedsYou[] = [];
  const groups: Group[] = [];
  for (const e of events) {
    const o = outing(e, now);
    const before = needsYou.length;
    if (o.needsAnswer) needsYou.push({ kind: 'invite', event: e });
    else if (o.needsCheckIn) needsYou.push({ kind: 'checkIn', event: e });
    else if (o.needsFeedback) needsYou.push({ kind: 'feedback', event: e });
    // Going (or waiting for enough people): the group chat is theirs. An
    // outing that needs an answer is listed there only, not twice. Over, out
    // or cancelled (outing() decides) is not a group.
    const listed = needsYou.length > before;
    if (!listed && e.response === InviteResponse.CONFIRMED && !o.muted && o.stage !== 'after') {
      groups.push({ kind: 'outing', event: e });
    }
  }
  for (const c of crews) {
    const v = crewView(c);
    if (v.listedAsGroup) groups.push({ kind: 'crew', crew: c });
    else if (v.needsAnswer) needsYou.push({ kind: 'crewOffer', crew: c });
  }
  needsYou.sort((a, b) => order[a.kind] - order[b.kind]);
  return { needsYou, groups, waiting: feed.filter((p) => p.signedUp) };
}
