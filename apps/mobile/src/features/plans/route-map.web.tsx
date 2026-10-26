import { AdvancedMarker, APIProvider, Map, Polyline } from '@vis.gl/react-google-maps';
import { useMemo } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { Brand } from '@/constants/theme';

import { DropIn } from './drop-in';
import { routeView } from './map-data';
import { decodePolyline } from './polyline';
import type { RouteMapProps } from './route-map';
import type { MeasuredPath } from './route-path';
import { useRouteDraw, useWalker } from './use-route-motion';

// As in plans-map.web.tsx.
const apiKey = process.env.EXPO_PUBLIC_GOOGLE_MAPS_WEB_API_KEY;
const mapId = process.env.EXPO_PUBLIC_GOOGLE_MAPS_WEB_MAP_ID ?? 'DEMO_MAP_ID';

/**
 * A plan's map in the browser: numbered stops and the walking route,
 * drawing itself and walked as on phones (route-map.tsx).
 */
export function RouteMap({ steps, routePolyline }: RouteMapProps) {
  const view = useMemo(() => routeView(steps, routePolyline, decodePolyline), [steps, routePolyline]);
  // No key: the timeline already says everything; skip the map quietly.
  if (!view.stops.length || !apiKey) return null;
  return <WebRouteMap view={view} routeKey={routePolyline} apiKey={apiKey} />;
}

function WebRouteMap({ view, routeKey, apiKey }: { view: ReturnType<typeof routeView>; routeKey: string; apiKey: string }) {
  const route = useRouteDraw(view.path, view.stops, routeKey);
  return (
    <View style={styles.frame}>
      <APIProvider apiKey={apiKey}>
        <Map
          style={StyleSheet.absoluteFill as object}
          mapId={mapId}
          defaultBounds={view.box ? { ...view.box, padding: 30 } : undefined}
          gestureHandling="cooperative"
          disableDefaultUI
          clickableIcons={false}>
          {route.path.length > 1 && <Polyline path={route.path} strokeColor={Brand.goldDeep} strokeWeight={4} />}
          {view.stops.map((s, i) => (
            // Centred on the stop, so the line runs through the middle of each pin.
            <AdvancedMarker key={s.n} position={{ lat: s.lat, lng: s.lng }} anchorLeft="-50%" anchorTop="-50%">
              <DropIn id={`${routeKey}:${s.n}`} delay={route.stopDelays[i]}>
                <View style={styles.pin}>
                  <Text style={styles.label}>{s.n}</Text>
                </View>
              </DropIn>
            </AdvancedMarker>
          ))}
          <Walker measured={route.measured} enabled={route.done} />
        </Map>
      </APIProvider>
    </View>
  );
}

/** The walker, in its own component so only it re-renders as it moves. */
function Walker({ measured, enabled }: { measured: MeasuredPath; enabled: boolean }) {
  const at = useWalker(measured, enabled);
  if (!at) return null;
  return (
    <AdvancedMarker position={at} clickable={false} zIndex={1} anchorLeft="-50%" anchorTop="-50%">
      <View style={styles.halo}>
        <View style={styles.walker} />
      </View>
    </AdvancedMarker>
  );
}

const styles = StyleSheet.create({
  // Tall enough for a walk's stops (a kilometre or two, often north to
  // south) to show apart rather than stacked at district scale.
  frame: { height: 320, borderRadius: 16, overflow: 'hidden' },
  pin: {
    width: 30,
    height: 30,
    borderRadius: 15,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 2,
    borderColor: '#ffffff',
    // As on phones (route-map.tsx): gold stops, deep gold route.
    backgroundColor: Brand.gold,
  },
  label: { fontSize: 13, fontWeight: '700', color: Brand.ink },
  halo: { width: 20, height: 20, borderRadius: 10, backgroundColor: 'rgba(222, 169, 44, 0.3)', alignItems: 'center', justifyContent: 'center' },
  walker: { width: 10, height: 10, borderRadius: 5, backgroundColor: Brand.gold },
});
