import type { PlanLevel } from '@issho/api-client';
import { StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

import { levelColors, levelLabels } from './level';

/**
 * How a plan suits the person, as a pill ("Love it", "Like it"). A level
 * without a colour (low, not for you) shows grey when `plain` is set and
 * nothing otherwise.
 */
export function LevelTag({ level, plain = false, style }: { level: PlanLevel; plain?: boolean; style?: StyleProp<ViewStyle> }) {
  const { t } = useI18n();
  const theme = useTheme();
  const colors = levelColors(level, theme);
  if (!colors && !plain) return null;
  return (
    <View style={[styles.tag, { backgroundColor: colors?.background ?? theme.backgroundSelected }, style]}>
      <ThemedText type="caption" style={{ color: colors?.text ?? theme.textSecondary, textAlign: 'center' }}>
        {t(levelLabels[level])}
      </ThemedText>
    </View>
  );
}

const styles = StyleSheet.create({
  tag: { borderRadius: 999, paddingVertical: Spacing.half, paddingHorizontal: Spacing.two },
});
