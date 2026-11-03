import { router, type Href } from 'expo-router';
import type { ReactNode } from 'react';
import { KeyboardAvoidingView, StyleSheet, View } from 'react-native';
import { SafeAreaView, useSafeAreaInsets, type Edge } from 'react-native-safe-area-context';

import { LanguageSwitch } from '@/components/language-switch';
import { BackLink } from '@/components/text-link';
import { ThemedView } from '@/components/themed-view';
import { FormWidth, MaxContentWidth, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

/** Centers a screen's content in a column that stays phone-width on a wide browser. */
export const columnStyle = {
  width: '100%',
  maxWidth: FormWidth,
  alignSelf: 'center',
} as const;

/**
 * What every back button does: go back, or, when there is nothing to go
 * back to (a link opened on the web, a reload), replace the screen with
 * `fallback`.
 */
export function useLeave(fallback: Href) {
  return () => (router.canGoBack() ? router.back() : router.replace(fallback));
}

export type FrameProps = {
  /** The usual header: a back link and the language switch. */
  onBack?: () => void;
  /**
   * Replaces the usual header (a step count, a title). With `accent` it is
   * instead the rest of the accent band, under the back row.
   */
  header?: ReactNode;
  /** More controls after the language switch in the usual header. */
  end?: ReactNode;
  /** The header on the accent colour, the band reaching up under the status bar (a crew). */
  accent?: boolean;
  /** A sheet over another screen: on the surface colour, its top edge the system's. */
  sheet?: boolean;
  /** A tab: its header has the tab title's padding and the bottom stays free for the tab bar. */
  tab?: boolean;
  /** The wider column of lists (plans, together), not the form column. */
  wide?: boolean;
};

/**
 * The frame of every screen, tabs included: safe area, the keyboard kept
 * off the content, the column, and the header. `onBack` gives the usual
 * header (back link and the language switch); `header` replaces it for
 * screens whose header says more (a tab's title, a step count). The body
 * is `children`, so each screen chooses between a scrolling body and a
 * fixed head. A new screen is a `ScreenFrame` (or a `FormScreen`) and
 * loads data through `QueryScreen`, `LoadState` or `LoadFailed`.
 */
export function ScreenFrame({ onBack, header, end, accent, tab, wide, sheet, children }: FrameProps & { children: ReactNode }) {
  const column = wide ? styles.wide : columnStyle;
  // The accent band owns the top edge; a sheet's top is the system's.
  const edges: Edge[] = accent || sheet ? ['bottom', 'left', 'right'] : tab ? ['top', 'left', 'right'] : ['top', 'bottom', 'left', 'right'];
  return (
    <ThemedView type={sheet ? 'surface' : undefined} style={styles.fill}>
      {accent && (
        <AccentBand onBack={onBack} end={end} column={column}>
          {header}
        </AccentBand>
      )}
      <SafeAreaView style={styles.fill} edges={edges}>
        <KeyboardAvoidingView behavior="padding" style={[styles.fill, column]}>
          {!accent &&
            (tab && header ? (
              <View style={styles.tabHeader}>{header}</View>
            ) : (
              (header ?? (onBack && <HeaderRow onBack={onBack} end={end} />))
            ))}
          {children}
        </KeyboardAvoidingView>
      </SafeAreaView>
    </ThemedView>
  );
}

/** The back link with the language switch (and `end`) opposite. */
function HeaderRow({ onBack, end, color, pad = true }: { onBack: () => void; end?: ReactNode; color?: string; pad?: boolean }) {
  const { t } = useI18n();
  return (
    <View style={[styles.header, pad && styles.pad]}>
      <BackLink label={t('common.back')} onPress={onBack} color={color} />
      <View style={styles.end}>
        <LanguageSwitch />
        {end}
      </View>
    </View>
  );
}

/** A header on the accent colour that reaches up under the status bar. */
function AccentBand({ onBack, end, column, children }: { onBack?: () => void; end?: ReactNode; column: object; children: ReactNode }) {
  const theme = useTheme();
  const insets = useSafeAreaInsets();
  return (
    <View
      style={{
        backgroundColor: theme.accent,
        paddingTop: insets.top + Spacing.two,
      }}>
      <View style={[column, styles.band]}>
        {onBack && <HeaderRow onBack={onBack} end={end} color={theme.onAccent} pad={false} />}
        {children}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  wide: { width: '100%', maxWidth: MaxContentWidth, alignSelf: 'center' },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  pad: { paddingHorizontal: Spacing.three },
  end: { flexDirection: 'row', alignItems: 'center', gap: Spacing.two },
  band: {
    paddingHorizontal: Spacing.three,
    paddingBottom: Spacing.three,
    gap: Spacing.two,
  },
  tabHeader: {
    paddingHorizontal: Spacing.three,
    paddingTop: Spacing.four,
    paddingBottom: Spacing.two,
  },
});
