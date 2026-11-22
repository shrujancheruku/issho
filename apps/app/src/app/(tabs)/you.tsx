import { useRouter } from 'expo-router';
import { ScrollView, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { useAuth } from '@/auth/auth-provider';
import { Account } from '@/components/account';
import { ApiVersion } from '@/components/api-version';
import { Button } from '@/components/form';
import { JoinLinks } from '@/components/join-links';
import { ProfileSummary } from '@/components/profile-summary';
import { RequireSignIn } from '@/components/require-sign-in';
import { FontSize, MaxContentWidth, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

export default function You() {
  return (
    <RequireSignIn path="/you">
      <Signed />
    </RequireSignIn>
  );
}

function Signed() {
  const { t } = useI18n();
  const theme = useTheme();
  const auth = useAuth();
  const router = useRouter();
  // The tab bar sits below, but nothing keeps the title clear of the status bar (Android draws edge to edge).
  const insets = useSafeAreaInsets();

  const signOut = async () => {
    // Leave first: once signed out, this screen would send the person to sign-in, and they chose to go.
    router.replace('/plans');
    await auth.signOut();
  };

  return (
    <View testID="screen" style={[styles.screen, { backgroundColor: theme.background, paddingTop: insets.top + Spacing.four }]}>
      <ScrollView style={styles.scroll} contentContainerStyle={styles.column}>
        <Text accessibilityRole="header" style={[styles.title, { color: theme.text }]}>
          {t('tabs.you')}
        </Text>
        <Account />
        <JoinLinks />
        <ProfileSummary />
        <Button quiet label={t('you.signOut')} onPress={signOut} />
        <ApiVersion />
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, alignItems: 'center', paddingHorizontal: Spacing.four },
  scroll: { width: '100%', maxWidth: MaxContentWidth },
  column: { gap: Spacing.three, paddingBottom: Spacing.four },
  title: { fontSize: FontSize.subtitle, fontWeight: '700' },
});
