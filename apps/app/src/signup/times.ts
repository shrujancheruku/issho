import type { Tick } from '@issho/api-client';

/** The parts of the day a tick can name, in order (proto/issho/v1/signup.proto). */
export const PARTS_OF_DAY = ['morning', 'afternoon', 'evening'] as const;

/** A signup's ticks start this many days ahead on the city's clock (policy.MinTickLeadDays on the server, which also refuses sooner ones). */
const LEAD_DAYS = 3;
/**
 * How many dates are offered. The server accepts ticks up to 7 days past the
 * round's last day, and a round's last day is never before today, so today + 7
 * is always inside: five dates from today + 3 never meet TICK_TOO_FAR.
 */
const OFFERED_DAYS = 5;

export const tickKey = (date: string, part: string) => `${date}|${part}`;

/** Today's calendar date on the city's clock, "YYYY-MM-DD". */
export function cityToday(now: Date, timeZone: string): string {
  // en-CA writes year-month-day; the zone is what makes it the city's day.
  return new Intl.DateTimeFormat('en-CA', { timeZone, year: 'numeric', month: '2-digit', day: '2-digit' }).format(now);
}

/** `date` ("YYYY-MM-DD") moved by whole days. A calendar date has no zone, so UTC arithmetic is exact. */
export function addDays(date: string, days: number): string {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + days);
  return d.toISOString().slice(0, 10);
}

/** The dates a signup offers: from 3 days ahead on the city's clock. */
export function signupDates(now: Date, timeZone: string): string[] {
  const today = cityToday(now, timeZone);
  return Array.from({ length: OFFERED_DAYS }, (_, i) => addDays(today, LEAD_DAYS + i));
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
