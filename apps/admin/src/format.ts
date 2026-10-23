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
