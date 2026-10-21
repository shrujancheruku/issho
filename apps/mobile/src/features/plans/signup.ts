import { TimeBlock } from '@issho/api-client';

import { parseSlotKey, slotKey } from '@/lib/slots';

/** A part of a day blocked out, as in the API's UnavailableTime. */
export type Unavailable = { date: string; block: TimeBlock };

/** The fields of the API's Signup that the plan screen uses. */
export type SavedSignup = { pattern: { weekday: number; block: TimeBlock }[]; unavailable: Unavailable[] };

/**
 * A signup as the plan screen edits it: weekly slots (slotKey) and whole
 * dates blocked out ("2030-01-09"). Converted to the API's Signup at the
 * edges so the rules here can be unit-tested.
 */
export type Times = {
  pattern: string[];
  blocked: string[];
  /** Parts of days blocked out some other way, kept as they are on save. */
  keep: Unavailable[];
};

/** The dates of a plan's window: days dates from start ("YYYY-MM-DD"). */
export function windowDates(start: string, days: number): string[] {
  const [y, m, d] = start.split('-').map(Number);
  return Array.from({ length: days }, (_, i) => new Date(Date.UTC(y, m - 1, d + i)).toISOString().slice(0, 10));
}

/**
 * The times to show: the saved signup's, or the profile's usual
 * availability for someone not signed up yet (or who kept it).
 */
export function initialTimes(signup: SavedSignup | undefined, usual: string[]): Times {
  const pattern = signup?.pattern.length ? signup.pattern.map((s) => slotKey(s.weekday, s.block)) : usual;
  const unavailable = signup?.unavailable ?? [];
  return {
    pattern: [...pattern],
    blocked: unavailable.filter((u) => u.block === TimeBlock.UNSPECIFIED).map((u) => u.date),
    keep: unavailable.filter((u) => u.block !== TimeBlock.UNSPECIFIED),
  };
}

/**
 * The signup to send. A pattern equal to the usual availability is sent
 * empty, meaning "my profile's availability", so later profile changes
 * still apply to the plan.
 */
export function toSignup(t: Times, usual: string[]) {
  const same = t.pattern.length === usual.length && t.pattern.every((k) => usual.includes(k));
  return {
    pattern: same ? [] : t.pattern.map(parseSlotKey),
    unavailable: [
      ...[...t.blocked].sort().map((date) => ({ date, block: TimeBlock.UNSPECIFIED })),
      ...t.keep.filter((u) => !t.blocked.includes(u.date)).map(({ date, block }) => ({ date, block })),
    ],
  };
}

export function toggle(list: string[], value: string): string[] {
  return list.includes(value) ? list.filter((v) => v !== value) : [...list, value];
}
