import type { Plan, Signup } from '@issho/api-client';

/**
 * One row of Together, ordered by time, not by type (docs/spec/app-shape.md).
 * Only signups exist so far: outings (#619) add their own `kind` and are
 * merged in by `day`, so the list's shape stays open.
 */
export type TogetherItem = { kind: 'signup'; day: string; planId: string; title: string };

/** The signups as rows on the day each will hear (the round's last day, the city's calendar date), earliest first. */
export function signupItems(signups: Signup[], plans: Plan[]): TogetherItem[] {
  const titles = new Map(plans.map((p) => [p.id, p.title]));
  return signups.map((s) => ({ kind: 'signup' as const, day: s.hearBy, planId: s.planId, title: titles.get(s.planId) ?? '' }));
}

/** Rows in time order; ties keep their given order. */
export const byDay = (items: TogetherItem[]) => [...items].sort((a, b) => (a.day < b.day ? -1 : a.day > b.day ? 1 : 0));
