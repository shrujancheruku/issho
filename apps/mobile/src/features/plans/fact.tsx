import { useState, type ReactNode } from 'react';
import { Pressable, StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';

/** A list of Fact rows, spaced so each reads as its own detail. */
export function Facts({ children }: { children: ReactNode }) {
  return <View style={styles.facts}>{children}</View>;
}

/**
 * One detail of a plan or event as a light two-column row: the label in a narrow
 * grey column, the value beside it, an optional note under the value.
 * `info` adds a tappable ⓘ after the value that shows `infoText` below it.
 */
export function Fact({
  label,
  value,
  note,
  info,
}: {
  label: string;
  value: string;
  note?: string;
  info?: { label: string; text: string };
}) {
  const theme = useTheme();
  const [open, setOpen] = useState(false);
  return (
    <View style={styles.fact}>
      <ThemedText type="small" themeColor="textSecondary" style={styles.factLabel}>
        {label}
      </ThemedText>
      <View style={styles.factBody}>
        <View style={styles.factValueRow}>
          <ThemedText style={styles.factValue}>{value}</ThemedText>
          {info && (
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={info.label}
              accessibilityState={{ expanded: open }}
              onPress={() => setOpen(!open)}
              hitSlop={10}
              style={[styles.info, { borderColor: open ? theme.text : theme.textSecondary }]}>
              <ThemedText type="small" style={[styles.infoMark, { color: open ? theme.text : theme.textSecondary }]}>
                i
              </ThemedText>
            </Pressable>
          )}
        </View>
        {!!note && (
          <ThemedText type="small" themeColor="textSecondary">
            {note}
          </ThemedText>
        )}
        {info && open && (
          <ThemedText type="small" themeColor="textSecondary">
            {info.text}
          </ThemedText>
        )}
      </View>
    </View>
  );
}

/** A section heading with its hint directly underneath, read as one unit. */
export function Heading({ title, hint }: { title: string; hint: string }) {
  return (
    <View style={styles.heading}>
      <ThemedText type="smallBold">{title}</ThemedText>
      <ThemedText type="small" themeColor="textSecondary">
        {hint}
      </ThemedText>
    </View>
  );
}

const styles = StyleSheet.create({
  facts: { gap: Spacing.three },
  fact: { flexDirection: 'row', gap: Spacing.three },
  // Wide enough for "Number of people interested" to wrap onto two lines, not five.
  factLabel: { width: 112, paddingTop: 2 },
  factBody: { flex: 1, gap: Spacing.half },
  factValueRow: { flexDirection: 'row', alignItems: 'center', gap: Spacing.two, flexWrap: 'wrap' },
  factValue: { fontSize: 16, lineHeight: 22 },
  info: { width: 20, height: 20, borderRadius: 10, borderWidth: 1.5, alignItems: 'center', justifyContent: 'center' },
  infoMark: { fontSize: 12, lineHeight: 14, fontWeight: 700 },
  heading: { gap: Spacing.half },
});
