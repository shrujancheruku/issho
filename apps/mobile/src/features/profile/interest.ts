import { Interest } from '@issho/api-client';

import { Brand } from '@/constants/theme';
import type { StringKey } from '@/i18n/strings';

/**
 * One colour per interest level, used by the tiles and by the explanation
 * above them, so the legend reads like the tiles. Text colours are fixed (not
 * the theme's) so each pair stays legible in light and dark mode.
 */
export const interestStyles: Record<Exclude<Interest, Interest.UNSPECIFIED>, { background: string; text: string; label: StringKey; tap: StringKey }> = {
  [Interest.OPEN_TO_TRY]: { background: '#F2E6BD', text: Brand.ink, label: 'interest.openToTry', tap: 'interest.tapOnce' },
  [Interest.LIKE]: { background: Brand.gold, text: Brand.ink, label: 'interest.like', tap: 'interest.tapTwice' },
  [Interest.LOVE]: { background: Brand.red, text: '#ffffff', label: 'interest.love', tap: 'interest.tapThree' },
};

export const interestLevels = [Interest.OPEN_TO_TRY, Interest.LIKE, Interest.LOVE] as const;

/** The onboarding grid's category order (data/README.md); unknown ones go last. */
export const categoryOrder = ['outdoors', 'sports', 'wellness', 'food', 'culture', 'games', 'creative', 'learning'];
