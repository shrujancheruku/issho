import { Pressable, StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Brand, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';

/**
 * A selectable pill. `secondary` is the same label in the second display
 * language, shown underneath (short text is shown side by side). `strength`
 * shades a selected chip for graded choices (activity interest 1–3).
 */
export function Chip({
  label,
  secondary,
  selected,
  strength = 3,
  badge,
  onPress,
  disabled,
}: {
  label: string;
  secondary?: string;
  selected: boolean;
  strength?: 1 | 2 | 3;
  badge?: string;
  onPress: () => void;
  disabled?: boolean;
}) {
  const theme = useTheme();
  const fill = selected ? [undefined, Brand.gold + '55', Brand.gold, Brand.red][strength] : theme.backgroundElement;
  const onFill = selected && strength === 3;
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityState={{ selected, disabled }}
      accessibilityLabel={[label, secondary, badge].filter(Boolean).join(', ')}
      onPress={onPress}
      disabled={disabled}
      style={({ pressed }) => [
        styles.chip,
        { backgroundColor: fill, opacity: disabled ? 0.4 : pressed ? 0.7 : 1 },
      ]}>
      <View>
        <ThemedText type="small" style={onFill && styles.onFill}>
          {label}
        </ThemedText>
        {!!secondary && (
          <ThemedText type="small" themeColor="textSecondary" style={[styles.secondary, onFill && styles.onFill]}>
            {secondary}
          </ThemedText>
        )}
        {!!badge && (
          <ThemedText type="small" style={[styles.badge, onFill && styles.onFill]}>
            {badge}
          </ThemedText>
        )}
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  chip: { borderRadius: 12, paddingVertical: Spacing.two, paddingHorizontal: Spacing.three },
  secondary: { fontSize: 12, lineHeight: 16 },
  badge: { fontSize: 11, lineHeight: 14, fontWeight: 700 },
  onFill: { color: '#ffffff' },
});
