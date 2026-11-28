import { Code, ConnectError } from '@connectrpc/connect';
import { OutingState, SeatState, type Feedback, type Outing } from '@issho/api-client';

/** An outing's seat holder can still answer or has said yes: the outing is still ahead of them. */
const OPEN_SEATS: SeatState[] = [SeatState.INVITED, SeatState.ACCEPTED, SeatState.GOING];
const OPEN_OUTINGS: OutingState[] = [OutingState.OFFERED, OutingState.SET];

const endsAt = (o: Outing) => (o.invitationEndsAt ? Number(o.invitationEndsAt.seconds) * 1000 : 0);

/** The invitation's fixed 48 hours have not ended (the device's clock; the server decides, and refuses a late answer). */
export const invitationOpen = (o: Outing, now: Date) => endsAt(o) > now.getTime();

/** An invitation that needs an answer: "Needs you" (docs/spec/app-shape.md). */
export const needsAnswer = (o: Outing, now: Date) => o.seatState === SeatState.INVITED && OPEN_OUTINGS.includes(o.outingState) && invitationOpen(o, now);

/** Says yes: an invitation, or a no changed inside the 48 hours. */
export const canSayYes = (o: Outing, now: Date) => (o.seatState === SeatState.INVITED || o.seatState === SeatState.DECLINED) && OPEN_OUTINGS.includes(o.outingState) && invitationOpen(o, now);

type Stamp = { seconds: bigint } | undefined;
/** A timestamp the server sent as a time in milliseconds; undefined when it sent none. */
const ms = (t: Stamp) => (t ? Number(t.seconds) * 1000 : undefined);
/** A timestamp the server sent as a Date, for format.ts to write on the city's clock. */
export const stampDate = (t: Stamp) => new Date(ms(t) ?? 0);

/**
 * Whether `now` is in [from, to): the instant a window opens counts, the
 * instant it closes does not, as the server draws them. The windows are the
 * server's own (`Outing.check_in_*` and `feedback_*`), never worked out here.
 */
const inWindow = (from: Stamp, to: Stamp, now: Date) => ms(from) !== undefined && ms(to) !== undefined && ms(from)! <= now.getTime() && now.getTime() < ms(to)!;

/**
 * The outing has ended: the server says so, or the plan's stated end has gone
 * by. The server does not move an outing to over yet, and the end is the one
 * instant it already sends (check-in's close).
 */
export const outingOver = (o: Outing, now: Date) => o.outingState === OutingState.OVER || (ms(o.checkInClosesAt) !== undefined && now.getTime() >= ms(o.checkInClosesAt)!);

/** Shown muted: an outing that is over, not set or cancelled, or one the person is out of (docs/spec/outings.md). */
export const muted = (o: Outing, now: Date = new Date()) => !OPEN_OUTINGS.includes(o.outingState) || !OPEN_SEATS.includes(o.seatState) || outingOver(o, now);

/** Where check-in stands for a person going: before its window, in it, or after it. */
export function checkInStage(o: Outing, now: Date): 'early' | 'open' | 'closed' {
  if (inWindow(o.checkInOpensAt, o.checkInClosesAt, now)) return 'open';
  return ms(o.checkInOpensAt) !== undefined && now.getTime() < ms(o.checkInOpensAt)! ? 'early' : 'closed';
}

/** A going seat in a set outing: the one who can check in and the one asked for feedback. */
export const isGoing = (o: Outing) => o.seatState === SeatState.GOING && (o.outingState === OutingState.SET || o.outingState === OutingState.OVER);

/** "I'm here" is offered: going, not checked in, inside check-in's window. */
export const canCheckIn = (o: Outing, now: Date) => isGoing(o) && !o.checkedIn && checkInStage(o, now) === 'open';

/** Check-in has not opened yet for a person going: the screen says when it will. */
export const checkInComingUp = (o: Outing, now: Date) => isGoing(o) && !o.checkedIn && checkInStage(o, now) === 'early' && ms(o.checkInOpensAt) !== undefined;

/**
 * Feedback is open for this seat: from the plan's end for 7 days, to a person
 * who is going (a seat that was going at the start and is still going; the
 * server decides the rest and refuses anyone it should).
 */
export const feedbackOpen = (o: Outing, now: Date) => isGoing(o) && inWindow(o.feedbackOpensAt, o.feedbackClosesAt, now);

/** Feedback has closed for a person who was going. */
export const feedbackClosed = (o: Outing, now: Date) => isGoing(o) && ms(o.feedbackClosesAt) !== undefined && now.getTime() >= ms(o.feedbackClosesAt)!;

/**
 * The person's feedback form as loaded for one outing. It is held in memory
 * only, for the signed-in person: it carries their own picks, which are
 * private, so it is never a cache key, a log line or part of an address.
 */
