import { Code, ConnectError } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Chip } from '@/components/chip';
import { LanguageSwitch } from '@/components/language-switch';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Brand, FormWidth, Spacing } from '@/constants/theme';
import { useEvent, useFeedback, useSubmitFeedback, type FeedbackAnswer } from '@/features/events/data';
import { Heading } from '@/features/plans/fact';
import { formatStart } from '@/features/plans/format';
import { useNow } from '@/hooks/use-now';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';

const ratings: [number, StringKey][] = [
  [1, 'feedback.rating1'],
  [2, 'feedback.rating2'],
  [3, 'feedback.rating3'],
  [4, 'feedback.rating4'],
  [5, 'feedback.rating5'],
];

/** Five rating chips, worded rather than stars so each reads the same in every language. */
function Rating({ value, onChange }: { value: number; onChange: (v: number) => void }) {
  const { t } = useI18n();
  return (
    <View style={styles.wrap}>
      {ratings.map(([v, key]) => (
        <Chip key={v} label={t(key)} selected={value === v} onPress={() => onChange(v)} />
      ))}
    </View>
  );
}

/**
 * After an outing: did you go, how were the plan and the group, and who
 * would you meet again (private; only mutual picks count). Sending again
 * replaces the earlier answer.
 */
export default function FeedbackScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t, languages, showing } = useI18n();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const now = useNow();
  const theme = useTheme();
  const form = useFeedback(id);
  const event = useEvent(id);
  const submit = useSubmitFeedback(id);
  // The person's changes; until they make one, the earlier answer (if any).
  const [draft, setAnswer] = useState<FeedbackAnswer | null>(null);
  const leave = () => (router.canGoBack() ? router.back() : router.replace('/'));

  const f = form.data;
  const answer: FeedbackAnswer =
    draft ??
    (f?.given
      ? { attended: f.attended, planRating: f.planRating, groupRating: f.groupRating, picks: f.picks, avoid: f.avoid }
      : { attended: true, planRating: 0, groupRating: 0, picks: [], avoid: [] });

  const e = event.data?.event;
  const title = e ? (showing === 'secondary' && e.secondaryTitle ? e.secondaryTitle : e.title) : '';
  const denied = form.error instanceof ConnectError && form.error.code === Code.PermissionDenied;
  const rated = answer.planRating > 0 && answer.groupRating > 0;
  const mates = f?.mates.filter((m) => !m.cancelled) ?? [];
  // Someone is in at most one of the two lists: choosing one clears the other.
  const toggle = (list: 'picks' | 'avoid', mate: string) => {
    const other = list === 'picks' ? 'avoid' : 'picks';
    const on = answer[list].includes(mate);
    setAnswer({
      ...answer,
      [list]: on ? answer[list].filter((p) => p !== mate) : [...answer[list], mate],
      [other]: answer[other].filter((p) => p !== mate),
    });
  };

  let body;
  if (denied) {
    body = <ThemedText themeColor="textSecondary">{t('feedback.notOpen')}</ThemedText>;
  } else if (form.isError) {
    body = (
      <Pressable onPress={() => void form.refetch()}>
        <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
        <ThemedText type="link">{t('common.retry')}</ThemedText>
      </Pressable>
    );
  } else if (!f) {
    body = <ActivityIndicator />;
  } else if (!f.open) {
    // Before it opens, after two weeks, or for someone whose group didn't go ahead.
    body = (
      <ThemedText themeColor="textSecondary">
        {!f.opensAt
          ? t('feedback.notOpen')
          : now < Number(f.opensAt.seconds) * 1000
            ? t('feedback.opensOn', { date: formatStart(f.opensAt, lang) })
            : t('feedback.closed')}
      </ThemedText>
    );
  } else {
    body = (
      <>
        <View style={styles.section}>
          <ThemedText type="smallBold">{t('feedback.attended')}</ThemedText>
          <View style={styles.wrap}>
            <Chip label={t('feedback.went')} selected={answer.attended} onPress={() => setAnswer({ ...answer, attended: true })} />
            <Chip label={t('feedback.didNotGo')} selected={!answer.attended} onPress={() => setAnswer({ ...answer, attended: false })} />
          </View>
        </View>
        {answer.attended && (
          <>
            <View style={styles.section}>
              <Heading title={t('feedback.plan')} hint={t('feedback.planHint')} />
              <Rating value={answer.planRating} onChange={(planRating) => setAnswer({ ...answer, planRating })} />
            </View>
            <View style={styles.section}>
              <Heading title={t('feedback.group')} hint={t('feedback.groupHint')} />
              <Rating value={answer.groupRating} onChange={(groupRating) => setAnswer({ ...answer, groupRating })} />
            </View>
            <View style={styles.section}>
              <Heading title={t('feedback.meetAgain')} hint={t('feedback.meetAgainHint')} />
              {mates.length ? (
                <View style={styles.wrap}>
                  {mates.map((m) => (
                    <Chip key={m.id} label={m.name} selected={answer.picks.includes(m.id)} onPress={() => toggle('picks', m.id)} />
                  ))}
                </View>
              ) : (
                <ThemedText type="small" themeColor="textSecondary">
                  {t('feedback.nobody')}
                </ThemedText>
              )}
            </View>
            {mates.length > 0 && (
              <View style={styles.section}>
                <Heading title={t('feedback.rather')} hint={t('feedback.ratherHint')} />
                <View style={styles.wrap}>
                  {mates.map((m) => (
                    <Chip
                      key={m.id}
                      label={m.name}
                      selected={answer.avoid.includes(m.id)}
                      // Grey, not red: a quiet "no", not an alarm.
                      selectedColors={{ background: theme.text, text: theme.background }}
                      onPress={() => toggle('avoid', m.id)}
                    />
                  ))}
                </View>
              </View>
            )}
          </>
        )}
      </>
    );
  }

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
          <ScrollView contentContainerStyle={styles.content}>
            <View style={styles.titleBlock}>
              <ThemedText type="subtitle">{t('feedback.title')}</ThemedText>
              {!!title && <ThemedText themeColor="textSecondary">{title}</ThemedText>}
            </View>
            {body}
            {/* Reporting is its own screen: also open to people who cancelled. */}
            {!!f?.mates.length && (
              <Pressable
                accessibilityRole="button"
                onPress={() => router.push({ pathname: '/report/[id]', params: { id } })}
                style={styles.link}>
                <ThemedText type="link">{t('event.report')}</ThemedText>
              </Pressable>
            )}
          </ScrollView>
          {f?.open && (
            <View style={styles.footer}>
              {submit.isError ? (
                <ThemedText style={styles.error}>
                  {submit.error instanceof ConnectError && submit.error.code === Code.FailedPrecondition
                    ? t('feedback.closed')
                    : t('error.network')}
                </ThemedText>
              ) : (
                answer.attended &&
                !rated && (
                  <ThemedText type="small" themeColor="textSecondary" style={styles.center}>
                    {t('feedback.ratingsNeeded')}
                  </ThemedText>
                )
              )}
              <PrimaryButton
                label={t('feedback.send')}
                disabled={answer.attended && !rated}
                busy={submit.isPending}
                onPress={() => submit.mutate(answer.attended ? answer : { attended: false, planRating: 0, groupRating: 0, picks: [], avoid: [] }, { onSuccess: leave })}
              />
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
  titleBlock: { gap: Spacing.one },
  section: { gap: Spacing.two },
  wrap: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.two },
  link: { alignItems: 'center', paddingVertical: Spacing.one },
  footer: { padding: Spacing.four, gap: Spacing.two },
  error: { color: Brand.red, textAlign: 'center' },
  center: { textAlign: 'center' },
});
