import { StyleSheet, View } from 'react-native';
import Animated from 'react-native-reanimated';

import { Motion } from '@/constants/motion';
import { useMotion } from '@/hooks/use-motion';
import { useScreenActive } from '@/hooks/use-screen-active';
import { useTheme } from '@/hooks/use-theme';

const breathe = {
  '0%': { transform: [{ scale: 1 }], opacity: 0.7 },
  '70%': { transform: [{ scale: 2.4 }], opacity: 0 },
  '100%': { transform: [{ scale: 2.4 }], opacity: 0 },
};

/**
 * Something happening now (an outing on today): a green dot that breathes
 * while the screen is on show. Use it as the screen's one live thing.
 */
export function LiveDot() {
  const theme = useTheme();
  const { loops } = useMotion();
  const active = useScreenActive();
  return (
    <View style={styles.box} aria-hidden>
      {loops && (
        <Animated.View
          style={[
            styles.dot,
            styles.ring,
            {
              backgroundColor: theme.ok,
              animationName: breathe,
              animationDuration: Motion.breathe,
              animationIterationCount: 'infinite',
              animationTimingFunction: 'ease-out',
              animationPlayState: active ? 'running' : 'paused',
            },
          ]}
        />
      )}
      <View style={[styles.dot, { backgroundColor: theme.ok }]} />
    </View>
  );
}

const styles = StyleSheet.create({
  box: { width: 10, height: 10 },
  dot: { width: 10, height: 10, borderRadius: 5 },
  ring: { position: 'absolute' },
});
