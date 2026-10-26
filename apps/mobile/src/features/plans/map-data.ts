import { PlanLevel, type FeedPlan } from '@issho/api-client';

/** One pin on the map of plans: an area with open plans. */
export type Pin = {
  areaId: string;
  lat: number;
  lng: number;
  /** The best level among the area's plans, for the pin's colour. */
  level: PlanLevel;
  plans: FeedPlan[];
};

// Best first. NOT_FOR_YOU is the lowest value, so it only colours a pin
// when every plan there is in a language the person doesn't speak.
const rank = (l: PlanLevel) => (l === PlanLevel.UNSPECIFIED ? -1 : l);

/**
 * Groups the feed's plans by area (plans carry area ids; coordinates come
 * from CatalogService.ListCities). Areas without coordinates are left out.
 * Each pin keeps its plans in feed order, best first.
 */
export function pinsFor(plans: FeedPlan[], areas: Map<string, { lat: number; lng: number }>): Pin[] {
  const pins = new Map<string, Pin>();
  for (const p of plans) {
    const a = areas.get(p.areaId);
    if (!a || (a.lat === 0 && a.lng === 0)) continue;
    const pin = pins.get(p.areaId) ?? { areaId: p.areaId, lat: a.lat, lng: a.lng, level: p.level, plans: [] };
    if (rank(p.level) > rank(pin.level)) pin.level = p.level;
    pin.plans.push(p);
    pins.set(p.areaId, pin);
  }
  return [...pins.values()];
}

/**
 * The smallest box around the points, padded so edge pins aren't cut off:
 * by default about a kilometre (a city's areas); a walking route uses less.
 */
export function bounds(pins: { lat: number; lng: number }[], pad = 0.01) {
  if (!pins.length) return null;
  const lats = pins.map((p) => p.lat);
  const lngs = pins.map((p) => p.lng);
  return {
    north: Math.max(...lats) + pad,
    south: Math.min(...lats) - pad,
    east: Math.max(...lngs) + pad,
    west: Math.min(...lngs) - pad,
  };
}

type Region = { latitude: number; longitude: number; latitudeDelta: number; longitudeDelta: number };

// At least ~5 km across, so one pin (or a tight cluster) isn't shown at
// street level; ~15 km around the home area when there are no pins.
const minDelta = 0.05;
const homeDelta = 0.15;

/**
 * The map's first view: the pins' box, or the person's home area when
 * there are none. null only when neither is known (then the map's own
 * default applies, which is the whole world: avoid it).
 */
export function initialRegion(box: ReturnType<typeof bounds>, home?: { lat: number; lng: number }, min = minDelta): Region | null {
  if (box) {
    return {
      latitude: (box.north + box.south) / 2,
      longitude: (box.east + box.west) / 2,
      latitudeDelta: Math.max(box.north - box.south, min),
      longitudeDelta: Math.max(box.east - box.west, min),
    };
  }
  if (home && !(home.lat === 0 && home.lng === 0)) {
    return { latitude: home.lat, longitude: home.lng, latitudeDelta: homeDelta, longitudeDelta: homeDelta };
  }
  return null;
}

/** A stop on a plan's map: its number in the plan (1, 2 …) and position. */
type Stop = { n: number; lat: number; lng: number };

/**
 * What a plan's map draws: its stops that have a venue location, numbered
 * as in the timeline, and the walking route between them. A broken route
 * string is dropped rather than failing the screen.
 */
export function routeView(steps: { lat: number; lng: number }[], polyline: string, decode: (s: string) => { lat: number; lng: number }[]) {
  // Numbered over the stops with a place, so a walk between them (no
  // place, no pin) doesn't leave a gap: 1, 2 rather than 1, 3.
  const stops: Stop[] = steps
    .filter((s) => !(s.lat === 0 && s.lng === 0))
    .map((s, i) => ({ n: i + 1, lat: s.lat, lng: s.lng }));
  let path: { lat: number; lng: number }[] = [];
  try {
    path = polyline ? decode(polyline) : [];
  } catch {
    path = [];
  }
  return { stops, path, box: bounds([...stops, ...path], routePad) };
}

// A walk between two stops is a kilometre or two: frame it closely (about
// 200 m of padding), but never closer than about 1 km across.
const routePad = 0.002;
export const routeMinDelta = 0.009;
