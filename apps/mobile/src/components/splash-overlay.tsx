import { Image } from 'expo-image';
import * as SplashScreen from 'expo-splash-screen';
import { useState } from 'react';
import { StyleSheet } from 'react-native';
import Animated, { FadeOut } from 'react-native-reanimated';

import { useTheme } from '@/hooks/use-theme';

/**
 * Carries on from the native splash (the mark, 160 wide, on the canvas
 * colour; app.json), then fades to the app once the first screen has laid
 * out, instead of cutting to it.
 */
export function SplashOverlay() {
  const theme = useTheme();
  const [shown, setShown] = useState(true);
  if (!shown) return null;
  return (
    <Animated.View
      exiting={FadeOut.duration(250)}
      pointerEvents="none"
      style={[styles.overlay, { backgroundColor: theme.background }]}
      // The overlay now covers what the native splash did: hide that, then fade this.
      onLayout={() => void SplashScreen.hideAsync().finally(() => setShown(false))}>
      <Image style={styles.mark} source={require('@/assets/images/splash-icon.png')} accessibilityLabel="Issho" />
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  overlay: { ...StyleSheet.absoluteFill, alignItems: 'center', justifyContent: 'center', zIndex: 1000 },
  mark: { width: 160, height: 160 },
});
