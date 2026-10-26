import { PlanLevel } from '@issho/api-client';

import type { Palette } from '@/constants/theme';

import { levelColors } from './level';

/**
 * A pin's colours on the map of plans, matching the level labels: love
 * vermilion, like gold, open to try pale gold, the rest grey (lighter when
 * the plans are in a language the person doesn't speak).
 */
export function pinColors(level: PlanLevel, theme: Palette): { background: string; text: string } {
  const c = levelColors(level, theme);
  if (c) return c;
  return level === PlanLevel.NOT_FOR_YOU
    ? { background: theme.pinLow, text: theme.pinLowText }
    : { background: theme.pinNone, text: '#ffffff' };
}
