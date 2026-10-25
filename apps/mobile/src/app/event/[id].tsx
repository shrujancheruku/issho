import { Code, ConnectError, EventStatus, InviteResponse } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { ActivityIndicator, Linking, Pressable, ScrollView, Share, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { LanguageSwitch } from '@/components/language-switch';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { FormWidth, Spacing } from '@/constants/theme';
import { useEvent, useRespond } from '@/features/events/data';
import { stateText, toneColor } from '@/features/events/event-card';
import { shareText } from '@/features/events/share';
import { eventState, type EventAction } from '@/features/events/state';
import { Fact, Facts, Heading } from '@/features/plans/fact';
import { formatStart, walkHours } from '@/features/plans/format';
import { usePlanNames } from '@/features/plans/names';
import { pairText } from '@/features/plans/plan-card';
import { RouteMap } from '@/features/plans/route-map';
import { Timeline } from '@/features/plans/timeline';
import { useNow } from '@/hooks/use-now';
import { useTheme } from '@/hooks/use-theme';
import { useI18n, useOtherLanguage } from '@/i18n/i18n-provider';

const ReportDays = 30; // as the server (plans.ReportFor)

/**
 * Opens the share sheet with the outing's details. Browsers without the
 * Web Share API (most desktop ones) get an email draft instead.
 */
async function shareOuting(message: string, title: string) {
  try {
    await Share.share({ message, title });
  } catch {
    void Linking.openURL(`mailto:?subject=${encodeURIComponent(title)}&body=${encodeURIComponent(message)}`);
  }
}

/**
 * One of the person's events: where they stand and what they can do
 * (confirm, decline, cancel), when and where it is, the stops, and who's
 * going (only once they've confirmed).
 */
export default function EventScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t, languages, showing } = useI18n();
  const otherLanguage = useOtherLanguage();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const theme = useTheme();
  const event = useEvent(id);
  const names = usePlanNames();
  const respond = useRespond(id);
  const leave = () => (router.canGoBack() ? router.back() : router.replace('/'));

  const e = event.data?.event;
  const notFound = event.error instanceof ConnectError && event.error.code === Code.NotFound;
  const now = useNow();
  const state = e ? eventState(e, now) : null;

  let body;
  if (notFound) {
    body = <ThemedText themeColor="textSecondary">{t('event.notFound')}</ThemedText>;
  } else if (event.isError) {
    body = (
      <Pressable onPress={() => void event.refetch()}>
        <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
        <ThemedText type="link">{t('common.retry')}</ThemedText>
      </Pressable>
    );
  } else if (!e || !state) {
    body = <ActivityIndicator />;
  } else {
    const color = toneColor(state.tone, theme);
    const title = showing === 'secondary' && e.secondaryTitle ? { name: e.secondaryTitle, secondary: e.title } : { name: e.title, secondary: e.secondaryTitle };
    const language = [e.primaryLanguage, e.secondaryLanguage].filter(Boolean).map(names.language).join(' / ');
    const attendees = event.data?.attendees ?? [];
    const summary =
      showing === 'secondary' && event.data?.secondarySummary ? event.data.secondarySummary : (event.data?.summary ?? '');
    const hours = walkHours(event.data?.steps ?? [], lang);
    body = (
      <>
        <View style={[styles.banner, { borderColor: color }]}>
          <ThemedText type="smallBold" style={{ color }}>
            {stateText(t, state, e, lang)}
          </ThemedText>
        </View>
        <View style={styles.titleBlock}>
          <ThemedText type="subtitle">{title.name}</ThemedText>
          {!!title.secondary && <ThemedText themeColor="textSecondary" lang={otherLanguage}>{title.secondary}</ThemedText>}
        </View>
        {!!summary && <ThemedText>{summary}</ThemedText>}
        <RouteMap steps={event.data?.steps ?? []} routePolyline={event.data?.routePolyline ?? ''} />
        <Timeline steps={event.data?.steps ?? []} names={names} />
        <Facts>
          <Fact label={t('plan.when')} value={formatStart(e.startsAt, lang)} />
          {!!hours && <Fact label={t('plan.length')} value={t('plan.lengthHours', { hours })} />}
          <Fact label={t('plan.where')} value={pairText(names.area(e.areaId))} />
          <Fact
            label={t('event.meetingPoint')}
            value={
              e.meetingPoint ||
              (e.revealAt && now < Number(e.revealAt.seconds) * 1000
                ? t('event.meetingPointLater', { date: formatStart(e.revealAt, lang) })
                : t('event.meetingPointFirstStop'))
            }
          />
          <Fact label={t('plan.language')} value={language} />
        </Facts>
        <View style={styles.section}>
          <Heading
            title={t('event.going')}
            hint={attendees.length ? attendees.join(', ') : t('event.goingHidden')}
          />
          <ThemedText type="small" themeColor="textSecondary">
            {t('event.confirmedCount', { count: e.confirmed })}
          </ThemedText>
        </View>
        {/* A safety habit: tell someone outside the app where you'll be. */}
        {e.response === InviteResponse.CONFIRMED && e.status === EventStatus.SCHEDULED && (
          <Pressable
            accessibilityRole="button"
            onPress={() =>
              void shareOuting(
                shareText(e, formatStart(e.startsAt, lang), event.data?.steps ?? [], pairText(names.area(e.areaId)), t),
                e.title,
              )
            }
            style={styles.share}>
            <ThemedText type="link">{t('event.share')}</ThemedText>
          </Pressable>
        )}
        {/* The chat is for people going; it says itself when it opens. */}
        {e.response === InviteResponse.CONFIRMED && e.status !== EventStatus.CANCELLED && (
          <Pressable
            accessibilityRole="button"
            onPress={() => router.push({ pathname: '/chat/[id]', params: { id: e.id } })}
            style={[styles.chatButton, { borderColor: theme.backgroundSelected }]}>
            <ThemedText type="smallBold">{t('chat.title')} ›</ThemedText>
          </Pressable>
        )}
        {/* Anyone who confirmed, even if they cancelled since, until 30 days after the start. */}
        {(e.response === InviteResponse.CONFIRMED || e.response === InviteResponse.DROPPED) &&
          (!e.startsAt || now <= Number(e.startsAt.seconds) * 1000 + ReportDays * 86_400_000) && (
            <Pressable
              accessibilityRole="button"
              onPress={() => router.push({ pathname: '/report/[id]', params: { id: e.id } })}
              style={styles.share}>
              <ThemedText type="link" themeColor="textSecondary">
                {t('event.report')}
              </ThemedText>
            </Pressable>
          )}
      </>
    );
  }

  const labels: Record<EventAction, string> = {
    confirm: t('event.confirm'),
    decline: t('event.decline'),
    cancel: t('event.cancel'),
    checkIn: t('event.checkIn'),
    feedback: e?.feedbackGiven ? t('event.feedbackEdit') : t('event.feedback'),
  };
  // Feedback is its own screen; everything else is an answer sent from here.
  const act = (a: EventAction) =>
    a === 'feedback' ? router.push({ pathname: '/feedback/[id]', params: { id } }) : respond.mutate(a);
  const error = respond.error;

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
          {!!state?.actions.length && (
            <View style={styles.footer}>
              {error && (
                <ThemedText themeColor="error" style={styles.error}>
                  {error instanceof ConnectError && error.code === Code.FailedPrecondition
                    ? t('event.tooLate')
                    : t('error.network')}
                </ThemedText>
              )}
              {state.actions.map((a) =>
                // The first action is the main one (confirm); the others are links.
                a === state.actions[0] && a !== 'cancel' ? (
                  <PrimaryButton key={a} label={labels[a]} busy={respond.isPending && respond.variables === a} onPress={() => act(a)} />
                ) : (
                  <Pressable
                    key={a}
                    accessibilityRole="button"
                    disabled={respond.isPending}
                    onPress={() => act(a)}
                    style={styles.secondary}>
                    <ThemedText type="link">{labels[a]}</ThemedText>
                  </Pressable>
                ),
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
  banner: { borderLeftWidth: 3, paddingLeft: Spacing.three, paddingVertical: Spacing.one },
  titleBlock: { gap: Spacing.one },
  section: { gap: Spacing.two },
  chatButton: { borderWidth: 1, borderRadius: 12, padding: Spacing.three, alignItems: 'center' },
  share: { alignItems: 'center', paddingVertical: Spacing.one },
  footer: { padding: Spacing.four, gap: Spacing.two },
  secondary: { alignItems: 'center', paddingVertical: Spacing.two },
  error: { textAlign: 'center' },
});
