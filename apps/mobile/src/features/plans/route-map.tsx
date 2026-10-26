import type { PlanStep } from '@issho/api-client';
import { useEffect, useMemo, useRef, useState } from 'react';
import { StyleSheet, View } from 'react-native';
import type MapViewType from 'react-native-maps';

import { Brand } from '@/constants/theme';

import { initialRegion, routeMinDelta, routeView } from './map-data';
import { BadgeMarker, MapUnavailable, maps } from './native-maps';
import { decodePolyline } from './polyline';
import type { MeasuredPath } from './route-path';
import { useRouteDraw, useWalker } from './use-route-motion';

export type RouteMapProps = { steps: PlanStep[]; routePolyline: string };

// The stops are numbered as in the timeline, on the gold of its thread. The
// route is deep gold: plain gold is too faint on Google's pale map tiles.
const stopColors = { background: Brand.gold, text: Brand.ink };

/**
 * A plan's (or event's) map on phones: Google Maps with a numbered pin at
 * each stop and the walking route between them (Google Routes). Nothing
 * when no stop has a location. The web version is route-map.web.tsx.
 *
 * The route draws itself from the meeting point the first time it's shown
 * this session, each stop popping in as the line reaches it, and then a
 * small gold walker strolls it now and again (use-route-motion.ts).
 */
export function RouteMap({ steps, routePolyline }: RouteMapProps) {
  // Memoised: the route is measured for drawing, once per route.
  const view = useMemo(() => routeView(steps, routePolyline, decodePolyline), [steps, routePolyline]);
  if (!view.stops.length) return null;
  return <View style={styles.frame}>{maps ? <NativeRouteMap view={view} routeKey={routePolyline} /> : <MapUnavailable />}</View>;
}

function NativeRouteMap({ view, routeKey }: { view: ReturnType<typeof routeView>; routeKey: string }) {
  const { default: MapView, Polyline, PROVIDER_GOOGLE } = maps!;
  const route = useRouteDraw(view.path, view.stops, routeKey);
  const map = useRef<MapViewType>(null);
  // As on the map of plans: start framed, and refit once the map has loaded
  // (Android ignores fitting before then).
  const fit = () => {
    if (!view.box) return;
    map.current?.fitToCoordinates(
      [
        { latitude: view.box.north, longitude: view.box.west },
        { latitude: view.box.south, longitude: view.box.east },
      ],
      { edgePadding: { top: 30, right: 30, bottom: 30, left: 30 }, animated: false },
    );
  };
  return (
    <MapView
      ref={map}
      provider={PROVIDER_GOOGLE}
      style={StyleSheet.absoluteFill}
      initialRegion={initialRegion(view.box, undefined, routeMinDelta) ?? undefined}
      onMapReady={fit}
      toolbarEnabled={false}
      showsPointsOfInterests={false}>
      {route.path.length > 1 && (
        <Polyline
          coordinates={route.path.map((p) => ({ latitude: p.lat, longitude: p.lng }))}
          strokeColor={Brand.goldDeep}
          strokeWidth={4}
        />
      )}
      {view.stops.map((s, i) => (
        <BadgeMarker
          key={s.n}
          lat={s.lat}
          lng={s.lng}
          label={String(s.n)}
          colors={stopColors}
          appear={{ id: `${routeKey}:${s.n}`, delay: route.stopDelays[i] }}
        />
      ))}
      <Walker measured={route.measured} enabled={route.done} />
    </MapView>
  );
}

/**
 * The walker: a gold dot with a soft halo, moved along the route. Its own
 * component, so only it re-renders as it moves, not the map and its pins.
 * Remounted for each walk (null between), so its bitmap is drawn fresh and
 * then frozen like the pins'.
 */
function Walker({ measured, enabled }: { measured: MeasuredPath; enabled: boolean }) {
  const at = useWalker(measured, enabled);
  if (!at || !maps) return null;
  return <WalkerMarker lat={at.lat} lng={at.lng} />;
}

function WalkerMarker({ lat, lng }: { lat: number; lng: number }) {
  const { Marker } = maps!;
  const [frozen, setFrozen] = useState(false);
  useEffect(() => {
    const timer = setTimeout(() => setFrozen(true), 500);
    return () => clearTimeout(timer);
  }, []);
  return (
    <Marker coordinate={{ latitude: lat, longitude: lng }} anchor={{ x: 0.5, y: 0.5 }} tracksViewChanges={!frozen} tappable={false} zIndex={1}>
      <View style={styles.halo} collapsable={false}>
        <View style={styles.walker} />
      </View>
    </Marker>
  );
}

const styles = StyleSheet.create({
  // Tall enough for a walk's stops (a kilometre or two, often north to
  // south) to show apart rather than stacked at district scale.
  frame: { height: 320, borderRadius: 16, overflow: 'hidden' },
  halo: { width: 20, height: 20, borderRadius: 10, backgroundColor: 'rgba(222, 169, 44, 0.3)', alignItems: 'center', justifyContent: 'center' },
  walker: { width: 10, height: 10, borderRadius: 5, backgroundColor: Brand.gold },
});
