import { useEffect, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Brand } from '@/constants/theme';
import { useMotion } from '@/hooks/use-motion';
import { useI18n } from '@/i18n/i18n-provider';

import { DropIn } from './drop-in';

/**
 * react-native-maps, shared by the phone maps. It's native code: a
 * development build made before it was added doesn't have it, and
 * importing it there throws at load time, which would take a whole screen
 * down. It's loaded here, once; null means "not in this build", and the
 * maps show MapUnavailable instead. (Expo Go and new builds include it.)
 */
export let maps: typeof import('react-native-maps') | null = null;
try {
  // eslint-disable-next-line @typescript-eslint/no-require-imports
  maps = require('react-native-maps');
} catch {
  maps = null;
}

/** Shown in place of a map that can't be drawn here. */
export function MapUnavailable() {
  const { t } = useI18n();
  return (
    <View style={styles.missing}>
      <ThemedText type="small" themeColor="textSecondary">
        {t('map.unavailable')}
      </ThemedText>
    </View>
  );
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
    <View style={[styles.pin, selected && styles.selected, { backgroundColor: colors.background }]}>
      <Text style={[styles.label, { color: colors.text }]}>{label}</Text>
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

const PIN = 30;
const PIN_SELECTED = 38;

const styles = StyleSheet.create({
  missing: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center', padding: 24 },
  // The bitmap's bounds: big enough for the selected pin and its border.
  box: { width: PIN_SELECTED + 6, height: PIN_SELECTED + 6, alignItems: 'center', justifyContent: 'center' },
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
  selected: { width: PIN_SELECTED, height: PIN_SELECTED, borderRadius: PIN_SELECTED / 2, borderColor: Brand.ink },
  label: { fontSize: 13, fontWeight: '700' },
});
