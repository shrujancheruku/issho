/// <reference types="jest" />
import {
  CrewResponse,
  CrewStatus,
  EventStatus,
  GroupStatus,
  InviteResponse,
  type Crew,
  type FeedPlan,
  type MyEvent,
} from '@issho/api-client';

import { sortTogether } from '../sort';

// Only seconds are read; the cast skips protobuf's message brand.
const at = (iso: string) => ({ seconds: BigInt(Date.parse(iso) / 1000), nanos: 0 }) as MyEvent['startsAt'] & {};
const now = Date.parse('2030-01-10T12:00:00Z');

function event(id: string, over: Partial<MyEvent>): MyEvent {
  return {
    id,
    status: EventStatus.SCHEDULED,
    groupStatus: GroupStatus.INVITING,
    response: InviteResponse.INVITED,
    confirmBy: at('2030-01-11T00:00:00Z'),
    startsAt: at('2030-01-12T14:00:00Z'),
    checkInFrom: at('2030-01-12T13:30:00Z'),
    checkInUntil: at('2030-01-12T17:00:00Z'),
    ...over,
  } as MyEvent;
}
const crew = (id: string, status: CrewStatus, response = CrewResponse.OFFERED) => ({ id, status, response, members: [] }) as unknown as Crew;
const plan = (id: string, signedUp: boolean) => ({ id, signedUp }) as FeedPlan;

describe('sortTogether', () => {
  it('puts invitations first, then crew offers and feedback, and lists only open answers', () => {
    const fb = event('fb', {
      response: InviteResponse.CONFIRMED,
      status: EventStatus.DONE,
      feedbackFrom: at('2030-01-09T00:00:00Z'),
      feedbackUntil: at('2030-01-20T00:00:00Z'),
    });
    const t = sortTogether(
      [fb, event('inv', {}), event('late', { confirmBy: at('2030-01-09T00:00:00Z') })],
      [crew('offer', CrewStatus.OFFERED), crew('said-yes', CrewStatus.OFFERED, CrewResponse.ACCEPTED)],
      [],
      now,
    );
    expect(t.needsYou.map((n) => n.kind)).toEqual(['invite', 'crewOffer', 'feedback']);
    expect(t.needsYou[1]).toMatchObject({ crew: { id: 'offer' } });
  });

  it('lists active crews and outings the person is going on as groups', () => {
    const going = event('going', { response: InviteResponse.CONFIRMED, groupStatus: GroupStatus.CONFIRMED });
    const cancelled = event('gone', { response: InviteResponse.CONFIRMED, status: EventStatus.CANCELLED });
    const t = sortTogether([going, cancelled], [crew('c', CrewStatus.ACTIVE)], [], now);
    expect(t.groups.map((g) => (g.kind === 'crew' ? g.crew.id : g.event.id))).toEqual(['going', 'c']);
    expect(t.needsYou).toEqual([]);
  });

  it('lists an outing that needs an answer only under needs you', () => {
    const today = event('today', { response: InviteResponse.CONFIRMED, groupStatus: GroupStatus.CONFIRMED });
    const t = sortTogether([today], [], [], Date.parse('2030-01-12T13:45:00Z'));
    expect(t.needsYou.map((n) => n.kind)).toEqual(['checkIn']);
    expect(t.groups).toEqual([]);
  });

  it('keeps signed-up plans as waiting for a date', () => {
    expect(sortTogether([], [], [plan('a', true), plan('b', false)], now).waiting.map((p) => p.id)).toEqual(['a']);
  });
});
