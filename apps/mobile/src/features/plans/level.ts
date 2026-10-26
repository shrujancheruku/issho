import { PlanLevel } from '@issho/api-client';

import { Brand, type Palette } from '@/constants/theme';
import type { StringKey } from '@/i18n/strings';

/**
 * How a plan suits the person, for labels, Plans' sections and map pins.
 * The three interest levels reuse the activity tiles' colours and labels
 * (features/profile/interest.ts), so "Love it" looks and reads the same
 * everywhere: "An activity you love" and "...you like" came out identical
 * in Japanese, while the tiles' labels stay distinct.
 */
export const levelLabels: Record<PlanLevel, StringKey> = {
  [PlanLevel.TOP]: 'interest.love',
  [PlanLevel.GOOD]: 'interest.like',
  [PlanLevel.MAYBE]: 'interest.openToTry',
  [PlanLevel.LOW]: 'plan.level.low',
  [PlanLevel.NOT_FOR_YOU]: 'plan.level.notForYou',
  [PlanLevel.UNSPECIFIED]: 'plan.level.low',
};

/** A level's fill and text: vermilion love, gold like, pale gold open to try; null for the rest. */
export function levelColors(level: PlanLevel, theme: Palette): { background: string; text: string } | null {
  switch (level) {
    case PlanLevel.TOP:
      return { background: theme.love, text: theme.onLove };
    case PlanLevel.GOOD:
      return { background: Brand.gold, text: Brand.ink };
    case PlanLevel.MAYBE:
      return { background: Brand.goldTint, text: Brand.ink };
    default:
      return null;
  }
}
