import type { PlanStep } from '@issho/api-client';
import { useRef } from 'react';
import { StyleSheet, View } from 'react-native';
import type MapViewType from 'react-native-maps';

import { Brand } from '@/constants/theme';

import { initialRegion, routeMinDelta, routeView } from './map-data';
import { BadgeMarker, MapUnavailable, maps } from './native-maps';
import { decodePolyline } from './polyline';

export type RouteMapProps = { steps: PlanStep[]; routePolyline: string };

// The stops are numbered as in the timeline, in the brand red.
const stopColors = { background: Brand.red, text: '#ffffff' };

/**
 * A plan's (or event's) map on phones: Google Maps with a numbered pin at
 * each stop and the walking route between them (Google Routes). Nothing
 * when no stop has a location. The web version is route-map.web.tsx.
 */
export function RouteMap({ steps, routePolyline }: RouteMapProps) {
  const view = routeView(steps, routePolyline, decodePolyline);
  if (!view.stops.length) return null;
  return <View style={styles.frame}>{maps ? <NativeRouteMap view={view} /> : <MapUnavailable />}</View>;
}

function NativeRouteMap({ view }: { view: ReturnType<typeof routeView> }) {
  const { default: MapView, Polyline, PROVIDER_GOOGLE } = maps!;
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
      {view.path.length > 1 && (
        <Polyline
          coordinates={view.path.map((p) => ({ latitude: p.lat, longitude: p.lng }))}
          strokeColor={Brand.red}
          strokeWidth={4}
        />
      )}
      {view.stops.map((s) => (
        <BadgeMarker key={s.n} lat={s.lat} lng={s.lng} label={String(s.n)} colors={stopColors} />
      ))}
    </MapView>
  );
}

const styles = StyleSheet.create({
  frame: { height: 220, borderRadius: 16, overflow: 'hidden' },
});
