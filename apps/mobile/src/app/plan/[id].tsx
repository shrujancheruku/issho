import { Code, ConnectError, PlanLevel } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { useMemo, useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { AvailabilityGrid } from '@/components/availability-grid';
import { Chip } from '@/components/chip';
import { LanguageSwitch } from '@/components/language-switch';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Brand, FormWidth, Spacing } from '@/constants/theme';
import { useSignupActions, usePlan } from '@/features/plans/data';
import { formatDate, lastSignupDay } from '@/features/plans/format';
import { levelStyles } from '@/features/plans/level';
import { usePlanNames } from '@/features/plans/names';
import { distanceText, pairText } from '@/features/plans/plan-card';
import { initialTimes, toggle, toSignup, windowDates, type Times } from '@/features/plans/signup';
import { useProfile } from '@/features/profile/use-profile';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { slotKey } from '@/lib/slots';

/** A labelled detail in the plan's facts card: label above, value, optional note. */
function Fact({ label, value, note, last }: { label: string; value: string; note?: string; last?: boolean }) {
  const theme = useTheme();
  return (
    <View style={[styles.fact, !last && { borderBottomWidth: 1, borderBottomColor: theme.backgroundSelected }]}>
      <ThemedText type="small" themeColor="textSecondary">
        {label}
      </ThemedText>
      <ThemedText type="smallBold" style={styles.factValue}>
        {value}
      </ThemedText>
      {!!note && (
        <ThemedText type="small" themeColor="textSecondary">
          {note}
        </ThemedText>
      )}
    </View>
  );
}

