import { EventStatus, GroupStatus, InviteResponse, type MyEvent } from '@issho/api-client';

import type { StringKey } from '@/i18n/strings';

export type EventAction = 'confirm' | 'decline' | 'cancel' | 'checkIn';

/** What to tell the person about an event, and what they can do now. */
export type EventState = {
  message: StringKey;
  /** 'action' needs their answer; 'good' is going; 'muted' is over or out. */
  tone: 'action' | 'good' | 'waiting' | 'muted';
  actions: EventAction[];
};

const ms = (ts: MyEvent['confirmBy']) => (ts ? Number(ts.seconds) * 1000 : NaN);

/**
 * The event from the person's side, following EventService's rules: answer
 * (confirm or decline, or change a decline) until confirm_by; cancel a
 * confirmed place until it starts; check in from 30 minutes before the
 * start until 3 hours after.
 */
export function eventState(e: MyEvent, now: number): EventState {
  const beforeDeadline = now < ms(e.confirmBy);
  const beforeStart = Number.isNaN(ms(e.startsAt)) || now < ms(e.startsAt);
  if (e.status === EventStatus.DONE) return { message: 'event.state.done', tone: 'muted', actions: [] };
  if (e.status === EventStatus.CANCELLED || e.groupStatus === GroupStatus.CANCELLED) {
    return { message: 'event.state.cancelled', tone: 'muted', actions: [] };
  }
  switch (e.response) {
    case InviteResponse.INVITED:
      return beforeDeadline
        ? { message: 'event.state.answer', tone: 'action', actions: ['confirm', 'decline'] }
        : { message: 'event.state.noAnswer', tone: 'muted', actions: [] };
    case InviteResponse.CONFIRMED: {
      if (e.checkedIn) return { message: 'event.state.checkedIn', tone: 'good', actions: [] };
      const checkInOpen = now >= ms(e.checkInFrom) && now <= ms(e.checkInUntil);
      // Checking in comes first: on the day it's the one thing to do.
      const actions: EventAction[] = [...(checkInOpen ? (['checkIn'] as const) : []), ...(beforeStart ? (['cancel'] as const) : [])];
      return {
        message: e.groupStatus === GroupStatus.CONFIRMED ? 'event.state.going' : 'event.state.waiting',
        tone: e.groupStatus === GroupStatus.CONFIRMED ? 'good' : 'waiting',
        actions,
      };
    }
    case InviteResponse.DECLINED:
      return { message: 'event.state.declined', tone: 'muted', actions: beforeDeadline ? ['confirm'] : [] };
    case InviteResponse.DROPPED:
      return { message: 'event.state.dropped', tone: 'muted', actions: [] };
    default:
      return { message: 'event.state.cancelled', tone: 'muted', actions: [] };
  }
}
