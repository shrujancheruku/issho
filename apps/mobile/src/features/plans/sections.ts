import { PlanLevel, type FeedPlan } from '@issho/api-client';

import type { StringKey } from '@/i18n/strings';

export type PlanFilter = { query: string; nearby: boolean; category: string | null };

export type PlanSection = {
  key: 'friends' | 'love' | 'like' | 'try' | 'other' | 'notForYou';
  title: StringKey;
  /** The level whose colour marks the section, if any. */
  level?: PlanLevel;
  data: FeedPlan[];
};

/** Trips of up to this long count as near ("Within 30 min"). */
const NearbyMinutes = 30;

type Lookup = {
  activity: (id: string) => { name: string; secondary: string };
  area: (id: string) => { name: string; secondary: string };
  category: (id: string) => string;
};

/** Lower case without accents, so "koenji" finds "Kōenji". */
function fold(s: string) {
  return s.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase();
}

/**
 * Plans as sections, the feed's ranking made visible: plans people you get
 * on with are interested in first, then by how they suit you (love, like,
 * open to try, the rest, and last the ones not in a language you speak).
 * Search and filters work on the device, over names the app already has.
 * Each plan appears once; empty sections are left out.
 */
export function planSections(plans: FeedPlan[], filter: PlanFilter, names: Lookup): PlanSection[] {
  const q = fold(filter.query.trim());
  const matches = (p: FeedPlan) => {
    if (filter.nearby && (p.travelMinutes < 0 || p.travelMinutes > NearbyMinutes)) return false;
    // A plan's follow-up (dinner after bouldering) counts too, when it has one.
    const inCategory = (id: string) => !!id && names.category(id) === filter.category;
    if (filter.category && !inCategory(p.activityId) && !inCategory(p.followUpActivityId)) return false;
    if (!q) return true;
    const area = names.area(p.areaId);
    const act = names.activity(p.activityId);
    return [p.title, p.secondaryTitle, area.name, area.secondary, act.name, act.secondary].some((s) => !!s && fold(s).includes(q));
  };
  const sections: PlanSection[] = [
    { key: 'friends', title: 'plans.friends', data: [] },
    { key: 'love', title: 'interest.love', level: PlanLevel.TOP, data: [] },
    { key: 'like', title: 'interest.like', level: PlanLevel.GOOD, data: [] },
    { key: 'try', title: 'interest.openToTry', level: PlanLevel.MAYBE, data: [] },
    { key: 'other', title: 'plans.other', data: [] },
    { key: 'notForYou', title: 'plan.level.notForYou', data: [] },
  ];
  const by = Object.fromEntries(sections.map((s) => [s.key, s])) as Record<PlanSection['key'], PlanSection>;
  for (const p of plans) {
    if (!matches(p)) continue;
    if (p.level === PlanLevel.NOT_FOR_YOU) by.notForYou.data.push(p);
    else if (p.friendsInterested > 0) by.friends.data.push(p);
    else if (p.level === PlanLevel.TOP) by.love.data.push(p);
    else if (p.level === PlanLevel.GOOD) by.like.data.push(p);
    else if (p.level === PlanLevel.MAYBE) by.try.data.push(p);
    else by.other.data.push(p);
  }
  return sections.filter((s) => s.data.length);
}

/** The categories the plans cover, most plans first, for the filter chips. */
export function planCategories(plans: FeedPlan[], category: (id: string) => string, max = 4): string[] {
  const counts = new Map<string, number>();
  for (const p of plans) {
    if (p.level === PlanLevel.NOT_FOR_YOU) continue;
    const c = category(p.activityId);
    if (c) counts.set(c, (counts.get(c) ?? 0) + 1);
  }
  return [...counts].sort((a, b) => b[1] - a[1]).slice(0, max).map(([c]) => c);
}
