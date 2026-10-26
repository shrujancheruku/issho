import type { ReactNode } from 'react';
import { Pressable, type PressableProps, type StyleProp, type ViewStyle } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withTiming } from 'react-native-reanimated';

import { EaseOut, Motion } from '@/constants/motion';
import { useMotion } from '@/hooks/use-motion';

const AnimatedPressable = Animated.createAnimatedComponent(Pressable);

/**
 * A Pressable that presses in (the "press" token) on touch down. The scale
 * runs on the UI thread, so it responds even while JS is busy. Rows that
 * span the screen use a pressed background instead (ListRow): scaling a
 * full-width row reads as the whole list shifting.
 */
export function PressableScale({
  style,
  children,
  onPressIn,
  onPressOut,
  ...props
}: Omit<PressableProps, 'style' | 'children'> & { style?: StyleProp<ViewStyle>; children?: ReactNode }) {
  const { pressScale } = useMotion();
  const scale = useSharedValue(1);
  const animated = useAnimatedStyle(() => ({ transform: [{ scale: scale.value }] }));
  return (
    <AnimatedPressable
      {...props}
      onPressIn={(e) => {
        scale.set(withTiming(pressScale, { duration: Motion.press.duration, easing: EaseOut }));
        onPressIn?.(e);
      }}
      onPressOut={(e) => {
        scale.set(withTiming(1, { duration: Motion.change, easing: EaseOut }));
        onPressOut?.(e);
      }}
      style={[style, animated]}>
      {children}
    </AnimatedPressable>
  );
}
