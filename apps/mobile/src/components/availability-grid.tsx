import { TimeBlock } from '@issho/api-client';
import { Pressable, StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Brand, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { blocks, slotKey, weekdays } from '@/lib/slots';

export const blockKeys: Record<TimeBlock, StringKey> = {
  [TimeBlock.UNSPECIFIED]: 'plan.wholeDay',
  [TimeBlock.MORNING]: 'profile.block.morning',
  [TimeBlock.AFTERNOON]: 'profile.block.afternoon',
  [TimeBlock.EVENING]: 'profile.block.evening',
};

/** Weekday names from the platform (Intl), in the language being shown. */
export function weekdayName(weekday: number, lang: string) {
  // 2024-01-07 was a Sunday (weekday 0).
  const date = new Date(Date.UTC(2024, 0, 7 + weekday, 12));
  try {
    return new Intl.DateTimeFormat(lang, { weekday: 'short', timeZone: 'UTC' }).format(date);
  } catch {
    return new Intl.DateTimeFormat('en', { weekday: 'short', timeZone: 'UTC' }).format(date);
  }
}

/**
 * The week as a grid of parts of the day to tap on and off: the profile's
 * usual availability, and the times that suit someone for one plan.
 * `selected` holds slotKey values.
 */
export function AvailabilityGrid({
  selected,
  onToggle,
}: {
  selected: readonly string[];
  onToggle: (weekday: number, block: TimeBlock) => void;
}) {
  const { t, languages, showing } = useI18n();
  const theme = useTheme();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  return (
    <View style={styles.grid}>
      <View style={styles.row}>
        <View style={styles.day} />
        {blocks.map((b) => (
          <ThemedText key={b} type="small" themeColor="textSecondary" style={styles.head}>
            {t(blockKeys[b])}
          </ThemedText>
        ))}
      </View>
      {weekdays.map((day) => (
        <View key={day} style={styles.row}>
          <ThemedText type="small" style={styles.day}>
            {weekdayName(day, lang)}
          </ThemedText>
          {blocks.map((b) => {
            const on = selected.includes(slotKey(day, b));
            return (
              <Pressable
                key={b}
                accessibilityRole="checkbox"
                accessibilityState={{ checked: on }}
                accessibilityLabel={`${weekdayName(day, lang)} ${t(blockKeys[b])}`}
                onPress={() => onToggle(day, b)}
                style={[styles.cell, { backgroundColor: on ? Brand.red : theme.backgroundElement }]}
              />
            );
          })}
        </View>
      ))}
    </View>
  );
}

const styles = StyleSheet.create({
  grid: { gap: Spacing.three },
  row: { flexDirection: 'row', alignItems: 'center', gap: Spacing.two },
  head: { flex: 1, textAlign: 'center' },
  day: { width: 56 },
  cell: { flex: 1, height: 40, borderRadius: 8 },
});
