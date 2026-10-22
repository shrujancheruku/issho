import { useEffect, useRef, useState } from 'react';
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
      {pins.map((p) => (
        <PinMarker key={p.areaId} pin={p} on={p.areaId === selected} onPress={() => onSelect(p.areaId)} Marker={Marker} />
      ))}
    </MapView>
  );
}

/**
 * One pin. On Android a custom marker is drawn once into a bitmap the size
 * of the view's first layout, so: a fixed-size transparent box with room
 * for the selected size (a scale transform would be cut off, as it's
 * outside the measured size), and the bitmap redrawn for a moment when the
 * pin appears or its selection changes, then frozen (tracking every frame
 * is slow).
 */
function PinMarker({
  pin,
  on,
  onPress,
  Marker,
}: {
  pin: Pin;
  on: boolean;
  onPress: () => void;
  Marker: (typeof import('react-native-maps'))['Marker'];
}) {
  const c = pinColors(pin.level);
  // What the bitmap shows; a change means it has to be redrawn.
  const look = `${on}|${pin.plans.length}|${pin.level}`;
  const [frozen, setFrozen] = useState<string | null>(null);
  useEffect(() => {
    const timer = setTimeout(() => setFrozen(look), 500);
    return () => clearTimeout(timer);
  }, [look]);
  const tracking = frozen !== look;
  return (
    <Marker
      coordinate={{ latitude: pin.lat, longitude: pin.lng }}
      onPress={onPress}
      anchor={{ x: 0.5, y: 0.5 }}
      tracksViewChanges={tracking}>
      <View style={styles.box} collapsable={false}>
        <View style={[styles.pin, on && styles.selected, { backgroundColor: c.background }]}>
          <Text style={[styles.count, { color: c.text }]}>{pin.plans.length}</Text>
        </View>
      </View>
    </Marker>
  );
}

const PIN = 30;
const PIN_SELECTED = 38;

const styles = StyleSheet.create({
  // The bitmap's bounds: big enough for the selected pin and its border.
  box: { width: PIN_SELECTED + 6, height: PIN_SELECTED + 6, alignItems: 'center', justifyContent: 'center' },
  missing: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center', padding: 24 },
  pin: {
    width: PIN,
    height: PIN,
    borderRadius: PIN / 2,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 2,
    borderColor: '#ffffff',
  },
  // Bigger rather than scaled, so it stays inside the measured box.
  selected: { width: PIN_SELECTED, height: PIN_SELECTED, borderRadius: PIN_SELECTED / 2, borderColor: '#111111' },
  count: { fontSize: 13, fontWeight: '700' },
});
