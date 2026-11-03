import { english, type StringKey } from '@/i18n/strings';
import type { useI18n } from '@/i18n/i18n-provider';

/** Short text side by side, as everywhere (docs/architecture.md#languages). */
export function pairText(n: { name: string; secondary: string }) {
  return n.secondary ? `${n.name} · ${n.secondary}` : n.name;
}

/** How far a plan is from the person's home area. */
export function distanceText(t: ReturnType<typeof useI18n>['t'], minutes: number) {
  if (minutes < 0) return t('plan.noRoute');
  return minutes === 0 ? t('plan.nearby') : t('plan.minutes', { minutes });
}

/** "Someone you get on with is interested", or how many. */
export function friendsText(t: ReturnType<typeof useI18n>['t'], count: number) {
  return t('plan.friends', { count });
}

/** A category's name, in the shown language. */
export function categoryLabel(t: (k: StringKey) => string, category: string) {
  const key = `category.${category}`;
  // A category added on the server before this build knows it shows its id.
  return key in english ? t(key as StringKey) : category;
}
