/// <reference types="jest" />
import { easeInOut, easeInOutInverse, measurePath, pathUpTo, pointAlong, stopFractions } from '../route-path';

// On the equator a degree of longitude counts the same as one of latitude.
const path = measurePath([
  { lat: 0, lng: 0 },
  { lat: 0, lng: 1 },
  { lat: 1, lng: 1 },
]);

describe('route path', () => {
  it('measures the distance to each point', () => {
    expect(path.at).toEqual([0, 1, 2]);
    expect(path.total).toBe(2);
  });

  it('finds points along the path, clamped to its ends', () => {
    expect(pointAlong(path, 0.5)).toEqual({ lat: 0, lng: 0.5 });
    expect(pointAlong(path, 1.5)).toEqual({ lat: 0.5, lng: 1 });
    expect(pointAlong(path, -1)).toEqual({ lat: 0, lng: 0 });
    expect(pointAlong(path, 9)).toEqual({ lat: 1, lng: 1 });
  });

  it('draws the path up to a length, ending where the line has got to', () => {
    expect(pathUpTo(path, 1.5)).toEqual([
      { lat: 0, lng: 0 },
      { lat: 0, lng: 1 },
      { lat: 0.5, lng: 1 },
    ]);
    expect(pathUpTo(path, 2)).toBe(path.points);
  });

  it('places each stop where the line passes it, in order', () => {
    expect(stopFractions(path, [{ lat: 0, lng: 0 }, { lat: 0.01, lng: 1 }, { lat: 1, lng: 1 }])).toEqual([0, 0.5, 1]);
    // A stop the walk comes back past is matched after the stop before it.
    const back = measurePath([
      { lat: 0, lng: 0 },
      { lat: 0, lng: 1 },
      { lat: 0, lng: 0 },
    ]);
    expect(stopFractions(back, [{ lat: 0, lng: 1 }, { lat: 0, lng: 0 }])).toEqual([0.5, 1]);
  });

  it('eases in and out', () => {
    expect(easeInOut(0)).toBe(0);
    expect(easeInOut(0.5)).toBe(0.5);
    expect(easeInOut(1)).toBe(1);
    for (const f of [0, 0.2, 0.5, 0.8, 1]) expect(easeInOut(easeInOutInverse(f))).toBeCloseTo(f);
  });
});
