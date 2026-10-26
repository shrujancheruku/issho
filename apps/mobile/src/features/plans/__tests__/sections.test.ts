/// <reference types="jest" />
import { PlanLevel, type FeedPlan } from '@issho/api-client';

import { planCategories, planSections } from '../sections';

const plan = (id: string, over: Partial<FeedPlan>) =>
  ({ id, title: id, secondaryTitle: '', areaId: 'koenji', activityId: 'walk', followUpActivityId: '', travelMinutes: 10, friendsInterested: 0, level: PlanLevel.GOOD, ...over }) as FeedPlan;

const names = {
  activity: (id: string) => ({ name: id === 'walk' ? 'Temple walk' : 'Ramen', secondary: '' }),
  area: (id: string) => ({ name: id === 'koenji' ? 'Kōenji' : 'Shibuya', secondary: '高円寺' }),
  category: (id: string) => (id === 'walk' ? 'culture' : 'food'),
};
const all = { query: '', nearby: false, category: null };

describe('planSections', () => {
  it('puts friends first, then levels, and plans in other languages last; each plan once', () => {
    const s = planSections(
      [
        plan('a', { level: PlanLevel.TOP }),
        plan('b', { level: PlanLevel.TOP, friendsInterested: 2 }),
        plan('c', { level: PlanLevel.MAYBE }),
        plan('d', { level: PlanLevel.NOT_FOR_YOU, friendsInterested: 1 }),
        plan('e', { level: PlanLevel.LOW }),
      ],
      all,
      names,
    );
    expect(s.map((x) => [x.key, x.data.map((p) => p.id)])).toEqual([
      ['friends', ['b']],
      ['love', ['a']],
      ['try', ['c']],
      ['other', ['e']],
      ['notForYou', ['d']],
    ]);
  });

  it('searches titles, areas and activities, ignoring accents and case', () => {
    const plans = [plan('a', {}), plan('b', { areaId: 'shibuya', activityId: 'ramen' })];
    expect(planSections(plans, { ...all, query: 'koenji' }, names)[0].data.map((p) => p.id)).toEqual(['a']);
    expect(planSections(plans, { ...all, query: 'RAMEN' }, names)[0].data.map((p) => p.id)).toEqual(['b']);
    expect(planSections(plans, { ...all, query: '高円寺' }, names)[0].data).toHaveLength(2);
  });

  it('filters by trip length and category', () => {
    const plans = [plan('near', {}), plan('far', { travelMinutes: 45 }), plan('noroute', { travelMinutes: -1 }), plan('food', { activityId: 'ramen' })];
    expect(planSections(plans, { ...all, nearby: true }, names)[0].data.map((p) => p.id)).toEqual(['near', 'food']);
    expect(planSections(plans, { ...all, category: 'food' }, names)[0].data.map((p) => p.id)).toEqual(['food']);
  });
});

describe('planCategories', () => {
  it('lists the categories with the most plans first', () => {
    const plans = [plan('a', { activityId: 'ramen' }), plan('b', { activityId: 'ramen' }), plan('c', {})];
    expect(planCategories(plans, names.category)).toEqual(['food', 'culture']);
  });
});