/** One plan: what it is, how it works, and signing up with the times that suit you. */
export default function PlanScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t, languages, showing } = useI18n();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const theme = useTheme();
  const plan = usePlan(id);
  const profile = useProfile();
  const names = usePlanNames();
  const { signUp, withdraw } = useSignupActions(id);

  const usual = useMemo(
    () => (profile.data?.profile?.availability ?? []).map((s) => slotKey(s.weekday, s.block)),
    [profile.data],
  );
  // The saved times until the first edit.
  const [edited, setTimes] = useState<Times | null>(null);
  const saved = useMemo(
    () => (plan.data && profile.data ? initialTimes(plan.data.signup, usual) : null),
    [plan.data, profile.data, usual],
  );
  const times = edited ?? saved;

  // Opened directly (a link on the web), there's no screen to go back to.
  const leave = () => (router.canGoBack() ? router.back() : router.replace('/'));

  const notFound = plan.error instanceof ConnectError && plan.error.code === Code.NotFound;
  const p = plan.data?.plan;

  let body;
  if (notFound) {
    body = <ThemedText themeColor="textSecondary">{t('plan.notFound')}</ThemedText>;
  } else if (plan.isError || profile.isError) {
    body = (
      <Pressable onPress={() => void Promise.all([plan.refetch(), profile.refetch()])}>
        <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
        <ThemedText type="link">{t('common.retry')}</ThemedText>
      </Pressable>
    );
  } else if (!p || !times) {
    body = <ActivityIndicator />;
  } else {
    const activity = names.activity(p.activityId);
    const followUp = p.followUpActivityId ? names.activity(p.followUpActivityId) : null;
    const dates = windowDates(p.windowStart, p.windowDays);
    const level = levelStyles[p.level];
    const notForYou = p.level === PlanLevel.NOT_FOR_YOU;
    const language = [p.primaryLanguage, p.secondaryLanguage].filter(Boolean).map(names.language).join(' / ');
    body = (
      <>
        <View style={[styles.pill, { backgroundColor: level.background ?? theme.backgroundSelected }]}>
          <ThemedText type="small" style={{ color: level.text ?? theme.textSecondary }}>
            {t(level.label)}
          </ThemedText>
        </View>
        <View style={styles.titleBlock}>
          <ThemedText type="subtitle">{activity.name}</ThemedText>
          {!!activity.secondary && <ThemedText themeColor="textSecondary">{activity.secondary}</ThemedText>}
          {followUp && (
            <ThemedText style={styles.followUp}>{t('plan.followUp', { activity: pairText(followUp) })}</ThemedText>
          )}
        </View>
        {/* One labelled row per fact, so they read as separate details rather than a paragraph. */}
        <View style={[styles.facts, { backgroundColor: theme.backgroundElement }]}>
          <Fact label={t('plan.where')} value={pairText(names.area(p.areaId))} note={distanceText(t, p.travelMinutes)} />
          <Fact label={t('plan.language')} value={language} />
          <Fact
            label={t('plan.when')}
            value={t('plan.dateRange', { start: formatDate(dates[0], lang), end: formatDate(dates[dates.length - 1], lang) })}
            note={t('plan.whenHint')}
          />
          <Fact label={t('plan.signupBy')} value={lastSignupDay(p.signupDeadline, lang)} />
          <Fact label={t('plan.interestedLabel')} value={String(p.signups)} last />
        </View>

        {notForYou ? (
          <ThemedText>{t('plan.notForYouBody')}</ThemedText>
        ) : (
          <>
            <View style={styles.section}>
              <ThemedText type="smallBold">{t('plan.times')}</ThemedText>
              <ThemedText type="small" themeColor="textSecondary">
                {t('plan.timesHint')}
              </ThemedText>
              <AvailabilityGrid
                selected={times.pattern}
                onToggle={(day, b) => setTimes({ ...times, pattern: toggle(times.pattern, slotKey(day, b)) })}
              />
            </View>
            <View style={styles.section}>
              <ThemedText type="smallBold">{t('plan.blockOut')}</ThemedText>
              <ThemedText type="small" themeColor="textSecondary">
                {t('plan.blockOutHint')}
              </ThemedText>
              <View style={styles.wrap}>
                {dates.map((d) => (
                  <Chip
                    key={d}
                    label={formatDate(d, lang)}
                    selected={times.blocked.includes(d)}
                    selectedColors={{ background: theme.text, text: theme.background }}
                    onPress={() => setTimes({ ...times, blocked: toggle(times.blocked, d) })}
                  />
                ))}
              </View>
            </View>
          </>
        )}
      </>
    );
  }

  const signedUp = !!plan.data?.plan?.signedUp;
  const canSave = !!p && !!times && p.level !== PlanLevel.NOT_FOR_YOU;
  const error = signUp.error ?? withdraw.error;

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <View style={[styles.fill, styles.column]}>
          <View style={styles.header}>
            <Pressable accessibilityRole="button" onPress={leave} hitSlop={8} style={styles.grow}>
              <ThemedText type="link">‹ {t('common.back')}</ThemedText>
            </Pressable>
            <LanguageSwitch />
          </View>
          <ScrollView contentContainerStyle={styles.content}>{body}</ScrollView>
          {canSave && (
            <View style={styles.footer}>
              {times!.pattern.length === 0 && <ThemedText style={styles.error}>{t('plan.pickTime')}</ThemedText>}
              {error && (
                <ThemedText style={styles.error}>
                  {error instanceof ConnectError && error.code === Code.NotFound ? t('plan.notFound') : t('error.network')}
                </ThemedText>
              )}
              <PrimaryButton
                label={signedUp ? t('plan.update') : t('plan.signUp')}
                disabled={times!.pattern.length === 0}
                busy={signUp.isPending}
                onPress={() =>
                  signUp.mutate(toSignup(times!, usual), {
                    // Back to the feed, which useSignupActions has refreshed by
                    // now, so the plan already shows as signed up there.
                    onSuccess: leave,
                  })
                }
              />
              {signedUp && (
                <Pressable
                  accessibilityRole="button"
                  disabled={withdraw.isPending}
                  onPress={() => withdraw.mutate()}
                  style={styles.withdraw}>
                  <ThemedText type="link">{t('plan.withdraw')}</ThemedText>
                </Pressable>
              )}
            </View>
          )}
        </View>
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  grow: { flex: 1 },
  column: { width: '100%', maxWidth: FormWidth, alignSelf: 'center' },
  header: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: Spacing.four, paddingTop: Spacing.three },
  content: { padding: Spacing.four, gap: Spacing.four },
  pill: { alignSelf: 'flex-start', borderRadius: 999, paddingVertical: Spacing.half, paddingHorizontal: Spacing.two },
  titleBlock: { gap: Spacing.one },
  followUp: { marginTop: Spacing.one },
  facts: { borderRadius: 16, paddingHorizontal: Spacing.three },
  fact: { paddingVertical: Spacing.three, gap: Spacing.half },
  factValue: { fontSize: 16, lineHeight: 22 },
  section: { gap: Spacing.three },
  wrap: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.two },
  footer: { padding: Spacing.four, gap: Spacing.two },
  withdraw: { alignItems: 'center', paddingVertical: Spacing.two },
  error: { color: Brand.red, textAlign: 'center' },
});
