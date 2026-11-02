import { EventStatus, GroupStatus, InviteResponse, type MyEvent } from '@issho/api-client';

import { lastSignupDay } from '@/features/plans/format';
import type { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { ms } from '@/lib/time';

export type EventAction = 'confirm' | 'decline' | 'cancel' | 'checkIn' | 'feedback';

/** What to tell the person about an event, and what they can do now. */
type EventState = {
  message: StringKey;
  /** 'action' needs their answer; 'good' is going; 'muted' is over or out. */
  tone: 'action' | 'good' | 'waiting' | 'muted';
  actions: EventAction[];
};

/**
 * The status and actions from the person's side, following EventService's
 * rules: answer (confirm or decline, or change a decline) until confirm_by;
 * cancel a confirmed place until it starts; check in from 30 minutes before
 * the start until 3 hours after; then give feedback (or change it) for two
 * weeks, if they went ahead with their group.
 */
function eventState(e: MyEvent, now: number): EventState {
  const beforeDeadline = now < ms(e.confirmBy);
  const beforeStart = Number.isNaN(ms(e.startsAt)) || now < ms(e.startsAt);
  // Unset (NaN) unless they went ahead with their group: never open.
  if (now >= ms(e.feedbackFrom) && now < ms(e.feedbackUntil)) {
    return e.feedbackGiven
      ? { message: 'event.state.feedbackGiven', tone: 'muted', actions: ['feedback'] }
      : { message: 'event.state.feedback', tone: 'action', actions: ['feedback'] };
  }
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

/** The local calendar day of a time, for "on the day". */
const dayOf = (t: number) => new Date(t).toDateString();

/**
 * Where an outing stands on its thread (components/thread.tsx), after
 * "signed up", which only a plan has: invited (date set, or out of it),
 * going, on the day, and after (how was it). "On the day" runs from the
 * start of the outing's local day (or check-in, if that opens earlier)
 * until check-in closes.
 */
export type Stage = 'invited' | 'going' | 'onTheDay' | 'after';

/** The thread's knot for each stage (knot 0 is "signed up"). */
const knot: Record<Stage, number> = { invited: 1, going: 2, onTheDay: 3, after: 4 };

function stageOf(e: MyEvent, now: number): Stage {
  if (now >= ms(e.feedbackFrom) && now < ms(e.feedbackUntil)) return 'after';
  if (e.response !== InviteResponse.CONFIRMED) return 'invited';
  if (e.status === EventStatus.DONE) return 'after';
  const start = ms(e.startsAt);
  const checkInUntil = ms(e.checkInUntil);
  const onTheDay = (!Number.isNaN(start) && dayOf(start) === dayOf(now)) || now >= ms(e.checkInFrom);
  if (onTheDay && (Number.isNaN(checkInUntil) || now <= checkInUntil)) return 'onTheDay';
  return 'going';
}

/** Where to meet: said outright, revealed later, or (nothing set, nothing pending) the first stop. */
export type MeetingPoint = { kind: 'set'; text: string } | { kind: 'later'; revealAt: MyEvent['revealAt'] } | { kind: 'firstStop' };

/**
 * An outing as the person sees it at one moment: the one place the app's
 * rules about it live, so the event screen and Together only read it.
 * `message` is display data (an i18n key); decisions use the flags.
 */
export type Outing = EventState & {
  stage: Stage;
  /** The thread's knot for this stage. */
  threadKnot: number;
  /** Over, out or not happening (the feedback window keeps its own screen). */
  muted: boolean;
  /** The screen to show: the stage, or 'muted' when it's over or out. */
  view: Stage | 'muted';
  /** The thread's live dot; on the day the time's own dot is the screen's one live thing. */
  live: boolean;
  /** Going: the chat fills the screen under a fixed head, so it can't scroll with the rest. */
  chatFirst: boolean;
  meetingPoint: MeetingPoint;
  canConfirm: boolean;
  canCancel: boolean;
  /** Check-in is open, or already done (the button then shows its result). */
  canCheckIn: boolean;
  /** Share with someone they trust. */
  canShare: boolean;
  /** Anyone who confirmed, even if they cancelled since, until the server's report_until. */
  canReport: boolean;
  /** Feedback can be given or changed. */
  canGiveFeedback: boolean;
  /** Together's "needs you" kinds. */
  needsAnswer: boolean;
  needsCheckIn: boolean;
  needsFeedback: boolean;
};

export function outing(e: MyEvent, now: number): Outing {
  const state = eventState(e, now);
  const stage = stageOf(e, now);
  const muted = state.tone === 'muted' && stage !== 'after';
  return {
    ...state,
    stage,
    threadKnot: knot[stage],
    muted,
    view: muted ? 'muted' : stage,
    live: !muted && stage !== 'onTheDay',
    chatFirst: stage === 'going' && !muted,
    meetingPoint: e.meetingPoint
      ? { kind: 'set', text: e.meetingPoint }
      : now < ms(e.revealAt)
        ? { kind: 'later', revealAt: e.revealAt }
        : { kind: 'firstStop' },
    canConfirm: state.actions.includes('confirm'),
    canCancel: state.actions.includes('cancel'),
    canCheckIn: state.actions.includes('checkIn') || !!e.checkedIn,
    canShare: e.response === InviteResponse.CONFIRMED && e.status === EventStatus.SCHEDULED,
    canReport:
      (e.response === InviteResponse.CONFIRMED || e.response === InviteResponse.DROPPED) && (!e.reportUntil || now <= ms(e.reportUntil)),
    canGiveFeedback: state.actions.includes('feedback'),
    needsAnswer: e.response === InviteResponse.INVITED && state.actions.includes('confirm'),
    needsCheckIn: state.actions.includes('checkIn'),
    needsFeedback: state.message === 'event.state.feedback',
  };
}

/** The status line for an event, with the answer-by date filled in. */
export function stateText(t: ReturnType<typeof useI18n>['t'], o: Pick<Outing, 'message'>, e: MyEvent, lang: string) {
  // Answer by the day before confirm_by (local midnight), as for signups.
  return t(o.message, { date: lastSignupDay(e.confirmBy, lang) });
}
