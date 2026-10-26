import { useEffect } from 'react';
import Animated, { useAnimatedProps, useSharedValue, withDelay, withTiming } from 'react-native-reanimated';
import Svg, { Path } from 'react-native-svg';

import { EaseOut, Motion } from '@/constants/motion';
import { useMotion } from '@/hooks/use-motion';

const AnimatedPath = Animated.createAnimatedComponent(Path);
// Length of the tick's path (M4 10.5l4 4 8-9), rounded up.
const LENGTH = 24;

/**
 * A tick that draws itself when `drawn` turns true while on screen (a
 * button becoming its result). Mounted already drawn, it just shows.
 */
export function Tick({ color, size = 18, drawn = true, animate = true }: { color: string; size?: number; drawn?: boolean; animate?: boolean }) {
  const { reduced } = useMotion();
  const offset = useSharedValue(drawn && !animate ? 0 : LENGTH);
  useEffect(() => {
    const to = drawn ? 0 : LENGTH;
    offset.value = reduced || !animate ? to : withDelay(120, withTiming(to, { duration: Motion.move + 40, easing: EaseOut }));
  }, [drawn, reduced, animate, offset]);
  const props = useAnimatedProps(() => ({ strokeDashoffset: offset.value }));
  return (
    <Svg width={size} height={size} viewBox="0 0 20 20" fill="none" aria-hidden>
      <AnimatedPath
        d="M4 10.5l4 4 8-9"
        stroke={color}
        strokeWidth={2.6}
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeDasharray={LENGTH}
        animatedProps={props}
      />
    </Svg>
  );
}
