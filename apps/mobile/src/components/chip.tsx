import { Pressable, StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

export type ChipColors = { background: string; text: string };


/**
 * A selectable pill. `secondary` is the same label in the second display
 * language, shown underneath (short text is shown side by side).
 * `selectedColors` overrides the selected look, e.g. per interest level.
 */
export function Chip({
  label,
  secondary,
  selected,
  selectedColors,
  badge,
  onPress,
  disabled,
}: {
  label: string;
  secondary?: string;
  selected: boolean;
  selectedColors?: ChipColors;
  badge?: string;
  onPress?: () => void;
  disabled?: boolean;
}) {
  const theme = useTheme();
  // With a second name, the chip shows a catalogue name: the label is in the
  // primary language and the second name in the secondary one, whichever is
  // being shown (see features/profile/data.ts).
  const { languages } = useI18n();
  const labelLang = secondary ? languages.primary : undefined;
  // Plain selection is the accent; gold fills are reserved for interest levels.
  const colors = selected ? (selectedColors ?? { background: theme.accent, text: theme.onAccent }) : { background: theme.backgroundElement, text: theme.text };
  const muted = selected ? colors.text : theme.textSecondary;
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ selected, disabled }}
      accessibilityLabel={[label, secondary, badge].filter(Boolean).join(', ')}
      onPress={onPress}
      disabled={disabled || !onPress}
      style={({ pressed }) => [
        styles.chip,
        { backgroundColor: colors.background, opacity: disabled ? 0.4 : pressed ? 0.7 : 1 },
      ]}>
      {/* flexShrink lets long labels wrap inside the chip instead of overflowing the row. */}
      <View style={styles.label}>
        <ThemedText type="small" style={{ color: colors.text }} lang={labelLang}>
          {label}
        </ThemedText>
        {!!secondary && (
          <ThemedText type="small" style={[styles.secondary, { color: muted }]} lang={languages.secondary}>
            {secondary}
          </ThemedText>
        )}
        {!!badge && (
          <ThemedText type="small" style={[styles.badge, { color: colors.text }]}>
            {badge}
          </ThemedText>
        )}
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  chip: { borderRadius: 12, paddingVertical: Spacing.two, paddingHorizontal: Spacing.three, maxWidth: '100%' },
  label: { flexShrink: 1 },
  secondary: { fontSize: 12, lineHeight: 16, opacity: 0.85 },
  badge: { fontSize: 11, lineHeight: 14, fontWeight: 700 },
});