export type FeedbackRead = { status: 'loading' } | { status: 'failed' } | { status: 'ready'; feedback: Feedback };

/** Feedback is waiting for an answer: open, and not answered yet (or its form could not be read, so it is not known to be done). */
export function needsFeedback(o: Outing, now: Date, read: FeedbackRead | undefined) {
  if (!feedbackOpen(o, now) || !read || read.status === 'loading') return false;
  return read.status === 'failed' || !read.feedback.answered;
}

/** The person has something ahead of them in an outing: a seat still open in an outing that is not over, or feedback to give. */
export const inPlayOutings = (outings: Outing[], now: Date = new Date()) => outings.some((o) => !muted(o, now) || feedbackOpen(o, now));

/** The outing of a plan to show on its page: one the person is in, then one open to answer, then the most recent. */
export function outingOfPlan(outings: Outing[], planId: string, now: Date): Outing | undefined {
  const mine = outings.filter((o) => o.planId === planId);
  // Feedback still to give ranks with an outing ahead of the person: it is the page they need.
  const rank = (o: Outing) => (o.seatState === SeatState.GOING && (!muted(o, now) || feedbackOpen(o, now)) ? 0 : !muted(o, now) ? 1 : 2);
  return [...mine].sort((a, b) => rank(a) - rank(b) || (a.date < b.date ? 1 : a.date > b.date ? -1 : 0))[0];
}

/** What a refused answer or cancel comes to, in words the screen has strings for. */
export type OutingRefusal = 'gone' | 'timeTaken' | 'ranOut' | 'full' | 'failed' | 'checkedIn' | 'cannotCancel' | 'cancelFailed' | 'checkInEarly' | 'checkInOver' | 'checkInFailed';

/**
 * Why the server refused. It answers FAILED_PRECONDITION with a message and no
 * structured reason, so the cause is read from the outing as the server now
 * has it (the app reloads before asking): a full seat is full; an invitation
 * past its 48 hours, not set, or unanswered has run out; a yes on a seat still
 * open can only have been the time taken by another outing. An outing the
 * server no longer gives the person a seat in (NOT_FOUND, PERMISSION_DENIED) is
 * gone whatever the action, as trying again cannot help. Any other failure is a
 * plain failure.
 */
export function refusalOf(err: unknown, action: 'answer' | 'cancel' | 'checkIn', after: Outing | undefined, now: Date): OutingRefusal {
  const code = ConnectError.from(err).code;
  if (code === Code.NotFound || code === Code.PermissionDenied) return 'gone';
  if (action === 'checkIn') {
    // Check-in is refused outside its window (or for a seat not going): the window is the outing's own, so the screen can say which side of it.
    if (ConnectError.from(err).code !== Code.FailedPrecondition || !after) return 'checkInFailed';
    const stage = checkInStage(after, now);
    return stage === 'early' ? 'checkInEarly' : stage === 'closed' ? 'checkInOver' : 'checkInFailed';
  }
  if (ConnectError.from(err).code !== Code.FailedPrecondition || !after) return action === 'cancel' ? 'cancelFailed' : 'failed';
  if (action === 'cancel') return after.checkedIn ? 'checkedIn' : 'cannotCancel';
  if (after.seatState === SeatState.FULL) return 'full';
  if (!OPEN_OUTINGS.includes(after.outingState) || after.seatState === SeatState.UNANSWERED || !invitationOpen(after, now)) return 'ranOut';
  return 'timeTaken';
}

/** What a refused feedback save comes to: its window is the outing's own, so the screen can say which side of it. */
export type FeedbackRefusal = 'notOpen' | 'closed' | 'failed';

export function feedbackRefusalOf(err: unknown, o: Outing | undefined, now: Date): FeedbackRefusal {
  if (ConnectError.from(err).code !== Code.FailedPrecondition || !o) return 'failed';
  if (ms(o.feedbackClosesAt) !== undefined && now.getTime() >= ms(o.feedbackClosesAt)!) return 'closed';
  if (ms(o.feedbackOpensAt) !== undefined && now.getTime() < ms(o.feedbackOpensAt)!) return 'notOpen';
  return 'failed';
}

/** What needs the person's answer or word: invitations to answer and feedback to give, soonest outing first, each tagged with which it is. */
export type NeedsYou = { kind: 'invitation' | 'feedback'; outing: Outing };

export function needsYouList(outings: Outing[], now: Date, feedbacks: Record<string, FeedbackRead>): NeedsYou[] {
  const list: NeedsYou[] = [];
  for (const outing of outings) {
    if (needsAnswer(outing, now)) list.push({ kind: 'invitation', outing });
    else if (needsFeedback(outing, now, feedbacks[outing.id])) list.push({ kind: 'feedback', outing });
  }
  return list.sort((a, b) => (a.outing.date < b.outing.date ? -1 : a.outing.date > b.outing.date ? 1 : 0));
}
