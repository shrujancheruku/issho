import { Redirect, useLocalSearchParams, useRouter } from 'expo-router';
import { useEffect, useState } from 'react';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { useApi } from '@/api/api-provider';
import { useAuth } from '@/auth/auth-provider';
import { returnPath } from '@/auth/return-path';
import { ErrorText } from '@/components/form';
import { ProfileForm } from '@/components/profile-form';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { Answers } from '@/profile/answers';

/**
 * The profile's questions. A signup with no profile opens this with `?next=` set
 * to itself (`useProfileGate`), and finishing returns the person there; the You
 * tab opens it to change a profile already given, which it then shows whole.
 */
export default function Profile() {
  const { t } = useI18n();
  const theme = useTheme();
  const auth = useAuth();
  const api = useApi();
  const router = useRouter();
  // Outside the tabs, so nothing else keeps the title clear of the status bar (Android draws edge to edge).
  const insets = useSafeAreaInsets();
  const next = returnPath(useLocalSearchParams<{ next?: string }>().next);
  // undefined until the API answers; null when it has none.
  const [existing, setExisting] = useState<Answers | null | undefined>(undefined);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (auth.status !== 'signedIn') return;
    let live = true;
    api.profile
      .getProfile({})
      .then((res) => {
        const p = res.profile;
        if (!live) return;
        setExisting(
          p
            ? {
                name: p.name,
                dateOfBirth: '',
                gender: p.gender as Answers['gender'],
                languages: [...p.languages],
                livesInCity: p.livesInCity,
                acceptedCodeOfConduct: true,
                onlyWomenAndNonBinary: p.onlyWomenAndNonBinary,
              }
            : null,
        );
      })
      .catch(() => live && setFailed(true));
    return () => {
      live = false;
    };
  }, [api, auth.status]);

  if (auth.status === 'loading') return null;
  if (auth.status === 'signedOut') return <Redirect href={{ pathname: '/sign-in', params: { next: '/profile' } }} />;

  return (
    <View testID="screen" style={[styles.screen, { backgroundColor: theme.background, paddingTop: insets.top + Spacing.four }]}>
      <ScrollView style={styles.scroll} contentContainerStyle={styles.column} keyboardShouldPersistTaps="handled">
        <Text accessibilityRole="header" style={[styles.title, { color: theme.text }]}>
          {t('profile.title')}
        </Text>
        {failed ? (
          <ErrorText>{t('profile.loadFailed')}</ErrorText>
        ) : existing === undefined ? null : (
          <ProfileForm existing={existing ?? undefined} onSaved={() => router.replace(next as never)} />
        )}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, alignItems: 'center', paddingHorizontal: Spacing.four },
  scroll: { width: '100%', maxWidth: 420 },
  column: { gap: Spacing.three, paddingBottom: Spacing.five },
  title: { fontSize: FontSize.subtitle, fontWeight: '700' },
});
