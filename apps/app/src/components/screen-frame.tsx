import type { ReactNode } from 'react';
import { ScrollView, StyleSheet, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { MaxContentWidth, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';

/**
 * A screen's ground and its one centred column, scrolling. It starts below the
 * top inset: on Android the app draws edge to edge, so without it the title
 * sits under the status bar (the same fix as the sign-in screen's).
 */
export function ScreenFrame({ children }: { children: ReactNode }) {
  const theme = useTheme();
  const insets = useSafeAreaInsets();
  return (
    <View testID="screen" style={[styles.screen, { backgroundColor: theme.background, paddingTop: insets.top + Spacing.four }]}>
      <ScrollView contentContainerStyle={styles.scroll}>
        <View style={styles.column}>{children}</View>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1 },
  scroll: { alignItems: 'center', paddingHorizontal: Spacing.four, paddingBottom: Spacing.four },
  column: { width: '100%', maxWidth: MaxContentWidth, gap: Spacing.three },
});
