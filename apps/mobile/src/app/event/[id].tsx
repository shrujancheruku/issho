import { Code, ConnectError, EventStatus, InviteResponse, type GetEventResponse, type MyEvent } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { useState, type ReactNode } from 'react';
import { ActivityIndicator, KeyboardAvoidingView, Linking, ScrollView, Share, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Avatar } from '@/components/avatar';
import { LanguageSwitch } from '@/components/language-switch';
import { LiveDot } from '@/components/live-dot';
import { PrimaryButton } from '@/components/primary-button';
import { ResultButton } from '@/components/result-button';
import { RollingText } from '@/components/rolling-text';
import { SectionTitle } from '@/components/screen-header';
import { Segmented } from '@/components/segmented';
import { BackLink, TextLink } from '@/components/text-link';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Thread } from '@/components/thread';
import { FormWidth, Spacing } from '@/constants/theme';
import { ChatPanel } from '@/features/events/chat-panel';
import { useEvent, useRespond } from '@/features/events/data';
import { shareText } from '@/features/events/share';
import { outingStage } from '@/features/events/stage';
import { eventState, stateText, type EventState } from '@/features/events/state';
import { Fact, Facts } from '@/features/plans/fact';
import { formatStart, lastSignupDay, walkHours } from '@/features/plans/format';
import { usePlanNames } from '@/features/plans/names';
import { pairText } from '@/features/plans/plan-card';
import { PlanStops, StopsToggle } from '@/features/plans/plan-stops';
import { useNow } from '@/hooks/use-now';
import { useTheme } from '@/hooks/use-theme';
import { useI18n, useOtherLanguage } from '@/i18n/i18n-provider';
import { haptics } from '@/lib/haptics';

const ReportDays = 30; // as the server (plans.ReportFor)

type Names = ReturnType<typeof usePlanNames>;
type Ctx = { id: string; data: GetEventResponse; e: MyEvent; state: EventState; names: Names; now: number; lang: string };

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
 * One outing, one screen: the gold thread says where it stands and the one
 * thing to do sits right under it. Date set: can you go? Going: the group
 * chat, with the plan and meeting point beside it. On the day: the
 * countdown and "I'm here". Afterwards: how was it.
 */
export default function EventScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t, languages, showing } = useI18n();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const event = useEvent(id);
  const names = usePlanNames();
  // Every 15 s: the countdown on the day is in minutes.
  const now = useNow(15_000);
  const leave = () => (router.canGoBack() ? router.back() : router.replace('/'));
  const [tab, setTab] = useState<'chat' | 'plan'>('chat');

  const e = event.data?.event;
  const notFound = event.error instanceof ConnectError && event.error.code === Code.NotFound;

  if (!e || !event.data) {
    return (
      <Frame onBack={leave}>
        <ScrollView contentContainerStyle={styles.content}>
          {notFound ? (
            <ThemedText themeColor="textSecondary">{t('event.notFound')}</ThemedText>
          ) : event.isError ? (
            <>
              <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
              <TextLink label={t('common.retry')} onPress={() => void event.refetch()} />
            </>
          ) : (
            <ActivityIndicator />
          )}
        </ScrollView>
      </Frame>
    );
  }

  const state = eventState(e, now);
  const stage = outingStage(e, now);
  const ctx: Ctx = { id, data: event.data, e, state, names, now, lang };
  const muted = state.tone === 'muted' && stage !== 4;
  // On the day the live dot by the time is the screen's one live thing.
  const thread = <Thread stage={stage} id={`event:${id}`} live={!muted && stage !== 3} />;

  // Going: the chat fills the screen under a fixed head, so it can't scroll with the rest.
  if (stage === 2 && !muted) {
    return (
      <Frame onBack={leave}>
        <View style={styles.head}>
          <Title e={e} detail={`${formatStart(e.startsAt, lang)}, ${names.area(e.areaId).name}`} />
          {thread}
          <Segmented
            label={t('event.tabs')}
            value={tab}
            onChange={setTab}
            options={[
              { value: 'chat', label: t('chat.title') },
              { value: 'plan', label: t('event.tabPlan') },
            ]}
          />
        </View>
        {tab === 'chat' ? (
          <ChatPanel target={{ eventId: id }} header={<Pinned ctx={ctx} />} />
        ) : (
          <ScrollView contentContainerStyle={styles.content}>
            <PlanTab ctx={ctx} />
          </ScrollView>
        )}
      </Frame>
    );
  }

  return (
    <Frame onBack={leave}>
      <ScrollView contentContainerStyle={styles.content}>
        <Title e={e} />
        {thread}
        {muted ? <Muted ctx={ctx} /> : stage === 1 ? <Invited ctx={ctx} /> : stage === 3 ? <OnTheDay ctx={ctx} /> : <HowWasIt ctx={ctx} />}
      </ScrollView>
    </Frame>
  );
}

