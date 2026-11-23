import type { Signup, Tick } from '@issho/api-client';

import { cityDateKey } from '@/lib/format';

/** The parts of the day a tick can name, in order (proto/issho/v1/signup.proto). */
export const PARTS_OF_DAY = ['morning', 'afternoon', 'evening'] as const;

/** A signup's ticks start this many days ahead on the city's clock (policy.MinTickLeadDays on the server, which also refuses sooner ones). */
const LEAD_DAYS = 3;
/** And end this many days after the round's last day (policy.RoundLength: a round, 7 days). */
const TRAIL_DAYS = 7;

export const tickKey = (date: string, part: string) => `${date}|${part}`;

/** Today's calendar date on the city's clock, "YYYY-MM-DD". */
export const cityToday = (now: Date, timeZone: string) => cityDateKey(now, timeZone);

/** `date` ("YYYY-MM-DD") moved by whole days. A calendar date has no zone, so UTC arithmetic is exact. */
export function addDays(date: string, days: number): string {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

/**
 * The dates a signup offers, exactly the ones SignUp accepts: from 3 days ahead
 * on the city's clock to 7 days after the round's last day (docs/spec/matching.md,
 * "Times"). `roundEnds` is the plan's `round_ends`. A round never ends before
 * today, so a missing one or one already past (a plan page cached before the
 * round turned) means today: a shorter window the server still accepts.
 */
export function signupDates(now: Date, timeZone: string, roundEnds: string): string[] {
  const today = cityToday(now, timeZone);
  const last = roundEnds > today ? roundEnds : today;
  const first = addDays(today, LEAD_DAYS);
  const dates: string[] = [];
  for (let d = first, end = addDays(last, TRAIL_DAYS); d <= end; d = addDays(d, 1)) dates.push(d);
  return dates;
}

/**
 * The person's last signup: the newest by when it was made or last changed
 * (`signedAt`), whatever plan it is on and whenever its round ends. The list
 * comes earliest hear-by first, which is not the order they signed up in.
 */
export function lastSignup(signups: Signup[]): Signup | undefined {
  const when = (s: Signup) => ({ seconds: s.signedAt?.seconds ?? BigInt(0), nanos: s.signedAt?.nanos ?? 0 });
  return signups.reduce<Signup | undefined>((best, s) => {
    if (!best) return s;
    const a = when(s);
    const b = when(best);
    return a.seconds > b.seconds || (a.seconds === b.seconds && a.nanos > b.nanos) ? s : best;
  }, undefined);
}

/**
 * The ticks to start from: the person's last signup's, where they fall in the
 * new window on a part the plan allows (docs/spec/matching.md: "prefilled from
 * the person's last signup"; the server does not prefill).
 */
export function prefill(last: Pick<Tick, 'date' | 'part'>[], dates: string[], allowedParts: string[]): Set<string> {
  const offered = new Set(dates);
  return new Set(last.filter((t) => offered.has(t.date) && allowedParts.includes(t.part)).map((t) => tickKey(t.date, t.part)));
}
