import { Children, Fragment, isValidElement, type ReactNode } from 'react';
import { Pressable, StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { isRightToLeft } from '@/i18n/direction';
import { useI18n } from '@/i18n/i18n-provider';

/**
 * Rows on one surface with hairlines between them (Together's lists, You's
 * settings). `inset` lines the hairline up with the rows' text, past an
 * avatar.
 */
export function ListGroup({ children, inset = Spacing.three }: { children: ReactNode; inset?: number }) {
  const theme = useTheme();
  const rows = Children.toArray(children).filter(isValidElement);
  return (
    <View style={[styles.group, { backgroundColor: theme.surface }]}>
      {rows.map((row, i) => (
        <Fragment key={row.key ?? i}>
          {i > 0 && <View style={[styles.hairline, { backgroundColor: theme.backgroundSelected, marginStart: inset }]} />}
          {row}
        </Fragment>
      ))}
    </View>
  );
}

/**
 * One tappable row. Full-width rows show a pressed background rather than
 * pressing in: scaling a whole row reads as the list shifting.
 */
export function ListRow({
  onPress,
  children,
  chevron = true,
  accessibilityLabel,
  style,
}: {
  onPress?: () => void;
  children: ReactNode;
  chevron?: boolean;
  accessibilityLabel?: string;
  style?: StyleProp<ViewStyle>;
}) {
  const theme = useTheme();
  const { languages } = useI18n();
  const flip = isRightToLeft(languages.primary);
  return (
    <Pressable
      accessibilityRole={onPress ? 'button' : undefined}
      accessibilityLabel={accessibilityLabel}
      onPress={onPress}
      disabled={!onPress}
      style={({ pressed }) => [styles.row, pressed && { backgroundColor: theme.backgroundElement }, style]}>
      {children}
      {chevron && !!onPress && (
        <ThemedText themeColor="textSecondary" style={flip && styles.flip} aria-hidden>
          ›
        </ThemedText>
      )}
    </Pressable>
  );
}

const styles = StyleSheet.create({
  group: { borderRadius: 16, overflow: 'hidden' },
  hairline: { height: StyleSheet.hairlineWidth * 2 },
  row: { flexDirection: 'row', alignItems: 'center', gap: Spacing.three - 4, paddingVertical: Spacing.three - 4, paddingHorizontal: Spacing.three, minHeight: 48 },
  // The chevron points the reading direction's way.
  flip: { transform: [{ scaleX: -1 }] },
});
