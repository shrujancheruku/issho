import { Code, ConnectError } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { AvailabilityGrid } from '@/components/availability-grid';
import { Chip } from '@/components/chip';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { FormWidth, Spacing } from '@/constants/theme';
import { useSignupActions } from '@/features/plans/data';
import { Heading } from '@/features/plans/fact';
import { formatDate } from '@/features/plans/format';
import { toggle, toSignup, windowDates, type Times } from '@/features/plans/signup';
import { usePlanTimes } from '@/features/plans/use-plan-times';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { haptics } from '@/lib/haptics';
import { registerForPush } from '@/lib/push';
import { slotKey } from '@/lib/slots';

/**
 * Signing up, as a sheet over the plan: the week with the person's usual
 * free times already chosen, and dates they can't go. Saving goes back to
 * the plan, which now shows where the outing stands.
 */
export default function SignupSheet() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t, languages, showing } = useI18n();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const theme = useTheme();
  const { plan, usual, saved } = usePlanTimes(id);
  const { signUp } = useSignupActions(id);
  // The saved times until the first edit.
  const [edited, setTimes] = useState<Times | null>(null);
  const times = edited ?? saved;
  const p = plan.data?.plan;
  const close = () => (router.canGoBack() ? router.back() : router.replace({ pathname: '/plan/[id]', params: { id } }));

  if (!p || !times) {
    return (
      <ThemedView style={[styles.fill, styles.center]}>
        {plan.isError ? <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText> : <ActivityIndicator />}
      </ThemedView>
    );
  }
  const dates = windowDates(p.windowStart, p.windowDays);
  const error = signUp.error;

  return (
    <ThemedView style={[styles.fill, { backgroundColor: theme.surface }]}>
      <SafeAreaView style={styles.fill} edges={['bottom', 'left', 'right']}>
        <View style={[styles.fill, styles.column]}>
          <View style={styles.head}>
            <View style={styles.fill}>
              <ThemedText type="subtitle" style={styles.title} accessibilityRole="header">
                {t('plan.times')}
              </ThemedText>
              <ThemedText type="small" themeColor="textSecondary">
                {t('plan.timesHint')}
              </ThemedText>
            </View>
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={t('common.close')}
              onPress={close}
              hitSlop={8}
              style={[styles.close, { backgroundColor: theme.backgroundElement }]}>
              <ThemedText style={styles.closeMark}>×</ThemedText>
            </Pressable>
          </View>
          <ScrollView contentContainerStyle={styles.content}>
            <AvailabilityGrid
              selected={times.pattern}
              onToggle={(day, b) => setTimes({ ...times, pattern: toggle(times.pattern, slotKey(day, b)) })}
            />
            <View style={styles.section}>
              <Heading title={t('plan.blockOut')} hint={t('plan.blockOutHint')} />
              <View style={styles.wrap}>
                {dates.map((d) => (
                  <Chip
                    key={d}
                    label={formatDate(d, lang)}
                    selected={times.blocked.includes(d)}
                    selectedColors={{ background: theme.text, text: theme.background }}
                    onPress={() => {
                      haptics.selection();
                      setTimes({ ...times, blocked: toggle(times.blocked, d) });
                    }}
                  />
                ))}
              </View>
            </View>
          </ScrollView>
          <View style={styles.footer}>
            {times.pattern.length === 0 && (
              <ThemedText themeColor="error" style={styles.centerText}>
                {t('plan.pickTime')}
              </ThemedText>
            )}
            {error && (
              <ThemedText themeColor="error" style={styles.centerText}>
                {error instanceof ConnectError && error.code === Code.NotFound ? t('plan.notFound') : t('error.network')}
              </ThemedText>
            )}
            <PrimaryButton
              label={p.signedUp ? t('plan.update') : t('plan.signUp')}
              disabled={times.pattern.length === 0}
              busy={signUp.isPending}
              onPress={() =>
                signUp.mutate(toSignup(times, usual), {
                  // Back to the plan, which useSignupActions has refreshed by
                  // now, so it shows the thread at "Signed up". Then ask for
                  // notifications: this is when they're worth having ("we'll
                  // tell you when there's a date").
                  onSuccess: () => {
                    haptics.success();
                    close();
                    void registerForPush({ ask: true });
                  },
                })
              }
            />
            <ThemedText type="small" themeColor="textSecondary" style={styles.centerText}>
              {t('plan.signUpHint')}
            </ThemedText>
          </View>
        </View>
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  center: { alignItems: 'center', justifyContent: 'center' },
  column: { width: '100%', maxWidth: FormWidth, alignSelf: 'center' },
  head: { flexDirection: 'row', alignItems: 'flex-start', gap: Spacing.three, padding: Spacing.three, paddingTop: Spacing.four },
  title: { fontSize: 22, lineHeight: 32 },
  close: { width: 44, height: 44, borderRadius: 22, alignItems: 'center', justifyContent: 'center' },
  closeMark: { fontSize: 24, lineHeight: 28 },
  content: { padding: Spacing.three, gap: Spacing.four },
  section: { gap: Spacing.three },
  wrap: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.two },
  footer: { padding: Spacing.three, gap: Spacing.two },
  centerText: { textAlign: 'center' },
});
