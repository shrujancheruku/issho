import { useRouter } from 'expo-router';
import { StyleSheet, Text, View } from 'react-native';

import { useAuth } from '@/auth/auth-provider';
import { Account } from '@/components/account';
import { ApiVersion } from '@/components/api-version';
import { Button } from '@/components/form';
import { JoinLinks } from '@/components/join-links';
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

  const signOut = async () => {
    // Leave first: once signed out, this screen would send the person to sign-in, and they chose to go.
    router.replace('/plans');
    await auth.signOut();
  };

  return (
    <View testID="screen" style={[styles.screen, { backgroundColor: theme.background }]}>
      <View style={styles.column}>
        <Text accessibilityRole="header" style={[styles.title, { color: theme.text }]}>
          {t('tabs.you')}
        </Text>
        <Account />
        <JoinLinks />
        <Button quiet label={t('you.signOut')} onPress={signOut} />
        <ApiVersion />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, alignItems: 'center', padding: Spacing.four },
  column: { width: '100%', maxWidth: MaxContentWidth, gap: Spacing.two },
  title: { fontSize: FontSize.subtitle, fontWeight: '700' },
});
