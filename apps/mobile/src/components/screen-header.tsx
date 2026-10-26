import type { ReactNode } from 'react';
import { StyleSheet, View } from 'react-native';

import { LanguageSwitch } from '@/components/language-switch';
import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';

/** A tab's title in the display face, with the language switch (or another control) beside it. */
export function ScreenHeader({ title, end }: { title: string; end?: ReactNode }) {
  return (
    <View style={styles.header}>
      <ThemedText type="subtitle" style={styles.title} accessibilityRole="header">
        {title}
      </ThemedText>
      {end ?? <LanguageSwitch />}
    </View>
  );
}

/** A section's heading: plain bold text, the section's content says the rest. */
export function SectionTitle({ children, accessory }: { children: string; accessory?: ReactNode }) {
  return (
    <View style={styles.section}>
      {accessory}
      <ThemedText style={styles.sectionText} accessibilityRole="header">
        {children}
      </ThemedText>
    </View>
  );
}

const styles = StyleSheet.create({
  header: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: Spacing.three },
  // Dela Gothic's ascent and descent add up to about 1.45 em: a tighter
  // line cuts off descenders (the g of "Together") on Android.
  title: { flex: 1, fontSize: 34, lineHeight: 50 },
  section: { flexDirection: 'row', alignItems: 'center', gap: Spacing.two },
  sectionText: { fontWeight: 700 },
});
