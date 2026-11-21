import { StyleSheet, Text, View } from 'react-native';

import { FontSize, MaxContentWidth, Spacing } from '@/constants/theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { useTheme } from '@/hooks/use-theme';

/** What a tab shows until its ticket builds it: the tab's name and one line saying it is empty. */
export function EmptyScreen({ titleKey, emptyKey }: { titleKey: StringKey; emptyKey: StringKey }) {
  const { t } = useI18n();
  const theme = useTheme();
  return (
    <View testID="screen" style={[styles.screen, { backgroundColor: theme.background }]}>
      <View style={styles.column}>
        <Text accessibilityRole="header" style={[styles.title, { color: theme.text }]}>
          {t(titleKey)}
        </Text>
        <Text style={[styles.empty, { color: theme.textSecondary }]}>{t(emptyKey)}</Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, alignItems: 'center', padding: Spacing.four },
  column: { width: '100%', maxWidth: MaxContentWidth, gap: Spacing.two },
  title: { fontSize: FontSize.subtitle, fontWeight: '700' },
  empty: { fontSize: FontSize.default },
});
