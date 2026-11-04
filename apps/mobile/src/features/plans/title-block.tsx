import { StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useOtherLanguage } from '@/i18n/i18n-provider';

/**
 * A plan's or event's name in the language being read, with the other
 * language under it when both are shown, and an optional highlighted line
 * (the event's state) in place of that second name.
 */
export function TitleBlock({ name, secondary, detail }: { name: string; secondary?: string; detail?: string }) {
  const otherLanguage = useOtherLanguage();
  return (
    <View style={styles.block}>
      <ThemedText type="subtitle" accessibilityRole="header">
        {name}
      </ThemedText>
      {!!secondary && !detail && (
        <ThemedText themeColor="textSecondary" lang={otherLanguage}>
          {secondary}
        </ThemedText>
      )}
      {!!detail && (
        <ThemedText type="smallBold" themeColor="link">
          {detail}
        </ThemedText>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  block: { gap: Spacing.half },
});
