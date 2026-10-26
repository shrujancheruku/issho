import { EventStatus, InviteResponse, type MyEvent } from '@issho/api-client';

const ms = (ts: MyEvent['startsAt']) => (ts ? Number(ts.seconds) * 1000 : NaN);

/** The local calendar day of a time, for "on the day". */
const dayOf = (t: number) => new Date(t).toDateString();

/**
 * Where an outing stands on its thread (components/thread.tsx): 0 signed
 * up (plans only), 1 date set (invited, or out of it), 2 going, 3 on the
 * day, 4 how was it. "On the day" runs from the start of the outing's
 * local day (or check-in, if that opens earlier) until check-in closes.
 */
export function outingStage(e: MyEvent, now: number): 1 | 2 | 3 | 4 {
  if (now >= ms(e.feedbackFrom) && now < ms(e.feedbackUntil)) return 4;
  if (e.response !== InviteResponse.CONFIRMED) return 1;
  if (e.status === EventStatus.DONE) return 4;
  const start = ms(e.startsAt);
  const checkInFrom = ms(e.checkInFrom);
  const checkInUntil = ms(e.checkInUntil);
  const onTheDay = (!Number.isNaN(start) && dayOf(start) === dayOf(now)) || now >= checkInFrom;
  if (onTheDay && (Number.isNaN(checkInUntil) || now <= checkInUntil)) return 3;
  return 2;
}
