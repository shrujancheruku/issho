import { Link } from 'expo-router';
import { Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { LanguageSwitch } from '@/components/language-switch';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BottomTabInset, FormWidth, Spacing, TopTabInset } from '@/constants/theme';
import { AccountActions } from '@/features/account/account-actions';
import { steps } from '@/features/profile/draft';
import { stepTitles } from '@/features/profile/steps';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

/** Settings: each onboarding step (except the code of conduct) can be edited on its own. */
export default function ProfileScreen() {
  const { t } = useI18n();
  const theme = useTheme();
  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill} edges={['top']}>
        <ScrollView contentContainerStyle={styles.content}>
          <View style={styles.header}>
            <ThemedText type="subtitle" style={styles.grow}>
              {t('profile.title')}
            </ThemedText>
            <LanguageSwitch />
          </View>
          <View style={[styles.list, { backgroundColor: theme.surface }]}>
            {steps
              .filter((s) => s !== 'conduct')
              .map((s) => (
                <Link key={s} href={{ pathname: '/profile-edit/[step]', params: { step: s } }} asChild>
                  <Pressable accessibilityRole="button" style={styles.row}>
                    <ThemedText style={styles.grow}>{t(stepTitles[s])}</ThemedText>
                    <ThemedText themeColor="textSecondary">›</ThemedText>
                  </Pressable>
                </Link>
              ))}
          </View>
          <AccountActions />
        </ScrollView>
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  grow: { flex: 1 },
  content: {
    padding: Spacing.four,
    paddingTop: TopTabInset + Spacing.four,
    paddingBottom: BottomTabInset + Spacing.four,
    gap: Spacing.four,
    width: '100%',
    maxWidth: FormWidth,
    alignSelf: 'center',
  },
  header: { flexDirection: 'row', alignItems: 'center', gap: Spacing.three },
  list: { borderRadius: 16, overflow: 'hidden' },
  row: { flexDirection: 'row', alignItems: 'center', paddingVertical: Spacing.three, paddingHorizontal: Spacing.three },
});
