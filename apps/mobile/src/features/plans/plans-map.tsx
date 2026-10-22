import { useRef } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import type MapViewType from 'react-native-maps';

import { ThemedText } from '@/components/themed-text';
import { useI18n } from '@/i18n/i18n-provider';

import { bounds, initialRegion, type Pin } from './map-data';
import { pinColors } from './pin-color';

// react-native-maps is native code: a development build made before it was
// added doesn't have it, and importing it there throws at load time, which
// would take the whole feed down. Load it here, once, and show "not
// available" instead. (Expo Go and new builds include it.)
let maps: typeof import('react-native-maps') | null = null;
try {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  maps = require('react-native-maps');
} catch {
  maps = null;
}

export type PlansMapProps = {
  pins: Pin[];
  selected: string | null;
  onSelect: (areaId: string) => void;
  /** Where to start when there are no pins: the person's home area. */
  home?: { lat: number; lng: number };
};

/**
 * The map of plans on phones: Google Maps (react-native-maps with the Google
 * provider, as Google's Places policy needs for venue data on a map), one
 * pin per area, coloured by its best plan, with how many plans it has.
 * The web version is plans-map.web.tsx.
 */
export function PlansMap(props: PlansMapProps) {
  const { t } = useI18n();
  if (!maps) {
    return (
      <View style={styles.missing}>
        <ThemedText type="small" themeColor="textSecondary">
          {t('map.unavailable')}
        </ThemedText>
      </View>
    );
  }
  return <NativeMap {...props} />;
}

function NativeMap({ pins, selected, onSelect, home }: PlansMapProps) {
  const { default: MapView, Marker, PROVIDER_GOOGLE } = maps!;
  const map = useRef<MapViewType>(null);
  const box = bounds(pins);
  // Start framed on the pins (or the home area). Without an initial region
  // the map opens on the whole world, and on Android it ignores
  // fitToCoordinates until it has loaded, so framing has to happen here and
  // again in onMapReady, not in an effect right after mounting.
  const region = initialRegion(box, home) ?? undefined;
  const fit = () => {
    if (!box) return;
    map.current?.fitToCoordinates(
      [
        { latitude: box.north, longitude: box.west },
        { latitude: box.south, longitude: box.east },
      ],
      { edgePadding: { top: 40, right: 40, bottom: 40, left: 40 }, animated: false },
    );
  };

  return (
    <MapView
      ref={map}
      provider={PROVIDER_GOOGLE}
      style={StyleSheet.absoluteFill}
      initialRegion={region}
      onMapReady={fit}
      toolbarEnabled={false}
      showsPointsOfInterests={false}>
      {pins.map((p) => {
        const c = pinColors(p.level);
        const on = p.areaId === selected;
        return (
          <Marker
            key={p.areaId}
            coordinate={{ latitude: p.lat, longitude: p.lng }}
            onPress={() => onSelect(p.areaId)}
            // Custom views are re-rendered every frame unless told not to.
            tracksViewChanges={false}>
            <View style={[styles.pin, { backgroundColor: c.background }, on && styles.selected]}>
              <Text style={[styles.count, { color: c.text }]}>{p.plans.length}</Text>
            </View>
          </Marker>
        );
      })}
    </MapView>
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
