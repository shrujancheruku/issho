/// <reference types="jest" />
import { EventStatus, GroupStatus, InviteResponse, type MyEvent } from '@issho/api-client';

import { outingStage } from '../stage';

// Only seconds are read; the cast skips protobuf's message brand.
const at = (iso: string) => ({ seconds: BigInt(Date.parse(iso) / 1000), nanos: 0 }) as MyEvent['startsAt'] & {};
const start = Date.parse('2030-01-12T14:00:00Z');

function event(over: Partial<MyEvent>): MyEvent {
  return {
    status: EventStatus.SCHEDULED,
    groupStatus: GroupStatus.CONFIRMED,
    response: InviteResponse.CONFIRMED,
    confirmBy: at('2030-01-11T00:00:00Z'),
    startsAt: at('2030-01-12T14:00:00Z'),
    checkInFrom: at('2030-01-12T13:30:00Z'),
    checkInUntil: at('2030-01-12T17:00:00Z'),
    ...over,
  } as MyEvent;
}

describe('outingStage', () => {
  it('puts an invitation, a decline and a missed answer at "date set"', () => {
    expect(outingStage(event({ response: InviteResponse.INVITED }), start - 2 * 86_400_000)).toBe(1);
    expect(outingStage(event({ response: InviteResponse.DECLINED }), start - 1)).toBe(1);
  });

  it('is "going" once confirmed, until the day', () => {
    expect(outingStage(event({}), start - 2 * 86_400_000)).toBe(2);
    expect(outingStage(event({ groupStatus: GroupStatus.INVITING }), start - 2 * 86_400_000)).toBe(2);
  });

  it('is "on the day" from check-in (at the latest) until check-in closes', () => {
    // 20 minutes before the start is on the start's day in every time zone.
    expect(outingStage(event({}), start - 20 * 60_000)).toBe(3);
    expect(outingStage(event({ checkedIn: true }), start + 60_000)).toBe(3);
    expect(outingStage(event({}), Date.parse('2030-01-12T17:00:01Z'))).toBe(2);
  });

  it('is "how was it" while feedback is open, and once the outing is done', () => {
    const fb = { feedbackFrom: at('2030-01-12T17:00:00Z'), feedbackUntil: at('2030-01-26T17:00:00Z') };
    expect(outingStage(event(fb), Date.parse('2030-01-13T10:00:00Z'))).toBe(4);
    expect(outingStage(event({ status: EventStatus.DONE }), Date.parse('2030-01-13T10:00:00Z'))).toBe(4);
  });
});
