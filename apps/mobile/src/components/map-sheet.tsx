import { useEffect, type ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';
import { GestureDetector, useCompetingGestures, usePanGesture, useTapGesture } from 'react-native-gesture-handler';
import Animated, { useAnimatedStyle, useSharedValue, withSpring, withTiming } from 'react-native-reanimated';

import { Motion } from '@/constants/motion';
import { Spacing } from '@/constants/theme';
import { useMotion } from '@/hooks/use-motion';
import { useTheme } from '@/hooks/use-theme';

/** How much of the sheet shows when it's down: the grabber and its title row. */
const SheetPeek = 92;

/**
 * Where the sheet's top edge can rest, from the top of the space it's in:
 * nearly full, half, or down to its title row (above `bottom`, the room
 * a tab bar takes over the content).
 */
function sheetStops(height: number, bottom = 0) {
  return { top: Math.max(24, height * 0.08), mid: height * 0.5, low: height - bottom - SheetPeek };
}

/**
 * A sheet over a map. Dragging its top (the grabber and `handle`) moves it
 * with the finger; let go and it settles on the nearest stop, carried
 * by the fling, with the settle spring (no overshoot). Tapping the top steps
 * it up, and screen readers adjust it like a slider. Only the top drags, so
 * the list inside scrolls as usual.
 *
 * `height` is the space it covers (measured by the parent, which also
 * keeps the map's pins clear of it); `bottom` is how much of that a tab bar
 * covers. When `raiseOn` changes to a value, a
 * lowered sheet comes up to half, so a tapped pin's plans are in view.
 */
export function MapSheet({
  height,
  bottom = 0,
  handle,
  label,
  raiseOn,
  children,
}: {
  height: number;
  bottom?: number;
  handle: ReactNode;
  label: string;
  raiseOn?: string | null;
  children: ReactNode;
}) {
  const theme = useTheme();
  const motion = useMotion();
  const reduced = motion.reduced;
  const { top, mid, low } = sheetStops(height, bottom);
  const y = useSharedValue(mid);
  const from = useSharedValue(mid);

  // Moves the sheet to a stop (from gesture callbacks, on the UI thread).
  const settleTo = (to: number, velocity: number) => {
    'worklet';
    y.set(reduced ? withTiming(to, { duration: Motion.change }) : withSpring(to, { ...Motion.settle, velocity }));
  };

  const pan = usePanGesture({
    onActivate: () => {
      from.set(y.get());
    },
    onUpdate: (e) => {
      y.set(Math.min(low, Math.max(top, from.get() + e.translationY)));
    },
    onDeactivate: (e) => {
      // Where a fling would carry it, then the nearest stop to that.
      const aim = y.get() + e.velocityY * 0.15;
      let to = mid;
      if (Math.abs(aim - top) < Math.abs(aim - to)) to = top;
      if (Math.abs(aim - low) < Math.abs(aim - to)) to = low;
      settleTo(to, e.velocityY);
    },
  });
  const tap = useTapGesture({
    onActivate: () => {
      const at = y.get();
      settleTo(at > mid + 1 ? mid : at > top + 1 ? top : mid, 0);
    },
  });
  const gesture = useCompetingGestures(pan, tap);

  // Keep the sheet at a stop when the space changes (rotation, a resized
  // browser window), and bring it up when something is picked on the map.
  useEffect(() => {
    y.set(Math.min(low, Math.max(top, y.get())));
  }, [top, low, y]);
  useEffect(() => {
    if (raiseOn && y.get() > mid + 1) y.set(motion.settle(mid));
  }, [raiseOn, mid, motion, y]);

  const style = useAnimatedStyle(() => ({ transform: [{ translateY: y.get() }] }));
  const step = (dir: 'increment' | 'decrement') => {
    const at = y.get();
    const order = [low, mid, top];
    const i = order.findIndex((s) => Math.abs(s - at) < 2);
    const next = order[Math.min(order.length - 1, Math.max(0, (i < 0 ? 1 : i) + (dir === 'increment' ? 1 : -1)))];
    y.set(motion.settle(next));
  };

  return (
    <Animated.View
      style={[styles.sheet, { height: height - top, backgroundColor: theme.background, shadowColor: theme.text }, style]}>
      <GestureDetector gesture={gesture}>
        <View
          style={styles.top}
          accessible
          accessibilityRole="adjustable"
          accessibilityLabel={label}
          accessibilityActions={[{ name: 'increment' }, { name: 'decrement' }]}
          onAccessibilityAction={(e) => step(e.nativeEvent.actionName as 'increment' | 'decrement')}>
          <View style={[styles.grabber, { backgroundColor: theme.textSecondary }]} />
          {handle}
        </View>
      </GestureDetector>
      <View style={styles.fill}>{children}</View>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  sheet: {
    position: 'absolute',
    left: 0,
    right: 0,
    top: 0,
    borderTopStartRadius: 20,
    borderTopEndRadius: 20,
    shadowOpacity: 0.12,
    shadowRadius: 12,
    shadowOffset: { width: 0, height: -2 },
    elevation: 8,
  },
  // Tall enough to grab: the grabber plus the title row fill the peek.
  top: { minHeight: SheetPeek, paddingHorizontal: Spacing.three, paddingTop: Spacing.two, paddingBottom: Spacing.three, gap: Spacing.two, cursor: 'grab' },
  grabber: { alignSelf: 'center', width: 36, height: 5, borderRadius: 3, opacity: 0.4 },
});
