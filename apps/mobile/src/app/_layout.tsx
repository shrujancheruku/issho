import { DelaGothicOne_400Regular, useFonts } from '@expo-google-fonts/dela-gothic-one';
import {
  ZenKakuGothicNew_400Regular,
  ZenKakuGothicNew_500Medium,
  ZenKakuGothicNew_700Bold,
} from '@expo-google-fonts/zen-kaku-gothic-new';
import { MutationCache, QueryCache, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { DarkTheme, DefaultTheme, Stack, ThemeProvider } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { ActivityIndicator, Pressable, StyleSheet } from 'react-native';
import { GestureHandlerRootView } from 'react-native-gesture-handler';

import { SplashOverlay } from '@/components/splash-overlay';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Colors, Spacing } from '@/constants/theme';
import { AccountActions } from '@/features/account/account-actions';
import { AuthProvider, isSuspendedError, useAuth } from '@/features/auth/auth-provider';
import { useColorScheme } from '@/hooks/use-color-scheme';
import { I18nProvider, useI18n } from '@/i18n/i18n-provider';
import { withMonitoring } from '@/lib/monitoring';

SplashScreen.preventAutoHideAsync();

// Suspended mid-session: every call is refused, so load the account again
// and the navigator shows the suspended screen.
const onError = (err: unknown) => {
  if (isSuspendedError(err)) void queryClient.invalidateQueries({ queryKey: ['me'] });
};
const queryClient = new QueryClient({
  queryCache: new QueryCache({ onError }),
  mutationCache: new MutationCache({ onError }),
  // Data counts as fresh for 30 s: going back and forth between tabs and
  // screens reuses it instead of asking the API again on every visit (each
  // request is a few hundred ms from a phone). Changes the person makes
  // invalidate what they affect, and pull to refresh still refetches.
  defaultOptions: { queries: { staleTime: 30_000 } },
});

// React Navigation paints screens, headers and the tab bar from its own theme,
// so give it the app's grounds; otherwise screens flash white or black.
const navTheme = (base: typeof DefaultTheme, c: (typeof Colors)['light' | 'dark']) => ({
  ...base,
  colors: { ...base.colors, primary: c.accent, background: c.background, card: c.surface, text: c.text, border: c.backgroundSelected },
});
const navLight = navTheme(DefaultTheme, Colors.light);
const navDark = navTheme(DarkTheme, Colors.dark);

function RootLayout() {
  const colorScheme = useColorScheme();
  // The keys become the font family names (DisplayFont and BodyFont in constants/theme.ts).
  const [fontsLoaded, fontError] = useFonts({
    DelaGothicOne_400Regular,
    ZenKakuGothicNew_400Regular,
    ZenKakuGothicNew_500Medium,
    ZenKakuGothicNew_700Bold,
  });
  // Keep the splash screen up until the fonts are ready, so text never flashes in
  // the system font. If loading fails, carry on: text falls back to the system font.
  if (!fontsLoaded && !fontError) return null;

  return (
    // Gesture Handler needs a root for its detectors (the plans sheet, the week grid).
    <GestureHandlerRootView style={styles.root}>
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          {/* Inside AuthProvider: the languages come from the account. */}
          <I18nProvider>
            <ThemeProvider value={colorScheme === 'dark' ? navDark : navLight}>
              <SplashOverlay />
              <RootNavigator />
            </ThemeProvider>
          </I18nProvider>
        </AuthProvider>
      </QueryClientProvider>
    </GestureHandlerRootView>
  );
}

/**
 * Routes are gated by auth state with Stack.Protected:
 *   no session              → sign-in
 *   session, no profile     → onboarding
 *   session and onboarded   → (tabs)
 * When state changes (sign-in, sign-out, onboarding done), the guards flip and
 * Expo Router redirects automatically, clearing history for blocked screens.
 * These guards are UX only; the API enforces auth on every request.
 */
function RootNavigator() {
  const { ready, session, onboarded, suspended, accountError, retryAccount, signOut } = useAuth();
  const { t } = useI18n();
  if (!ready) return null; // the splash screen stays up while SecureStore is read

  // Suspended by a moderator: the API refuses everything else anyway, except
  // deleting the account, which shouldn't depend on a moderator.
  if (session && suspended) {
    return (
      <ThemedView style={styles.center}>
        <ThemedText type="subtitle">{t('account.suspendedTitle')}</ThemedText>
        <ThemedText style={styles.message}>{t('account.suspendedBody')}</ThemedText>
        <AccountActions />
      </ThemedView>
    );
  }

  // Signed in, but the API hasn't told us yet whether the user is onboarded.
  if (session && onboarded === null) {
    return (
      <ThemedView style={styles.center}>
        {accountError ? (
          <>
            <ThemedText>{t('error.network')}</ThemedText>
            <Pressable onPress={retryAccount}>
              <ThemedText type="link">{t('common.retry')}</ThemedText>
            </Pressable>
            <Pressable onPress={signOut}>
              <ThemedText type="link">{t('account.signOut')}</ThemedText>
            </Pressable>
          </>
        ) : (
          <ActivityIndicator />
        )}
      </ThemedView>
    );
  }

  return (
    <Stack screenOptions={{ headerShown: false }}>
      <Stack.Protected guard={!session}>
        <Stack.Screen name="sign-in" />
      </Stack.Protected>
      <Stack.Protected guard={!!session && onboarded === false}>
        <Stack.Screen name="onboarding" />
      </Stack.Protected>
      <Stack.Protected guard={!!session && onboarded === true}>
        <Stack.Screen name="(tabs)" />
        <Stack.Screen name="profile-edit/[step]" />
        <Stack.Screen name="plan/[id]" />
        {/* Choosing times slides up over the plan, as a sheet on phones. */}
        <Stack.Screen
          name="signup/[id]"
          options={{ presentation: 'formSheet', sheetAllowedDetents: [0.92], sheetGrabberVisible: true, sheetCornerRadius: 20 }}
        />
        <Stack.Screen name="event/[id]" />
        <Stack.Screen name="chat/[id]" />
        <Stack.Screen name="feedback/[id]" />
        <Stack.Screen name="report/[id]" />
        <Stack.Screen name="crew/[id]" />
        <Stack.Screen name="crew-leave/[id]" />
      </Stack.Protected>
    </Stack>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1 },
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', gap: Spacing.three, padding: Spacing.four },
  message: { textAlign: 'center', maxWidth: 420 },
});

// Reports render crashes when Sentry is on (lib/monitoring.ts).
export default withMonitoring(RootLayout);
