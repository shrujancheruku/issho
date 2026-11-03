import { AdvancedMarker, APIProvider, Map } from '@vis.gl/react-google-maps';
import { StyleSheet, Text, View } from 'react-native';

import { useTheme } from '@/hooks/use-theme';

import { dropDelay, DropIn } from './drop-in';
import { bounds } from './map-data';
import { MapUnavailable } from './map-unavailable';
import { pinStyles } from './map-pins';
import { pinColors } from './pin-color';
import type { PlansMapProps } from './plans-map';

// A browser key restricted to the site's addresses (it's in the page, like
// every web Maps key). Markers drawn as HTML need a map id; Google's
// DEMO_MAP_ID works for development.
const apiKey = process.env.EXPO_PUBLIC_GOOGLE_MAPS_WEB_API_KEY;
const mapId = process.env.EXPO_PUBLIC_GOOGLE_MAPS_WEB_MAP_ID ?? 'DEMO_MAP_ID';

/**
 * The map of plans in the browser: Google's Maps JavaScript API through
 * @vis.gl/react-google-maps (react-native-maps has no web version). Same
 * pins as on phones, dropping in the same way.
 */
export function PlansMap({ pins, selected, onSelect, home, bottomInset = 0 }: PlansMapProps) {
  const theme = useTheme();
  const box = bounds(pins);
  if (!apiKey) {
    return <MapUnavailable />;
  }
  return (
    <APIProvider apiKey={apiKey}>
      <Map
        style={StyleSheet.absoluteFill as object}
        mapId={mapId}
        // The pins' box; with none, the home area (never the whole world).
        defaultBounds={box ? { ...box, padding: { top: 40, right: 40, bottom: 40 + bottomInset, left: 40 } } : undefined}
        defaultCenter={box || !home ? undefined : { lat: home.lat, lng: home.lng }}
        defaultZoom={box || !home ? undefined : 12}
        gestureHandling="greedy"
        disableDefaultUI
        clickableIcons={false}>
        {pins.map((p, i) => {
          const c = pinColors(p.level, theme);
          const on = p.areaId === selected;
          return (
            <AdvancedMarker key={p.areaId} position={{ lat: p.lat, lng: p.lng }} onClick={() => onSelect(p.areaId)}>
              <DropIn id={`area:${p.areaId}`} delay={dropDelay(i)}>
                <View style={[pinStyles.countPin, { backgroundColor: c.background }, on && pinStyles.selectedWeb]}>
                  <Text style={[pinStyles.label, { color: c.text }]}>{p.plans.length}</Text>
                </View>
              </DropIn>
            </AdvancedMarker>
          );
        })}
      </Map>
    </APIProvider>
  );
}
