import { StyleSheet, View } from 'react-native';
import Animated, { FadeInDown, FadeOutUp } from 'react-native-reanimated';

import { ThemedText } from '@/components/themed-text';
import { EaseOut, Motion } from '@/constants/motion';

const enter = FadeInDown.duration(Motion.move).easing(EaseOut);
const exit = FadeOutUp.duration(Motion.move).easing(EaseOut);

/**
 * A line that rolls when it changes (a countdown): the old text rises out
 * as the new one rises in. The whole line rolls rather than just its
 * number, since translations put the number in different places. Keyed by
 * value, so only a change animates; with reduced motion Reanimated skips
 * the movement. ThemedText picks the face for the language.
 */
export function RollingText({ value }: { value: string }) {
  return (
    <View style={styles.clip}>
      <Animated.View key={value} entering={enter} exiting={exit}>
        <ThemedText type="subtitle">{value}</ThemedText>
      </Animated.View>
    </View>
  );
}

const styles = StyleSheet.create({
  clip: { overflow: 'hidden' },
});
