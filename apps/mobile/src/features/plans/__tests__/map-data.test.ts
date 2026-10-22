/// <reference types="jest" />
import { PlanLevel, type FeedPlan } from '@issho/api-client';

import { bounds, initialRegion, pinsFor } from '../map-data';

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

describe('initialRegion', () => {
  it('centres on the pins, at least about 5 km across', () => {
    const r = initialRegion({ north: 35.715, south: 35.693, east: 139.659, west: 139.57 });
    expect(r?.latitude).toBeCloseTo(35.704, 6);
    expect(r?.longitude).toBeCloseTo(139.6145, 6);
    expect(r?.latitudeDelta).toBeCloseTo(0.05, 6); // 0.022 tall, widened to the minimum
    expect(r?.longitudeDelta).toBeCloseTo(0.089, 6);
  });

  it('falls back to the home area, then to nothing (never the whole world)', () => {
    expect(initialRegion(null, { lat: 35.703, lng: 139.58 })).toEqual({ latitude: 35.703, longitude: 139.58, latitudeDelta: 0.15, longitudeDelta: 0.15 });
    expect(initialRegion(null, { lat: 0, lng: 0 })).toBeNull();
    expect(initialRegion(null)).toBeNull();
  });
});
