import { CrewResponse, CrewStatus, EventStatus, InviteResponse, type Crew, type FeedPlan, type MyEvent } from '@issho/api-client';

import { eventState } from '@/features/events/state';

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
 * following EventService's rules (features/events/state.ts). Pure, so the
 * rules are unit-tested.
 */
export function sortTogether(events: MyEvent[], crews: Crew[], feed: FeedPlan[], now: number): Together {
  const needsYou: NeedsYou[] = [];
  const groups: Group[] = [];
  for (const e of events) {
    const state = eventState(e, now);
    const before = needsYou.length;
    if (e.response === InviteResponse.INVITED && state.actions.includes('confirm')) needsYou.push({ kind: 'invite', event: e });
    else if (state.actions.includes('checkIn')) needsYou.push({ kind: 'checkIn', event: e });
    else if (state.message === 'event.state.feedback') needsYou.push({ kind: 'feedback', event: e });
    // Going (or waiting for enough people): the group chat is theirs. An
    // outing that needs an answer is listed there only, not twice.
    const listed = needsYou.length > before;
    if (!listed && e.response === InviteResponse.CONFIRMED && e.status !== EventStatus.CANCELLED && e.status !== EventStatus.DONE) {
      groups.push({ kind: 'outing', event: e });
    }
  }
  for (const c of crews) {
    if (c.status === CrewStatus.ACTIVE) groups.push({ kind: 'crew', crew: c });
    else if (c.status === CrewStatus.OFFERED && c.response !== CrewResponse.ACCEPTED) needsYou.push({ kind: 'crewOffer', crew: c });
  }
  needsYou.sort((a, b) => order[a.kind] - order[b.kind]);
  return { needsYou, groups, waiting: feed.filter((p) => p.signedUp) };
}