function Frame({ onBack, children }: { onBack: () => void; children: ReactNode }) {
  const { t } = useI18n();
  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <KeyboardAvoidingView behavior="padding" style={[styles.fill, styles.column]}>
          <View style={styles.header}>
            <BackLink label={t('common.back')} onPress={onBack} />
            <LanguageSwitch />
          </View>
          {children}
        </KeyboardAvoidingView>
      </SafeAreaView>
    </ThemedView>
  );
}

function Title({ e, detail }: { e: MyEvent; detail?: string }) {
  const { showing } = useI18n();
  const otherLanguage = useOtherLanguage();
  const [name, secondary] = showing === 'secondary' && e.secondaryTitle ? [e.secondaryTitle, e.title] : [e.title, e.secondaryTitle];
  return (
    <View style={styles.titleBlock}>
      <ThemedText type="subtitle" accessibilityRole="header">
        {name}
      </ThemedText>
      {!!secondary && !detail && (
        <ThemedText themeColor="textSecondary" lang={otherLanguage}>
          {secondary}
        </ThemedText>
      )}
      {!!detail && (
        <ThemedText type="smallBold" themeColor="link">
          {detail}
        </ThemedText>
      )}
    </View>
  );
}

function Card({ children }: { children: ReactNode }) {
  const theme = useTheme();
  return <View style={[styles.card, { backgroundColor: theme.surface }]}>{children}</View>;
}

function meetingPoint({ e, now, lang }: Ctx, t: ReturnType<typeof useI18n>['t']) {
  return (
    e.meetingPoint ||
    (e.revealAt && now < Number(e.revealAt.seconds) * 1000
      ? t('event.meetingPointLater', { date: formatStart(e.revealAt, lang) })
      : t('event.meetingPointFirstStop'))
  );
}

/** Stage two, invited: can you go? The answer becomes its result in place. */
function Invited({ ctx }: { ctx: Ctx }) {
  const { t } = useI18n();
  const { e, lang, names, data } = ctx;
  const respond = useRespond(ctx.id, { refreshAfter: 1400 });
  const tooLate = respond.error instanceof ConnectError && respond.error.code === Code.FailedPrecondition;
  const going = respond.isSuccess && respond.variables === 'confirm';
  return (
    <>
      <Card>
        <ThemedText type="smallBold" themeColor="goldText">
          {t('event.state.answer', { date: lastSignupDay(e.confirmBy, lang) })}
        </ThemedText>
        <ThemedText type="subtitle" style={styles.cardTitle}>
          {formatStart(e.startsAt, lang)}
        </ThemedText>
        <ThemedText>{pairText(names.area(e.areaId))}</ThemedText>
        <ThemedText type="small" themeColor="textSecondary" style={styles.gapBelow}>
          {t('event.confirmedCount', { count: e.confirmed })}
        </ThemedText>
        {respond.isError && <ThemedText themeColor="error">{tooLate ? t('event.tooLate') : t('error.network')}</ThemedText>}
        <ResultButton
          label={t('event.confirm')}
          doneLabel={t('event.state.going')}
          done={going}
          busy={respond.isPending && respond.variables === 'confirm'}
          onPress={() => respond.mutate('confirm', { onSuccess: haptics.success })}
        />
        {!going && <TextLink label={t('event.decline')} disabled={respond.isPending} onPress={() => respond.mutate('decline')} />}
      </Card>
      <View style={styles.section}>
        <SectionTitle>{t('event.ifYouGo')}</SectionTitle>
        <ThemedText themeColor="textSecondary">{t('event.ifYouGoBody')}</ThemedText>
      </View>
      <StopsToggle steps={data.steps} routePolyline={data.routePolyline} names={names} />
    </>
  );
}

