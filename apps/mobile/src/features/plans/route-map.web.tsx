import { AdvancedMarker, APIProvider, Map, Polyline } from '@vis.gl/react-google-maps';
import { StyleSheet, Text, View } from 'react-native';

import { Brand } from '@/constants/theme';

import { routeView } from './map-data';
import { decodePolyline } from './polyline';
import type { RouteMapProps } from './route-map';

// As in plans-map.web.tsx.
const apiKey = process.env.EXPO_PUBLIC_GOOGLE_MAPS_WEB_API_KEY;
const mapId = process.env.EXPO_PUBLIC_GOOGLE_MAPS_WEB_MAP_ID ?? 'DEMO_MAP_ID';

/** A plan's map in the browser: numbered stops and the walking route. */
export function RouteMap({ steps, routePolyline }: RouteMapProps) {
  const view = routeView(steps, routePolyline, decodePolyline);
  // No key: the timeline already says everything; skip the map quietly.
  if (!view.stops.length || !apiKey) return null;
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
          {view.path.length > 1 && <Polyline path={view.path} strokeColor={Brand.red} strokeWeight={4} />}
          {view.stops.map((s) => (
            <AdvancedMarker key={s.n} position={{ lat: s.lat, lng: s.lng }}>
              <View style={styles.pin}>
                <Text style={styles.label}>{s.n}</Text>
              </View>
            </AdvancedMarker>
          ))}
        </Map>
      </APIProvider>
    </View>
  );
}

const styles = StyleSheet.create({
  frame: { height: 220, borderRadius: 16, overflow: 'hidden' },
  pin: {
    width: 30,
    height: 30,
    borderRadius: 15,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 2,
    borderColor: '#ffffff',
    backgroundColor: Brand.red,
  },
  label: { fontSize: 13, fontWeight: '700', color: '#ffffff' },
});
