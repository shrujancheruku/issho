import { useEffect, useState } from 'react';
import { StyleSheet, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue } from 'react-native-reanimated';

import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { Tick } from '@/components/tick';
import { Spacing } from '@/constants/theme';
import { useMotion } from '@/hooks/use-motion';
import { useTheme } from '@/hooks/use-theme';

/**
 * The button becomes its result: tap "I'm here" and the same box turns into
 * "You checked in", with a tick drawing itself. No spinner followed by a
 * toast. The swap only animates when `done` changes on screen; opening a
 * screen that is already done just shows the result.
 */
export function ResultButton({
  label,
  doneLabel,
  done,
  busy,
  onPress,
}: {
  label: string;
  doneLabel: string;
  done: boolean;
  busy?: boolean;
  onPress: () => void;
}) {
  const theme = useTheme();
  const m = useMotion();
  const shown = useSharedValue(done ? 1 : 0);
  // Whether `done` has changed since mounting (React's "store information
  // from previous renders" pattern), so only a change on screen animates.
  const [initial] = useState(done);
  const [changed, setChanged] = useState(false);
  if (!changed && done !== initial) setChanged(true);
  const { gather } = m;
  useEffect(() => {
    if (changed) shown.value = gather(done ? 1 : 0);
  }, [done, changed, gather, shown]);

  const before = useAnimatedStyle(() => ({
    opacity: 1 - shown.value,
    transform: [{ scale: 1 + shown.value * 0.04 }],
  }));
  const after = useAnimatedStyle(() => ({
    opacity: shown.value,
    transform: [{ scale: 0.92 + shown.value * 0.08 }],
  }));

  return (
    <View style={styles.box}>
      <Animated.View style={[StyleSheet.absoluteFill, before]} pointerEvents={done ? 'none' : 'auto'}>
        <PrimaryButton label={label} busy={busy} onPress={onPress} />
      </Animated.View>
      <Animated.View
        style={[StyleSheet.absoluteFill, styles.status, { backgroundColor: theme.backgroundElement }, after]}
        pointerEvents="none"
        aria-hidden={!done}
        accessibilityLiveRegion="polite"
        accessibilityRole="text">
        <Tick color={theme.ok} drawn={done} animate={changed} />
        <ThemedText type="smallBold" themeColor="ok" style={styles.statusText}>
          {doneLabel}
        </ThemedText>
      </Animated.View>
    </View>
  );
}

const styles = StyleSheet.create({
  box: { minHeight: 56 },
  status: { borderRadius: 12, flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: Spacing.two, paddingHorizontal: Spacing.three },
  statusText: { fontSize: 15, textAlign: 'center', flexShrink: 1 },
});
