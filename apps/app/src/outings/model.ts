import { Code, ConnectError } from '@connectrpc/connect';
import { OutingState, SeatState, type Outing } from '@issho/api-client';

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

/** Shown muted: an outing that is over, not set or cancelled, or one the person is out of (docs/spec/outings.md). */
export const muted = (o: Outing) => !OPEN_OUTINGS.includes(o.outingState) || !OPEN_SEATS.includes(o.seatState);

/** The person has something ahead of them in an outing: a seat still open in an outing that is not over. */
export const inPlayOutings = (outings: Outing[]) => outings.some((o) => !muted(o));

/** The outing of a plan to show on its page: one the person is in, then one open to answer, then the most recent. */
export function outingOfPlan(outings: Outing[], planId: string, now: Date): Outing | undefined {
  const mine = outings.filter((o) => o.planId === planId);
  const rank = (o: Outing) => (o.seatState === SeatState.GOING && !muted(o) ? 0 : !muted(o) ? 1 : 2);
  return [...mine].sort((a, b) => rank(a) - rank(b) || (a.date < b.date ? 1 : a.date > b.date ? -1 : 0))[0];
}

/** What a refused answer or cancel comes to, in words the screen has strings for. */
export type OutingRefusal = 'timeTaken' | 'ranOut' | 'full' | 'failed' | 'checkedIn' | 'cannotCancel' | 'cancelFailed';

/**
 * Why the server refused. It answers FAILED_PRECONDITION with a message and no
 * structured reason, so the cause is read from the outing as the server now
 * has it (the app reloads before asking): a full seat is full; an invitation
 * past its 48 hours, not set, or unanswered has run out; a yes on a seat still
 * open can only have been the time taken by another outing. Any other failure
 * is a plain failure.
 */
export function refusalOf(err: unknown, action: 'answer' | 'cancel', after: Outing | undefined, now: Date): OutingRefusal {
  if (ConnectError.from(err).code !== Code.FailedPrecondition || !after) return action === 'cancel' ? 'cancelFailed' : 'failed';
  if (action === 'cancel') return after.checkedIn ? 'checkedIn' : 'cannotCancel';
  if (after.seatState === SeatState.FULL) return 'full';
  if (!OPEN_OUTINGS.includes(after.outingState) || after.seatState === SeatState.UNANSWERED || !invitationOpen(after, now)) return 'ranOut';
  return 'timeTaken';
}
