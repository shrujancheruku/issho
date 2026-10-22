/// <reference types="jest" />
import { PlanLevel, type FeedPlan } from '@issho/api-client';

import { bounds, pinsFor } from '../map-data';

const plan = (id: string, areaId: string, level: PlanLevel) => ({ id, areaId, level }) as FeedPlan;
const areas = new Map([
  ['kichijoji', { lat: 35.703, lng: 139.58 }],
  ['koenji', { lat: 35.705, lng: 139.649 }],
  ['nowhere', { lat: 0, lng: 0 }],
]);

it('makes one pin per area, coloured by its best plan, keeping feed order', () => {
  const pins = pinsFor(
    [
      plan('a', 'kichijoji', PlanLevel.GOOD),
      plan('b', 'koenji', PlanLevel.NOT_FOR_YOU),
      plan('c', 'kichijoji', PlanLevel.TOP),
      plan('d', 'koenji', PlanLevel.LOW),
      plan('e', 'nowhere', PlanLevel.TOP),
      plan('f', 'unknown', PlanLevel.TOP),
    ],
    areas,
  );
  expect(pins.map((p) => [p.areaId, p.level, p.plans.map((x) => x.id)])).toEqual([
    ['kichijoji', PlanLevel.TOP, ['a', 'c']],
    ['koenji', PlanLevel.LOW, ['b', 'd']],
  ]);
});

it('greys a pin only when every plan there is not for you', () => {
  expect(pinsFor([plan('a', 'koenji', PlanLevel.NOT_FOR_YOU)], areas)[0].level).toBe(PlanLevel.NOT_FOR_YOU);
});

it('frames all pins with a little padding', () => {
  const b = bounds(pinsFor([plan('a', 'kichijoji', PlanLevel.TOP), plan('b', 'koenji', PlanLevel.TOP)], areas));
  // Floating-point sums: compare to 6 decimal places.
  for (const [k, v] of Object.entries({ north: 35.715, south: 35.693, east: 139.659, west: 139.57 })) {
    expect(b?.[k as keyof typeof b]).toBeCloseTo(v, 6);
  }
  expect(bounds([])).toBeNull();
});
