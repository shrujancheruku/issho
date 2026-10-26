import { useEffect, type ReactNode } from 'react';
import Animated, { interpolate, useAnimatedStyle, useSharedValue, withDelay } from 'react-native-reanimated';

import { Motion } from '@/constants/motion';
import { useMotion } from '@/hooks/use-motion';

// Pins that have already dropped this session. Markers remount when the map
// is shown again (list → map) and replaying the drop each time would be
// noise; panning never remounts them. A pin that's new since (a new area
// in the feed) still drops: that's a change worth showing.
const dropped = new Set<string>();

/**
 * When the i-th pin drops: 45 ms after the one before. Capped so a busy
 * map is done within about half a second rather than trickling on.
 */
export const dropDelay = (i: number) => Math.min(i, 12) * Motion.pinStagger;

/**
 * A map pin dropping into place: down a little, growing from 0.6 with the
 * gather spring, after `delay` ms (so pins land one after another, or as a
 * route reaches them). With reduced motion it fades in instead.
 *
 * `rise` is how far it falls. Inside a phone marker, which is drawn into a
 * fixed-size bitmap, keep it within the marker's spare room or the pin is
 * cut off as it falls.
 */
export function DropIn({ id, delay = 0, rise = 14, children }: { id: string; delay?: number; rise?: number; children: ReactNode }) {
  const motion = useMotion();
  const first = !dropped.has(id);
  const p = useSharedValue(first ? 0 : 1);
  useEffect(() => {
    if (!first) return;
    dropped.add(id);
    p.set(withDelay(delay, motion.gather(1)));
  }, [first, id, delay, motion, p]);
  const reduced = motion.reduced;
  const style = useAnimatedStyle(() => {
    const v = p.get();
    const opacity = interpolate(v, [0, 0.5], [0, 1], 'clamp');
    if (reduced) return { opacity };
    return { opacity, transform: [{ translateY: (v - 1) * rise }, { scale: 0.6 + 0.4 * v }] };
  });
  return <Animated.View style={style}>{children}</Animated.View>;
}
