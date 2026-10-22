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

/** The smallest box around the pins, padded, for the map's first view. */
export function bounds(pins: Pin[]) {
  if (!pins.length) return null;
  const lats = pins.map((p) => p.lat);
  const lngs = pins.map((p) => p.lng);
  const pad = 0.01; // about a kilometre, so edge pins aren't cut off
  return {
    north: Math.max(...lats) + pad,
    south: Math.min(...lats) - pad,
    east: Math.max(...lngs) + pad,
    west: Math.min(...lngs) - pad,
  };
}
