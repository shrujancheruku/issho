import { PlanLevel } from '@issho/api-client';

import { Brand } from '@/constants/theme';
import type { StringKey } from '@/i18n/strings';

/**
 * How a plan suits the person, for the feed's labels (and the map's colours
 * later). The three interest levels reuse the activity tiles' colours from
 * onboarding (features/profile/interest.ts), so "love" looks the same
 * everywhere, and so do their labels: "An activity you love" and "...you like"
 * came out identical in Japanese, while the tiles' "I love it" / "I like it"
 * stay distinct. null background = the theme's neutral element colour.
 */
export const levelStyles: Record<PlanLevel, { background: string | null; text: string | null; label: StringKey }> = {
  [PlanLevel.TOP]: { background: Brand.red, text: '#ffffff', label: 'interest.love' },
  [PlanLevel.GOOD]: { background: Brand.gold, text: Brand.ink, label: 'interest.like' },
  [PlanLevel.MAYBE]: { background: '#F2E6BD', text: Brand.ink, label: 'interest.openToTry' },
  [PlanLevel.LOW]: { background: null, text: null, label: 'plan.level.low' },
  [PlanLevel.NOT_FOR_YOU]: { background: null, text: null, label: 'plan.level.notForYou' },
  [PlanLevel.UNSPECIFIED]: { background: null, text: null, label: 'plan.level.low' },
};
