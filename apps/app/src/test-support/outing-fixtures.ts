import { OutingState, SeatState, type Feedback, type Outing } from '@issho/api-client';

const at = (iso: string) => ({ $typeName: 'google.protobuf.Timestamp' as const, seconds: BigInt(Math.floor(Date.parse(iso) / 1000)), nanos: 0 });

/**
 * An outing as ListMyOutings returns it (proto/issho/v1/outing.proto), for
 * screen tests: invited to Lantern making on Wed 25 November at 09:00 in Da
 * Nang, the invitation open until 23 Nov 16:00 UTC (the tests' "now" is 22 Nov
 * 10:00 UTC), two have said yes, the group shares English and Vietnamese. The
 * plan runs 09:00 to 12:30 on the city's clock, so check-in opens at 08:30
 * (01:30 UTC), closes at 12:30 (05:30 UTC), and feedback opens then and closes
 * 7 days later, as the server computes them.
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
    checkInOpensAt: at('2026-11-25T01:30:00Z'),
    checkInClosesAt: at('2026-11-25T05:30:00Z'),
    feedbackOpensAt: at('2026-11-25T05:30:00Z'),
    feedbackClosesAt: at('2026-12-02T05:30:00Z'),
    ...over,
  } as Outing;
}

/**
 * A feedback form as GetFeedback returns it (proto/issho/v1/outing.proto) for
 * the fixture outing: nothing answered yet, two others who were going, named by
 * first name and a per-outing handle, and feedback closing 2 Dec 05:30 UTC.
 */
export function aFeedback(over: Partial<Feedback> = {}): Feedback {
  return {
    $typeName: 'issho.v1.Feedback',
    outingId: 'o1',
    answered: false,
    went: false,
    others: [
      { $typeName: 'issho.v1.Companion', handle: 'h-linh', firstName: 'Linh' },
      { $typeName: 'issho.v1.Companion', handle: 'h-minh', firstName: 'Minh' },
    ],
    notThere: [],
    meetAgain: [],
    closesAt: at('2026-12-02T05:30:00Z'),
    ...over,
  } as Feedback;
}
