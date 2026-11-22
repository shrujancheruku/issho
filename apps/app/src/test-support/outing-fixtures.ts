import { OutingState, SeatState, type Outing } from '@issho/api-client';

const at = (iso: string) => ({ $typeName: 'google.protobuf.Timestamp' as const, seconds: BigInt(Math.floor(Date.parse(iso) / 1000)), nanos: 0 });

/**
 * An outing as ListMyOutings returns it (proto/issho/v1/outing.proto), for
 * screen tests: invited to Lantern making on Wed 25 November at 09:00 in Da
 * Nang, the invitation open until 23 Nov 16:00 UTC (the tests' "now" is 22 Nov
 * 10:00 UTC), two have said yes, the group shares English and Vietnamese.
 */
export function anOuting(over: Partial<Outing> = {}): Outing {
  return {
    $typeName: 'issho.v1.Outing',
    id: 'o1',
    planId: 'p1',
    date: '2026-11-25',
    part: 'morning',
    startsAt: '09:00',
    languages: ['en', 'vi'],
    seatState: SeatState.INVITED,
    invitedAt: at('2026-11-21T16:00:00Z'),
    invitationEndsAt: at('2026-11-23T16:00:00Z'),
    outingState: OutingState.OFFERED,
    yesCount: 2,
    othersGoing: [],
    checkedIn: false,
    checkedInNames: [],
    ...over,
  } as Outing;
}
