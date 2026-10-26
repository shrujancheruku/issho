import { Interest } from '@issho/api-client';

import { Brand, type Palette } from '@/constants/theme';
import type { StringKey } from '@/i18n/strings';

type Level = Exclude<Interest, Interest.UNSPECIFIED>;

/** Each interest level's label and how many taps it takes, for the tiles and their legend. */
export const interestLabels: Record<Level, { label: StringKey; tap: StringKey }> = {
  [Interest.OPEN_TO_TRY]: { label: 'interest.openToTry', tap: 'interest.tapOnce' },
  [Interest.LIKE]: { label: 'interest.like', tap: 'interest.tapTwice' },
  [Interest.LOVE]: { label: 'interest.love', tap: 'interest.tapThree' },
};

/**
 * One colour per interest level, used by the tiles and by the explanation
 * above them, so the legend reads like the tiles: pale gold, gold, and
 * vermilion for love (the theme's, lighter on the night ground). Ink text
 * on the golds in both themes.
 */
export function interestColors(level: Level, theme: Palette): { background: string; text: string } {
  switch (level) {
    case Interest.OPEN_TO_TRY:
      return { background: Brand.goldTint, text: Brand.ink };
    case Interest.LIKE:
      return { background: Brand.gold, text: Brand.ink };
    default:
      return { background: theme.love, text: theme.onLove };
  }
}

export const interestLevels = [Interest.OPEN_TO_TRY, Interest.LIKE, Interest.LOVE] as const;

/** The onboarding grid's category order (data/README.md); unknown ones go last. */
export const categoryOrder = ['outdoors', 'sports', 'wellness', 'food', 'culture', 'games', 'creative', 'learning'];

/** A level's colours and words together, as the tiles use them. */
export function interestStyle(level: Interest, theme: Palette) {
  if (level === Interest.UNSPECIFIED) return undefined;
  return { ...interestColors(level, theme), ...interestLabels[level] };
}
