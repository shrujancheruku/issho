import { StyleSheet, View } from 'react-native';

import { PressableScale } from '@/components/pressable-scale';
import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { haptics } from '@/lib/haptics';

/**
 * Two or three views of one thing (Group chat / Plan and meeting point;
 * Match my phone / Light / Dark). `onAccent` for use on an accent header.
 */
export function Segmented<T extends string>({
  options,
  value,
  onChange,
  label,
  onAccent = false,
}: {
  options: readonly { value: T; label: string }[];
  value: T;
  onChange: (v: T) => void;
  label: string;
  onAccent?: boolean;
}) {
  const theme = useTheme();
  return (
    <View
      accessibilityRole="tablist"
      accessibilityLabel={label}
      style={[styles.bar, { backgroundColor: onAccent ? 'rgba(255,255,255,0.14)' : theme.backgroundElement }]}>
      {options.map((o) => {
        const on = o.value === value;
        return (
          <PressableScale
            key={o.value}
            accessibilityRole="tab"
            accessibilityState={{ selected: on }}
            onPress={() => {
              if (on) return;
              haptics.selection();
              onChange(o.value);
            }}
            style={[styles.seg, on && { backgroundColor: onAccent ? 'rgba(255,255,255,0.22)' : theme.surface }]}>
            <ThemedText
              type="smallBold"
              style={{ color: onAccent ? theme.onAccent : on ? theme.text : theme.textSecondary, textAlign: 'center' }}>
              {o.label}
            </ThemedText>
          </PressableScale>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  bar: { flexDirection: 'row', gap: Spacing.one, padding: Spacing.one, borderRadius: 12 },
  // Grow from each label's width, so a longer label ("Match my phone") gets more room instead of wrapping.
  seg: { flexGrow: 1, flexShrink: 1, flexBasis: 'auto', minHeight: 40, borderRadius: 10, alignItems: 'center', justifyContent: 'center', paddingHorizontal: Spacing.two },
});