/** Pinned above the group chat: where to meet and who is going. */
function Pinned({ ctx }: { ctx: Ctx }) {
  const { t } = useI18n();
  const theme = useTheme();
  const attendees = ctx.data.attendees;
  return (
    <View style={[styles.pinned, { backgroundColor: theme.surface }]}>
      <ThemedText type="smallBold">
        {t('event.meetingPoint')}: {meetingPoint(ctx, t)}
      </ThemedText>
      <ThemedText type="small" themeColor="textSecondary">
        {t('event.going')}: {attendees.length ? attendees.join(', ') : t('event.confirmedCount', { count: ctx.e.confirmed })}
      </ThemedText>
    </View>
  );
}

/** The plan beside the chat: facts, the stops, and the safety and account actions. */
function PlanTab({ ctx }: { ctx: Ctx }) {
  const { t } = useI18n();
  const { e, data, names, lang } = ctx;
  const respond = useRespond(ctx.id);
  const hours = walkHours(data.steps, lang);
  const language = [e.primaryLanguage, e.secondaryLanguage].filter(Boolean).map(names.language).join(' / ');
  const summary = useSummary(data);
  return (
    <>
      <ThemedText type="smallBold" themeColor={ctx.state.tone === 'good' ? 'link' : 'textSecondary'}>
        {stateText(t, ctx.state, e, lang)}
      </ThemedText>
      {!!summary && <ThemedText style={styles.summary}>{summary}</ThemedText>}
      <Facts>
        <Fact label={t('plan.when')} value={formatStart(e.startsAt, lang)} />
        {!!hours && <Fact label={t('plan.length')} value={t('plan.lengthHours', { hours })} />}
        <Fact label={t('plan.where')} value={pairText(names.area(e.areaId))} />
        <Fact label={t('event.meetingPoint')} value={meetingPoint(ctx, t)} />
        <Fact label={t('plan.language')} value={language} />
        <Fact label={t('event.going')} value={data.attendees.join(', ') || t('event.confirmedCount', { count: e.confirmed })} />
      </Facts>
      <PlanStops steps={data.steps} routePolyline={data.routePolyline} names={names} />
      <SafetyLinks ctx={ctx} />
      {ctx.state.actions.includes('cancel') && (
        <>
          {respond.isError && <ThemedText themeColor="error">{t('error.network')}</ThemedText>}
          <TextLink label={t('event.cancel')} muted disabled={respond.isPending} onPress={() => respond.mutate('cancel')} />
        </>
      )}
    </>
  );
}

function useSummary(data: GetEventResponse) {
  const { showing } = useI18n();
  return showing === 'secondary' && data.secondarySummary ? data.secondarySummary : data.summary;
}

/** Share with someone you trust (a safety habit) and report someone: plain links, no motion. */
function SafetyLinks({ ctx }: { ctx: Ctx }) {
  const { t } = useI18n();
  const { e, data, names, now, lang } = ctx;
  const canShare = e.response === InviteResponse.CONFIRMED && e.status === EventStatus.SCHEDULED;
  // Anyone who confirmed, even if they cancelled since, until 30 days after the start.
  const canReport =
    (e.response === InviteResponse.CONFIRMED || e.response === InviteResponse.DROPPED) &&
    (!e.startsAt || now <= Number(e.startsAt.seconds) * 1000 + ReportDays * 86_400_000);
  return (
    <View>
      {canShare && (
        <TextLink
          label={t('event.share')}
          onPress={() =>
            void shareOuting(shareText(e, formatStart(e.startsAt, lang), data.steps, pairText(names.area(e.areaId)), t), e.title)
          }
        />
      )}
      {canReport && (
        <TextLink label={t('event.report')} muted onPress={() => router.push({ pathname: '/report/[id]', params: { id: e.id } })} />
      )}
    </View>
  );
}

