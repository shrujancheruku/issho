import { ActivityIndicator, StyleSheet } from 'react-native';

import { PressableScale } from '@/components/pressable-scale';
import { ThemedText } from '@/components/themed-text';
import { Brand, Spacing } from '@/constants/theme';

/**
 * The gold button: the one thing to do on a screen. Full width in a column;
 * `grow` fills what's left of a row (beside a Back link).
 */
export function PrimaryButton({
  label,
  onPress,
  disabled,
  busy,
  grow,
}: {
  label: string;
  onPress: () => void;
  disabled?: boolean;
  busy?: boolean;
  grow?: boolean;
}) {
  return (
    <PressableScale
      accessibilityRole="button"
      accessibilityState={{ disabled: disabled || busy, busy }}
      onPress={onPress}
      disabled={disabled || busy}
      style={[styles.button, grow && styles.grow, (disabled || busy) && styles.disabled]}>
      {busy ? <ActivityIndicator color={Brand.ink} /> : <ThemedText style={styles.label}>{label}</ThemedText>}
    </PressableScale>
  );
}

const buttonStyles = StyleSheet.create({
  // Gold, the thread: joining a plan picks it up. Ink on gold is 8:1.
  button: {
    backgroundColor: Brand.gold,
    borderRadius: 12,
    minHeight: 56,
    paddingHorizontal: Spacing.three,
    alignItems: 'center',
    justifyContent: 'center',
    flexDirection: 'row',
    gap: Spacing.two,
  },
  label: { color: Brand.ink, fontWeight: '700', textAlign: 'center' },
});

const styles = StyleSheet.create({
  button: buttonStyles.button,
  label: buttonStyles.label,
  grow: { flexGrow: 1 },
  disabled: { opacity: 0.4 },
});
