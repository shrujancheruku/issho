import { ActivityIndicator, Pressable, StyleSheet } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Brand, Spacing } from '@/constants/theme';

export function PrimaryButton({
  label,
  onPress,
  disabled,
  busy,
}: {
  label: string;
  onPress: () => void;
  disabled?: boolean;
  busy?: boolean;
}) {
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ disabled: disabled || busy, busy }}
      onPress={onPress}
      disabled={disabled || busy}
      style={({ pressed }) => [styles.button, (disabled || busy) && styles.disabled, pressed && styles.pressed]}>
      {busy ? <ActivityIndicator color={Brand.ink} /> : <ThemedText style={styles.label}>{label}</ThemedText>}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  // Gold, the thread: joining a plan picks it up. Ink on gold is 7.8:1.
  button: { backgroundColor: Brand.gold, borderRadius: 12, paddingVertical: Spacing.three, alignItems: 'center', flexGrow: 1 },
  disabled: { opacity: 0.4 },
  pressed: { opacity: 0.8 },
  label: { color: Brand.ink, fontWeight: '600' },
});