/** Stage four, on the day: the countdown, where to meet, and "I'm here". */
function OnTheDay({ ctx }: { ctx: Ctx }) {
  const { t } = useI18n();
  const { e, data, now, lang } = ctx;
  const respond = useRespond(ctx.id);
  const start = e.startsAt ? Number(e.startsAt.seconds) * 1000 : NaN;
  const minutes = Math.ceil((start - now) / 60_000);
  const mapsUrl = data.steps.find((s) => s.venueMapsUrl)?.venueMapsUrl;
  const canCheckIn = ctx.state.actions.includes('checkIn') || e.checkedIn;
  return (
    <>
      <Card>
        <View style={styles.liveRow}>
          <LiveDot />
          <ThemedText type="small" themeColor="textSecondary">
            {formatStart(e.startsAt, lang)}
          </ThemedText>
        </View>
        {minutes > 0 && minutes <= 180 && <RollingText value={t('event.startsIn', { minutes })} />}
        <ThemedText style={styles.bold}>
          {t('event.meetingPoint')}: {meetingPoint(ctx, t)}
        </ThemedText>
        <View style={styles.gapAbove}>
          {respond.isError && <ThemedText themeColor="error">{t('error.network')}</ThemedText>}
          {canCheckIn ? (
            <ResultButton
              label={t('event.checkIn')}
              doneLabel={t('event.state.checkedIn')}
              done={e.checkedIn || (respond.isSuccess && respond.variables === 'checkIn')}
              busy={respond.isPending}
              onPress={() => respond.mutate('checkIn', { onSuccess: haptics.success })}
            />
          ) : (
            <ThemedText type="small" themeColor="textSecondary">
              {t('event.checkInLater')}
            </ThemedText>
          )}
        </View>
        {!!mapsUrl && <TextLink label={t('plan.openInMaps')} onPress={() => void Linking.openURL(mapsUrl)} />}
      </Card>
      <View style={styles.section}>
        <SectionTitle>{t('event.going')}</SectionTitle>
        <View style={styles.people}>
          {data.attendees.map((name, i) => (
            <View key={`${name}-${i}`} style={styles.person}>
              <Avatar name={name} size={40} />
              <ThemedText type="small" numberOfLines={1}>
                {name}
              </ThemedText>
            </View>
          ))}
        </View>
        <TextLink label={t('event.openChat')} onPress={() => router.push({ pathname: '/chat/[id]', params: { id: e.id } })} />
      </View>
      <SafetyLinks ctx={ctx} />
    </>
  );
}

/** Stage five: how was it, then who you'd meet again. */
function HowWasIt({ ctx }: { ctx: Ctx }) {
  const { t } = useI18n();
  const { e } = ctx;
  const open = () => router.push({ pathname: '/feedback/[id]', params: { id: e.id } });
  const feedbackOpen = ctx.state.actions.includes('feedback');
  return (
    <>
      <Card>
        <ThemedText type="subtitle" style={styles.cardTitle}>
          {e.feedbackGiven ? t('event.state.feedbackGiven') : t('feedback.title')}
        </ThemedText>
        {!e.feedbackGiven && <ThemedText themeColor="textSecondary">{t('event.state.feedback')}</ThemedText>}
        {feedbackOpen &&
          (e.feedbackGiven ? (
            <TextLink label={t('event.feedbackEdit')} onPress={open} />
          ) : (
            <View style={styles.gapAbove}>
              <PrimaryButton label={t('event.feedback')} onPress={open} />
            </View>
          ))}
      </Card>
      <SafetyLinks ctx={ctx} />
    </>
  );
}

/** Over, out or not happening: says so plainly, with any answer still open. */
function Muted({ ctx }: { ctx: Ctx }) {
  const { t } = useI18n();
  const respond = useRespond(ctx.id);
  return (
    <>
      <Card>
        <ThemedText themeColor="textSecondary">{stateText(t, ctx.state, ctx.e, ctx.lang)}</ThemedText>
        {ctx.state.actions.includes('confirm') && (
          <View style={styles.gapAbove}>
            <PrimaryButton label={t('event.confirm')} busy={respond.isPending} onPress={() => respond.mutate('confirm')} />
          </View>
        )}
      </Card>
      <SafetyLinks ctx={ctx} />
    </>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  bold: { fontWeight: 700 },
  column: { width: '100%', maxWidth: FormWidth, alignSelf: 'center' },
  header: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: Spacing.three },
  head: { paddingHorizontal: Spacing.three, paddingBottom: Spacing.two, gap: Spacing.three },
  content: { padding: Spacing.three, paddingTop: Spacing.two, gap: Spacing.four - 4, paddingBottom: Spacing.five },
  titleBlock: { gap: Spacing.half },
  card: { borderRadius: 16, padding: Spacing.three, gap: Spacing.one },
  cardTitle: { fontSize: 24, lineHeight: 35 },
  gapBelow: { marginBottom: Spacing.two },
  gapAbove: { marginTop: Spacing.two, gap: Spacing.two },
  section: { gap: Spacing.two },
  summary: { lineHeight: 26 },
  pinned: { borderRadius: 12, padding: Spacing.three - 4, gap: Spacing.half, marginBottom: Spacing.two },
  liveRow: { flexDirection: 'row', alignItems: 'center', gap: Spacing.two },
  people: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.two },
  person: { width: 56, alignItems: 'center', gap: Spacing.one },
});
