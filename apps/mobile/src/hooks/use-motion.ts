import { useMemo } from 'react';
import { useReducedMotion, withSpring, withTiming } from 'react-native-reanimated';

import { EaseOut, Motion } from '@/constants/motion';

/**
 * Timings for the motion tokens in both modes, so components never check
 * reduced motion themselves. With reduced motion, loops stop, movement
 * becomes a quick fade and springs settle without overshoot; haptics stay.
 *
 * The helpers return animations to assign to shared values from event
 * handlers and effects (the JS thread). The kit is stable while the mode
 * is, so effects can depend on it.
 */
export function useMotion() {
  const reduced = useReducedMotion();
  return useMemo(() => kit(reduced), [reduced]);
}

function kit(reduced: boolean) {
  return {
    reduced,
    /** Whether loops (breathing, live dots) may run at all. */
    loops: !reduced,
    pressScale: reduced ? 1 : Motion.press.scale,
    change: (to: number) => withTiming(to, { duration: Motion.change, easing: EaseOut }),
    move: (to: number) => withTiming(to, { duration: reduced ? Motion.change : Motion.move, easing: EaseOut }),
    /** The thread's fill: one draw per segment, or a quick change. */
    draw: (to: number, segments = 1) =>
      withTiming(to, { duration: reduced ? Motion.change : Motion.draw * Math.max(1, segments), easing: EaseOut }),
    gather: (to: number) => (reduced ? withTiming(to, { duration: Motion.change }) : withSpring(to, Motion.gather)),
    settle: (to: number) => (reduced ? withTiming(to, { duration: Motion.change }) : withSpring(to, Motion.settle)),
  };
}

export type MotionKit = ReturnType<typeof kit>;
