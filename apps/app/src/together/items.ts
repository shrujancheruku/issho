import type { Outing, Plan, Signup } from '@issho/api-client';

import { needsAnswer } from '@/outings/model';
import { DEFAULT_CITY_TIME_ZONE } from '@/plans/city';

/**
 * One row of Together, ordered by time, not by type (docs/spec/app-shape.md): a
 * signup on the day its person will hear, an outing on its date.
 */
export type TogetherItem =
  | { kind: 'signup'; day: string; planId: string; title: string }
  | { kind: 'outing'; day: string; planId: string; title: string; outing: Outing; timeZone: string };

/** The signups as rows on the day each will hear (the round's last day, the city's calendar date), earliest first. */
export function signupItems(signups: Signup[], plans: Plan[]): TogetherItem[] {
  const titles = new Map(plans.map((p) => [p.id, p.title]));
  return signups.map((s) => ({ kind: 'signup' as const, day: s.hearBy, planId: s.planId, title: titles.get(s.planId) ?? '' }));
}

const planOf = (plans: Plan[], id: string) => plans.find((p) => p.id === id);

/** The outings that are not waiting on an answer, on their dates; the ones that are are in "Needs you". */
export function outingItems(outings: Outing[], plans: Plan[], now: Date): TogetherItem[] {
  return outings
    .filter((o) => !needsAnswer(o, now))
    .map((outing) => ({ kind: 'outing' as const, day: outing.date, planId: outing.planId, title: planOf(plans, outing.planId)?.title ?? '', outing, timeZone: planOf(plans, outing.planId)?.timeZone ?? DEFAULT_CITY_TIME_ZONE }));
}

/** The invitations to answer, soonest outing first, each with the plan it is for. */
export function needsYou(outings: Outing[], plans: Plan[], now: Date) {
  return outings
    .filter((o) => needsAnswer(o, now))
    .sort((a, b) => (a.date < b.date ? -1 : a.date > b.date ? 1 : 0))
    .map((outing) => ({ outing, title: planOf(plans, outing.planId)?.title ?? '', timeZone: planOf(plans, outing.planId)?.timeZone ?? DEFAULT_CITY_TIME_ZONE }));
}

/** Rows in time order; ties keep their given order. */
export const byDay = (items: TogetherItem[]) => [...items].sort((a, b) => (a.day < b.day ? -1 : a.day > b.day ? 1 : 0));

/** A row's key: a signup by its plan and day, an outing by its id. */
export const itemKey = (item: TogetherItem) => (item.kind === 'signup' ? `signup-${item.planId}-${item.day}` : `outing-${item.outing.id}`);
