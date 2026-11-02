import { router, type Href } from 'expo-router';
import type { ReactNode } from 'react';
import { KeyboardAvoidingView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { LanguageSwitch } from '@/components/language-switch';
import { BackLink } from '@/components/text-link';
import { ThemedView } from '@/components/themed-view';
import { FormWidth, Spacing } from '@/constants/theme';
import { useI18n } from '@/i18n/i18n-provider';

/** Centers a screen's content in a column that stays phone-width on a wide browser. */
export const columnStyle = { width: '100%', maxWidth: FormWidth, alignSelf: 'center' } as const;

/**
 * What every back button does: go back, or, when there is nothing to go
 * back to (a link opened on the web, a reload), replace the screen with
 * `fallback`.
 */
export function useLeave(fallback: Href) {
  return () => (router.canGoBack() ? router.back() : router.replace(fallback));
}

/**
 * The frame of a screen that isn't a tab: safe area, the keyboard kept off
 * the content, the column, and the header. `onBack` gives the usual header
 * (back link and the language switch); `header` replaces it for screens
 * whose header says more (a step count, a title). The body is `children`,
 * so each screen chooses between a scrolling body and a fixed head.
 */
export function ScreenFrame({ onBack, header, children }: { onBack?: () => void; header?: ReactNode; children: ReactNode }) {
  const { t } = useI18n();
  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <KeyboardAvoidingView behavior="padding" style={[styles.fill, columnStyle]}>
          {header ??
            (onBack && (
              <View style={styles.header}>
                <BackLink label={t('common.back')} onPress={onBack} />
                <LanguageSwitch />
              </View>
            ))}
          {children}
        </KeyboardAvoidingView>
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  header: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: Spacing.three },
});
