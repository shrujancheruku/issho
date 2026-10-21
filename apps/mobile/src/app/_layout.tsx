import { DelaGothicOne_400Regular, useFonts } from '@expo-google-fonts/dela-gothic-one';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { DarkTheme, DefaultTheme, Stack, ThemeProvider } from 'expo-router';
import * as SplashScreen from 'expo-splash-screen';
import { ActivityIndicator, Pressable, StyleSheet, useColorScheme } from 'react-native';

import { AnimatedSplashOverlay } from '@/components/animated-icon';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Spacing } from '@/constants/theme';
import { AuthProvider, useAuth } from '@/features/auth/auth-provider';
import { I18nProvider, useI18n } from '@/i18n/i18n-provider';

SplashScreen.preventAutoHideAsync();

const queryClient = new QueryClient();

export default function RootLayout() {
  const colorScheme = useColorScheme();
  // The key becomes the font family name (DisplayFont in constants/theme.ts).
  const [fontsLoaded, fontError] = useFonts({ DelaGothicOne_400Regular });
  // Keep the splash screen up until the font is ready, so headings never flash in
  // the system font. If loading fails, carry on: text falls back to the system font.
  if (!fontsLoaded && !fontError) return null;

  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        {/* Inside AuthProvider: the languages come from the account. */}
        <I18nProvider>
          <ThemeProvider value={colorScheme === 'dark' ? DarkTheme : DefaultTheme}>
            <AnimatedSplashOverlay />
            <RootNavigator />
          </ThemeProvider>
        </I18nProvider>
      </AuthProvider>
    </QueryClientProvider>
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
  const { ready, session, onboarded, accountError, retryAccount, signOut } = useAuth();
  const { t } = useI18n();
  if (!ready) return null; // the splash screen stays up while SecureStore is read

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
      </Stack.Protected>
    </Stack>
  );
}

const styles = StyleSheet.create({
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', gap: Spacing.three, padding: Spacing.four },
});
