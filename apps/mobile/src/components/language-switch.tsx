import { Pressable, StyleSheet } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

/**
 * Switches longer text between the two display languages. Hidden when the
 * person reads only one.
 */
export function LanguageSwitch() {
  const theme = useTheme();
  const { languages, showing, toggleShowing, t } = useI18n();
  if (!languages.secondary) return null;
  const [current, other] =
    showing === 'primary' ? [languages.primary, languages.secondary] : [languages.secondary, languages.primary];
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={t('language.switch')}
      onPress={toggleShowing}
      hitSlop={8}
      style={[styles.switch, { backgroundColor: theme.backgroundElement }]}>
      <ThemedText type="smallBold">{current.toUpperCase()}</ThemedText>
      <ThemedText type="small" themeColor="textSecondary">
        ⇄ {other.toUpperCase()}
      </ThemedText>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  switch: {
    flexDirection: 'row',
    gap: Spacing.one,
    alignSelf: 'flex-end',
    borderRadius: 999,
    paddingVertical: Spacing.one,
    paddingHorizontal: Spacing.three,
  },
});
