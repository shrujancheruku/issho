import type { MyEvent, PlanStep } from '@issho/api-client';

/**
 * The text someone sends a trusted contact about their outing: what, when,
 * where (each stop with its address) and a map link. Only their own outing:
 * never who else is going. Lines come from the UI catalogue, one
 * placeholder each, so a translation that loses one only drops to English
 * for that line.
 */
export function shareText(
  e: Pick<MyEvent, 'title'>,
  when: string,
  steps: Pick<PlanStep, 'venueName' | 'venueAddress' | 'venueMapsUrl'>[],
  area: string,
  t: (key: 'share.intro' | 'share.when' | 'share.where' | 'share.map', params: Record<string, string>) => string,
): string {
  const places = steps.filter((s) => s.venueName).map((s, i) => `${i + 1}. ${s.venueName}${s.venueAddress ? `, ${s.venueAddress}` : ''}`);
  const map = steps.find((s) => s.venueMapsUrl)?.venueMapsUrl;
  return [
    t('share.intro', { title: e.title }),
    t('share.when', { date: when }),
    t('share.where', { place: area }),
    ...places,
    ...(map ? [t('share.map', { url: map })] : []),
  ].join('\n');
}
