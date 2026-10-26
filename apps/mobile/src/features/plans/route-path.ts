export type LatLng = { lat: number; lng: number };

/**
 * A path measured for drawing along it: the distance from the start to each
 * point, in degrees of latitude. Longitude is scaled by cos(latitude) so a
 * step east counts the same as a step north; over a walk of a kilometre or
 * two that flat approximation is plenty to pace an animation.
 */
export type MeasuredPath = { points: LatLng[]; at: number[]; total: number };

export function measurePath(points: LatLng[]): MeasuredPath {
  const at = [0];
  const k = points.length ? Math.cos((points[0].lat * Math.PI) / 180) : 1;
  for (let i = 1; i < points.length; i++) {
    const dLat = points[i].lat - points[i - 1].lat;
    const dLng = (points[i].lng - points[i - 1].lng) * k;
    at.push(at[i - 1] + Math.hypot(dLat, dLng));
  }
  return { points, at, total: at[at.length - 1] ?? 0 };
}

/** The point `len` along the path (clamped to its ends). */
export function pointAlong(m: MeasuredPath, len: number): LatLng {
  const { points, at } = m;
  if (len <= 0 || points.length < 2) return points[0];
  for (let i = 1; i < points.length; i++) {
    if (len <= at[i]) {
      const f = at[i] === at[i - 1] ? 1 : (len - at[i - 1]) / (at[i] - at[i - 1]);
      return { lat: points[i - 1].lat + f * (points[i].lat - points[i - 1].lat), lng: points[i - 1].lng + f * (points[i].lng - points[i - 1].lng) };
    }
  }
  return points[points.length - 1];
}

/** The path drawn as far as `len`: the points passed, then where the line's end is now. */
export function pathUpTo(m: MeasuredPath, len: number): LatLng[] {
  if (len >= m.total) return m.points;
  const passed = m.points.filter((_, i) => m.at[i] < len);
  return [...passed, pointAlong(m, len)];
}

/**
 * How far along the path (0 to 1) the line reaches each stop, so a stop can
 * pop in as the line passes through it. Each stop is matched to the path's
 * nearest point at or after the previous stop's, as a walk can come back
 * past an earlier stop.
 */
export function stopFractions(m: MeasuredPath, stops: LatLng[]): number[] {
  if (m.total === 0) return stops.map(() => 0);
  let from = 0;
  return stops.map((s) => {
    let best = from;
    let bestD = Infinity;
    for (let i = from; i < m.points.length; i++) {
      const d = (m.points[i].lat - s.lat) ** 2 + (m.points[i].lng - s.lng) ** 2;
      if (d < bestD) {
        bestD = d;
        best = i;
      }
    }
    from = best;
    return m.at[best] / m.total;
  });
}

/** Ease in and out: the line sets off and arrives gently, like a walk. */
export const easeInOut = (t: number) => (t < 0.5 ? 2 * t * t : 1 - (-2 * t + 2) ** 2 / 2);

/** When (0 to 1) easeInOut reaches `f`: so a stop pops in as the eased line gets to it. */
export function easeInOutInverse(f: number) {
  return f < 0.5 ? Math.sqrt(f / 2) : 1 - Math.sqrt(2 * (1 - f)) / 2;
}
