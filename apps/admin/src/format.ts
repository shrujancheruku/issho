import { LeaveReason, ReportReason } from '@issho/api-client';
import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt';

export const reportReasons: Record<ReportReason, string> = {
  [ReportReason.UNSPECIFIED]: 'Unknown',
  [ReportReason.DATING]: 'Here for dating',
  [ReportReason.HARASSMENT]: 'Rude or uncomfortable',
  [ReportReason.UNSAFE]: 'Unsafe',
  [ReportReason.NO_SHOW]: 'No-show',
  [ReportReason.OTHER]: 'Something else',
};

export const leaveReasons: Record<LeaveReason, string> = {
  [LeaveReason.UNSPECIFIED]: 'Unknown',
  [LeaveReason.UNCOMFORTABLE]: 'Made me uncomfortable',
  [LeaveReason.NOT_INTERESTED]: 'Not interested',
  [LeaveReason.INACTIVE]: 'Not enough happens',
  [LeaveReason.TOO_MANY_MESSAGES]: 'Too many messages',
  [LeaveReason.OTHER]: 'Something else',
};

/** "23 Oct 2026, 14:05" in the moderator's own time zone; "" if unset. */
export function formatTime(ts: Timestamp | undefined): string {
  if (!ts) return '';
  return timestampDate(ts).toLocaleString('en-GB', { dateStyle: 'medium', timeStyle: 'short' });
}

const genders: Record<string, string> = { woman: 'women', man: 'men' };

/**
 * One sentence for a pick_gender_lean flag (matching.Lean in the backend):
 * "9 of 10 meet-again picks were women, who were 45% of the people they met
 * (p = 0.002)." Falls back to the raw JSON if the shape is unexpected.
 */
export function leanSummary(detailsJson: string): string {
  try {
    const d = JSON.parse(detailsJson) as {
      gender: string; picked: number; picks: number; met: number; met_share: number; p_value: number;
    };
    if (typeof d.picked !== 'number' || typeof d.picks !== 'number') return detailsJson;
    const who = genders[d.gender] ?? d.gender;
    const share = Math.round(d.met_share * 100);
    return `${d.picked} of ${d.picks} meet-again picks were ${who}, who were ${share}% of the ${d.met} people they met (p = ${formatP(d.p_value)}).`;
  } catch {
    return detailsJson;
  }
}

function formatP(p: number): string {
  return p < 0.001 ? '< 0.001' : p.toFixed(3);
}

const placeholder = /\{[a-zA-Z]+\}/g;

/**
 * The {placeholders} a correction must keep, as the API checks
 * (uistrings.SamePlaceholders): returns those missing or added, empty if fine.
 */
export function placeholderProblems(source: string, text: string): string[] {
  const want = (source.match(placeholder) ?? []).sort();
  const got = (text.match(placeholder) ?? []).sort();
  const problems: string[] = [];
  for (const p of new Set(want)) {
    const diff = want.filter((x) => x === p).length - got.filter((x) => x === p).length;
    if (diff > 0) problems.push(`missing ${p}`);
  }
  for (const p of new Set(got)) {
    if (got.filter((x) => x === p).length > want.filter((x) => x === p).length) problems.push(`unexpected ${p}`);
  }
  return problems;
}

/** The admin SPA's link to one person's view. */
export function personHref(userId: string): string {
  return `#/person/${userId}`;
}

/** The admin SPA's link to one plan's review page. */
export function planHref(planId: string): string {
  return `#/plan/${planId}`;
}

/** The admin SPA's link to one proposed round or scheduled event. */
export function eventHref(eventId: string): string {
  return `#/event/${eventId}`;
}

/** "Sat 12 Jan 2030, 14:00" in the given time zone (the event's city); "" if unset. */
export function localTime(ts: Timestamp | undefined, timeZone: string): string {
  if (!ts) return '';
  return timestampDate(ts).toLocaleString('en-GB', {
    timeZone: timeZone || undefined, weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit',
  });
}

/** A timestamp as a local date (YYYY-MM-DD) and time (HH:MM) in the zone, for date and time inputs. */
export function localParts(ts: Timestamp | undefined, timeZone: string): { date: string; time: string } {
  if (!ts) return { date: '', time: '' };
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: timeZone || undefined, year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
  }).formatToParts(timestampDate(ts));
  const get = (type: string) => parts.find((p) => p.type === type)?.value ?? '';
  return { date: `${get('year')}-${get('month')}-${get('day')}`, time: `${get('hour')}:${get('minute')}` };
}
