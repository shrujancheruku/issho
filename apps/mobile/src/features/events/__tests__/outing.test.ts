/// <reference types="jest" />
import { EventStatus, GroupStatus, InviteResponse, type MyEvent } from '@issho/api-client';

import { outing } from '../outing';

// Only seconds are read; the cast skips protobuf's message brand.
const at = (iso: string) => ({ seconds: BigInt(Date.parse(iso) / 1000), nanos: 0 }) as MyEvent['startsAt'] & {};
const deadline = Date.parse('2030-01-11T00:00:00Z');
const start = Date.parse('2030-01-12T14:00:00Z');

function event(over: Partial<MyEvent>): MyEvent {
  return {
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

describe('outing: message and actions', () => {
  it('asks for an answer until the deadline, then says it was missed', () => {
    expect(outing(event({}), deadline - 1)).toMatchObject({ message: 'event.state.answer', actions: ['confirm', 'decline'] });
    expect(outing(event({}), deadline)).toMatchObject({ message: 'event.state.noAnswer', actions: [] });
  });

  it('tells a confirmed person whether the group is going ahead; they can cancel until the start', () => {
    const waiting = event({ response: InviteResponse.CONFIRMED });
    expect(outing(waiting, deadline - 1)).toMatchObject({ message: 'event.state.waiting', actions: ['cancel'] });
    const going = event({ response: InviteResponse.CONFIRMED, groupStatus: GroupStatus.CONFIRMED });
    // Check-in is open by then too (see below).
    expect(outing(going, start - 1)).toMatchObject({ message: 'event.state.going', tone: 'good', actions: ['checkIn', 'cancel'] });
    // From the start, cancelling is over; checking in isn't.
    expect(outing(going, start).actions).toEqual(['checkIn']);
  });

  it('offers check-in from 30 minutes before the start until 3 hours after, first', () => {
    const going = event({ response: InviteResponse.CONFIRMED, groupStatus: GroupStatus.CONFIRMED });
    expect(outing(going, start - 31 * 60_000).actions).toEqual(['cancel']);
    expect(outing(going, start - 30 * 60_000).actions).toEqual(['checkIn', 'cancel']);
    expect(outing(going, start + 60_000).actions).toEqual(['checkIn']);
    expect(outing(going, start + 3 * 3_600_000 + 1).actions).toEqual([]);
    expect(outing(event({ response: InviteResponse.CONFIRMED, checkedIn: true }), start)).toMatchObject({ message: 'event.state.checkedIn', actions: [] });
  });

  it('asks for feedback for two weeks after the outing, then lets it be changed', () => {
    const went = event({
      response: InviteResponse.CONFIRMED,
      groupStatus: GroupStatus.CONFIRMED,
      feedbackFrom: at('2030-01-12T17:00:00Z'),
      feedbackUntil: at('2030-01-26T14:00:00Z'),
    } as Partial<MyEvent>);
    const opens = start + 3 * 3_600_000;
    expect(outing(went, opens - 1).actions).toEqual(['checkIn']);
    expect(outing(went, opens)).toMatchObject({ message: 'event.state.feedback', tone: 'action', actions: ['feedback'] });
    // Still asked once the event is marked done.
    expect(outing({ ...went, status: EventStatus.DONE }, opens + 86_400_000).actions).toEqual(['feedback']);
    expect(outing({ ...went, feedbackGiven: true }, opens)).toMatchObject({ message: 'event.state.feedbackGiven', tone: 'muted', actions: ['feedback'] });
    expect(outing({ ...went, status: EventStatus.DONE }, Date.parse('2030-01-26T14:00:00Z'))).toMatchObject({ message: 'event.state.done', actions: [] });
    // No window (didn't go ahead with the group): never asked.
    expect(outing(event({ status: EventStatus.DONE, response: InviteResponse.DROPPED }), opens).actions).toEqual([]);
  });

  it('lets a decline be changed until the deadline', () => {
    const declined = event({ response: InviteResponse.DECLINED });
    expect(outing(declined, deadline - 1).actions).toEqual(['confirm']);
    expect(outing(declined, deadline).actions).toEqual([]);
  });

  it('offers nothing once cancelled or done', () => {
    expect(outing(event({ groupStatus: GroupStatus.CANCELLED }), 0).message).toBe('event.state.cancelled');
    expect(outing(event({ status: EventStatus.CANCELLED, response: InviteResponse.CONFIRMED }), 0).actions).toEqual([]);
    expect(outing(event({ status: EventStatus.DONE, response: InviteResponse.CONFIRMED }), 0).message).toBe('event.state.done');
  });
});

const day = 86_400_000;
const going = (over: Partial<MyEvent> = {}) => event({ response: InviteResponse.CONFIRMED, groupStatus: GroupStatus.CONFIRMED, ...over });

describe('outing: stage', () => {
  it('puts an invitation, a decline and a missed answer at "invited"', () => {
    expect(outing(event({}), start - 2 * day).stage).toBe('invited');
    expect(outing(event({ response: InviteResponse.DECLINED }), start - 1).stage).toBe('invited');
    expect(outing(event({}), start - 1).stage).toBe('invited');
  });

  it('is "going" once confirmed, until the day', () => {
    expect(outing(going(), start - 2 * day).stage).toBe('going');
    expect(outing(event({ response: InviteResponse.CONFIRMED }), start - 2 * day).stage).toBe('going');
  });

  it('is "onTheDay" from check-in (at the latest) until check-in closes', () => {
    // 20 minutes before the start is on the start's day in every time zone.
    expect(outing(going(), start - 20 * 60_000).stage).toBe('onTheDay');
    expect(outing(going({ checkedIn: true }), start + 60_000).stage).toBe('onTheDay');
    expect(outing(going(), Date.parse('2030-01-12T17:00:01Z')).stage).toBe('going');
  });

  it('is "after" while feedback is open, and once the outing is done', () => {
    const fb = { feedbackFrom: at('2030-01-12T17:00:00Z'), feedbackUntil: at('2030-01-26T17:00:00Z') };
    expect(outing(going(fb), Date.parse('2030-01-13T10:00:00Z')).stage).toBe('after');
    expect(outing(going({ status: EventStatus.DONE }), Date.parse('2030-01-13T10:00:00Z')).stage).toBe('after');
  });

  it('maps each stage to its thread knot', () => {
    expect(outing(event({}), start - 2 * day).threadKnot).toBe(1);
    expect(outing(going(), start - 2 * day).threadKnot).toBe(2);
    expect(outing(going(), start - 20 * 60_000).threadKnot).toBe(3);
    expect(outing(going({ status: EventStatus.DONE }), start + day).threadKnot).toBe(4);
  });
});

describe('outing: what the screen reads', () => {
  const fb = { feedbackFrom: at('2030-01-12T17:00:00Z'), feedbackUntil: at('2030-01-26T17:00:00Z') };
  const afterFb = Date.parse('2030-01-13T10:00:00Z');
  const early = start - 2 * day;

  // [name, event, now, expected flags]
  const cases: [string, MyEvent, number, Partial<ReturnType<typeof outing>>][] = [
    ['invited, open', event({}), early, { view: 'invited', muted: false, live: true, chatFirst: false, canConfirm: true, canCancel: false, canCheckIn: false, canShare: false, canReport: false, needsAnswer: true, needsCheckIn: false, needsFeedback: false }],
    ['invited, deadline missed', event({}), deadline, { view: 'muted', muted: true, live: false, canConfirm: false, needsAnswer: false }],
    ['declined, can change', event({ response: InviteResponse.DECLINED }), early, { view: 'muted', muted: true, canConfirm: true, needsAnswer: false }],
    ['going, group in', going(), early, { view: 'going', muted: false, live: true, chatFirst: true, canCancel: true, canCheckIn: false, canShare: true, canReport: true, needsAnswer: false }],
    ['confirmed, group waiting', event({ response: InviteResponse.CONFIRMED }), early, { view: 'going', chatFirst: true, canShare: true }],
    ['on the day, check-in open', going(), start - 10 * 60_000, { view: 'onTheDay', live: false, chatFirst: false, canCheckIn: true, canCancel: true, needsCheckIn: true }],
    ['on the day, before check-in', going(), start - 20 * 60_000 - 20 * 60_000, { view: 'onTheDay', canCheckIn: false, needsCheckIn: false }],
    ['checked in', going({ checkedIn: true }), start + 60_000, { view: 'onTheDay', canCheckIn: true, needsCheckIn: false, canCancel: false }],
    ['feedback asked', going(fb), afterFb, { view: 'after', muted: false, canGiveFeedback: true, needsFeedback: true, canShare: true, canReport: true }],
    ['feedback given', going({ ...fb, feedbackGiven: true }), afterFb, { view: 'after', muted: false, tone: 'muted', canGiveFeedback: true, needsFeedback: false }],
    ['done, no window', going({ status: EventStatus.DONE }), afterFb, { view: 'after', muted: false, canGiveFeedback: false, needsFeedback: false, canShare: false }],
    ['cancelled outing', going({ status: EventStatus.CANCELLED }), early, { view: 'muted', muted: true, canShare: false, canReport: true, canCancel: false }],
    ['dropped', event({ response: InviteResponse.DROPPED }), early, { view: 'muted', muted: true, canReport: true, canShare: false }],
  ];
  it.each(cases)('%s', (_name, e, now, expected) => {
    expect(outing(e, now)).toMatchObject(expected);
  });

  it('closes reporting at report_until', () => {
    const e = going({ reportUntil: at('2030-01-20T00:00:00Z') });
    expect(outing(e, Date.parse('2030-01-20T00:00:00Z')).canReport).toBe(true);
    expect(outing(e, Date.parse('2030-01-20T00:00:01Z')).canReport).toBe(false);
  });

  it('says where to meet: the set point, the reveal time, or the first stop', () => {
    expect(outing(going({ meetingPoint: 'The red gate' }), early).meetingPoint).toEqual({ kind: 'set', text: 'The red gate' });
    const reveal = at('2030-01-12T08:00:00Z');
    expect(outing(going({ revealAt: reveal }), early).meetingPoint).toEqual({ kind: 'later', revealAt: reveal });
    expect(outing(going({ revealAt: reveal }), Date.parse('2030-01-12T08:00:00Z')).meetingPoint).toEqual({ kind: 'firstStop' });
    expect(outing(going(), early).meetingPoint).toEqual({ kind: 'firstStop' });
  });
});
