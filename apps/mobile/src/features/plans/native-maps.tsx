import Constants, { ExecutionEnvironment } from 'expo-constants';
import { useEffect, useState } from 'react';
import { Platform, StyleSheet, Text, View } from 'react-native';

import { useMotion } from '@/hooks/use-motion';

import { DropIn } from './drop-in';
import { PIN_SELECTED, pinStyles } from './map-pins';

/**
 * react-native-maps, shared by the phone maps. It's native code, so a
 * development build has to include it (every current one does; an older one
 * is rebuilt, since importing it there would throw at load time).
 */
// eslint-disable-next-line @typescript-eslint/no-require-imports
export let maps: typeof import('react-native-maps') | null = require('react-native-maps');
// An Android build made without GOOGLE_MAPS_ANDROID_API_KEY crashes natively
// the moment a Google map is drawn (no JS error to catch), a build-config
// mistake, so treat the maps as missing there and show MapUnavailable.
// Expo Go brings its own key.
if (
  Platform.OS === 'android' &&
  Constants.executionEnvironment !== ExecutionEnvironment.StoreClient &&
  !Constants.expoConfig?.extra?.androidMapsKey
) {
  maps = null;
}

/**
 * A round badge pin (a count, or a stop's number). On Android a custom
 * marker is drawn once into a bitmap the size of the view's first layout,
 * so: a fixed-size transparent box with room for the selected size (a
 * scale transform would be cut off, as it's outside the measured size),
 * and the bitmap redrawn for a moment when the pin appears or changes,
 * then frozen (tracking every frame is slow). While it drops in (`appear`),
 * the bitmap keeps being redrawn until it has landed.
 */
export function BadgeMarker({
  lat,
  lng,
  label,
  colors,
  selected = false,
  appear,
  onPress,
}: {
  lat: number;
  lng: number;
  label: string;
  colors: { background: string; text: string };
  selected?: boolean;
  /** Drop in after `delay` ms, the first time this pin (by id) shows this session. */
  appear?: { id: string; delay: number };
  onPress?: () => void;
}) {
  const motion = useMotion();
  // What the bitmap shows; a change means it has to be redrawn.
  const look = `${selected}|${label}|${colors.background}`;
  const [frozen, setFrozen] = useState<string | null>(null);
  // Long enough for the drop's spring to settle (about 600 ms) after its delay.
  const dropping = appear ? appear.delay + 600 : 0;
  useEffect(() => {
    const timer = setTimeout(() => setFrozen(look), 500 + dropping);
    return () => clearTimeout(timer);
  }, [look, dropping]);
  if (!maps) return null;
  const { Marker } = maps;
  const pin = (
    <View style={[pinStyles.pin, selected && pinStyles.selected, { backgroundColor: colors.background }]}>
      <Text style={[pinStyles.label, { color: colors.text }]}>{label}</Text>
    </View>
  );
  return (
    <Marker coordinate={{ latitude: lat, longitude: lng }} onPress={onPress} anchor={{ x: 0.5, y: 0.5 }} tracksViewChanges={frozen !== look}>
      <View style={styles.box} collapsable={false}>
        {appear ? (
          // A short fall: the box only has (44 - 30) / 2 = 7 points spare above the pin.
          <DropIn id={appear.id} delay={motion.reduced ? 0 : appear.delay} rise={6}>
            {pin}
          </DropIn>
        ) : (
          pin
        )}
      </View>
    </Marker>
  );
}

const styles = StyleSheet.create({
  // The bitmap's bounds: big enough for the selected pin and its border.
  box: { width: PIN_SELECTED + 6, height: PIN_SELECTED + 6, alignItems: 'center', justifyContent: 'center' },
});
