import { OutingState, SeatState, type Outing } from '@issho/api-client';

import type { Language, StringKey } from '@/i18n/strings';
import { languageName } from '@/profile/languages';

/** The outing's state in the spec's words (docs/spec/outings.md, "What an outing is"). */
export const stateKey = (state: OutingState): StringKey => {
  switch (state) {
    case OutingState.PROPOSED:
      return 'outing.state.proposed';
    case OutingState.SET:
      return 'outing.state.set';
    case OutingState.OVER:
      return 'outing.state.over';
    case OutingState.NOT_SET:
      return 'outing.state.notSet';
    case OutingState.CANCELLED:
      return 'outing.state.cancelled';
    default:
      return 'outing.state.offered';
  }
};

/**
 * The one line saying where the person stands in an outing: how the outing
 * ended if it did, else their own seat. A seat still invited here is one whose
 * 48 hours went by with no answer.
 */
export function statusKey(o: Outing): StringKey {
  if (o.outingState === OutingState.NOT_SET) return 'outing.status.outingNotSet';
  if (o.outingState === OutingState.CANCELLED) return 'outing.status.outingCancelled';
  if (o.outingState === OutingState.OVER) return 'outing.status.outingOver';
  switch (o.seatState) {
    case SeatState.GOING:
      return 'outing.status.going';
    case SeatState.ACCEPTED:
      return 'outing.status.accepted';
    case SeatState.DECLINED:
      return 'outing.status.declined';
    case SeatState.FULL:
      return 'outing.status.full';
    case SeatState.CANCELLED:
      return 'outing.status.cancelled';
    case SeatState.LATE_CANCEL:
      return 'outing.status.lateCancel';
    case SeatState.REMOVED:
      return 'outing.status.removed';
    default:
      return 'outing.status.unanswered';
  }
}

/** The group's shared languages by name in the display language. */
export const languagesText = (o: Outing, lang: Language) => o.languages.map((code) => languageName(code, lang)).join(', ');
