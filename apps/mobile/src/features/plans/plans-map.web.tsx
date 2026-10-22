import { AdvancedMarker, APIProvider, Map } from '@vis.gl/react-google-maps';
import { StyleSheet, Text, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { useI18n } from '@/i18n/i18n-provider';

import { bounds } from './map-data';
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
 * pins as on phones.
 */
export function PlansMap({ pins, selected, onSelect, home }: PlansMapProps) {
  const { t } = useI18n();
  const box = bounds(pins);
  if (!apiKey) {
    return (
      <View style={styles.missing}>
        <ThemedText type="small" themeColor="textSecondary">
          {t('map.unavailable')}
        </ThemedText>
      </View>
    );
  }
  return (
    <APIProvider apiKey={apiKey}>
      <Map
        style={StyleSheet.absoluteFill as object}
        mapId={mapId}
        // The pins' box; with none, the home area (never the whole world).
        defaultBounds={box ?? undefined}
        defaultCenter={box || !home ? undefined : { lat: home.lat, lng: home.lng }}
        defaultZoom={box || !home ? undefined : 12}
        gestureHandling="greedy"
        disableDefaultUI
        clickableIcons={false}>
        {pins.map((p) => {
          const c = pinColors(p.level);
          const on = p.areaId === selected;
          return (
            <AdvancedMarker key={p.areaId} position={{ lat: p.lat, lng: p.lng }} onClick={() => onSelect(p.areaId)}>
              <View style={[styles.pin, { backgroundColor: c.background }, on && styles.selected]}>
                <Text style={[styles.count, { color: c.text }]}>{p.plans.length}</Text>
              </View>
            </AdvancedMarker>
          );
        })}
      </Map>
    </APIProvider>
  );
}

const styles = StyleSheet.create({
  missing: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center', padding: 24 },
  pin: {
    minWidth: 30,
    height: 30,
    borderRadius: 15,
    paddingHorizontal: 8,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 2,
    borderColor: '#ffffff',
  },
  selected: { borderColor: '#111111', transform: [{ scale: 1.2 }] },
  count: { fontSize: 13, fontWeight: '700' },
});
