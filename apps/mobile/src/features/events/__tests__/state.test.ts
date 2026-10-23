/// <reference types="jest" />
import { EventStatus, GroupStatus, InviteResponse, type MyEvent } from '@issho/api-client';

import { eventState } from '../state';

const at = (iso: string) => ({ seconds: BigInt(Date.parse(iso) / 1000), nanos: 0 });
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

describe('eventState', () => {
  it('asks for an answer until the deadline, then says it was missed', () => {
    expect(eventState(event({}), deadline - 1)).toMatchObject({ message: 'event.state.answer', actions: ['confirm', 'decline'] });
    expect(eventState(event({}), deadline)).toMatchObject({ message: 'event.state.noAnswer', actions: [] });
  });

  it('tells a confirmed person whether the group is going ahead; they can cancel until the start', () => {
    const waiting = event({ response: InviteResponse.CONFIRMED });
    expect(eventState(waiting, deadline - 1)).toMatchObject({ message: 'event.state.waiting', actions: ['cancel'] });
    const going = event({ response: InviteResponse.CONFIRMED, groupStatus: GroupStatus.CONFIRMED });
    // Check-in is open by then too (see below).
    expect(eventState(going, start - 1)).toMatchObject({ message: 'event.state.going', tone: 'good', actions: ['checkIn', 'cancel'] });
    // From the start, cancelling is over; checking in isn't.
    expect(eventState(going, start).actions).toEqual(['checkIn']);
  });

  it('offers check-in from 30 minutes before the start until 3 hours after, first', () => {
    const going = event({ response: InviteResponse.CONFIRMED, groupStatus: GroupStatus.CONFIRMED });
    expect(eventState(going, start - 31 * 60_000).actions).toEqual(['cancel']);
    expect(eventState(going, start - 30 * 60_000).actions).toEqual(['checkIn', 'cancel']);
    expect(eventState(going, start + 60_000).actions).toEqual(['checkIn']);
    expect(eventState(going, start + 3 * 3_600_000 + 1).actions).toEqual([]);
    expect(eventState(event({ response: InviteResponse.CONFIRMED, checkedIn: true }), start)).toMatchObject({ message: 'event.state.checkedIn', actions: [] });
  });

  it('asks for feedback for two weeks after the outing, then lets it be changed', () => {
    const went = event({
      response: InviteResponse.CONFIRMED,
      groupStatus: GroupStatus.CONFIRMED,
      feedbackFrom: at('2030-01-12T17:00:00Z'),
      feedbackUntil: at('2030-01-26T14:00:00Z'),
    } as Partial<MyEvent>);
    const opens = start + 3 * 3_600_000;
    expect(eventState(went, opens - 1).actions).toEqual(['checkIn']);
    expect(eventState(went, opens)).toMatchObject({ message: 'event.state.feedback', tone: 'action', actions: ['feedback'] });
    // Still asked once the event is marked done.
    expect(eventState({ ...went, status: EventStatus.DONE }, opens + 86_400_000).actions).toEqual(['feedback']);
    expect(eventState({ ...went, feedbackGiven: true }, opens)).toMatchObject({ message: 'event.state.feedbackGiven', tone: 'muted', actions: ['feedback'] });
    expect(eventState({ ...went, status: EventStatus.DONE }, Date.parse('2030-01-26T14:00:00Z'))).toMatchObject({ message: 'event.state.done', actions: [] });
    // No window (didn't go ahead with the group): never asked.
    expect(eventState(event({ status: EventStatus.DONE, response: InviteResponse.DROPPED }), opens).actions).toEqual([]);
  });

  it('lets a decline be changed until the deadline', () => {
    const declined = event({ response: InviteResponse.DECLINED });
    expect(eventState(declined, deadline - 1).actions).toEqual(['confirm']);
    expect(eventState(declined, deadline).actions).toEqual([]);
  });

  it('offers nothing once cancelled or done', () => {
    expect(eventState(event({ groupStatus: GroupStatus.CANCELLED }), 0).message).toBe('event.state.cancelled');
    expect(eventState(event({ status: EventStatus.CANCELLED, response: InviteResponse.CONFIRMED }), 0).actions).toEqual([]);
    expect(eventState(event({ status: EventStatus.DONE, response: InviteResponse.CONFIRMED }), 0).message).toBe('event.state.done');
  });
});
