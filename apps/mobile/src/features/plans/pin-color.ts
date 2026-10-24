import { PlanLevel } from '@issho/api-client';

import { Brand } from '@/constants/theme';

/**
 * A pin's colours on the map of plans, matching the feed's level labels:
 * love deep gold, like gold, open to try pale gold, the rest grey (lighter when
 * the plans are in a language the person doesn't speak).
 */
export function pinColors(level: PlanLevel): { background: string; text: string } {
  switch (level) {
    case PlanLevel.TOP:
      return { background: Brand.goldDeep, text: '#ffffff' };
    case PlanLevel.GOOD:
      return { background: Brand.gold, text: Brand.ink };
    case PlanLevel.MAYBE:
      return { background: Brand.goldTint, text: Brand.ink };
    case PlanLevel.NOT_FOR_YOU:
      return { background: '#C8CBD0', text: '#5E5E5E' };
    default:
      return { background: '#8A8F98', text: '#ffffff' };
  }
}
