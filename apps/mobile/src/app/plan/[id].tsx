import { Code, ConnectError, PlanClosedReason, PlanLevel, type GetPlanResponse } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { ActivityIndicator, ScrollView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { weekdayName, blockKeys } from '@/components/availability-grid';
import { LanguageSwitch } from '@/components/language-switch';
import { PrimaryButton } from '@/components/primary-button';
import { BackLink, TextLink } from '@/components/text-link';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Thread } from '@/components/thread';
import { FormWidth, Spacing } from '@/constants/theme';
import { useSignupActions } from '@/features/plans/data';
import { Fact, Facts } from '@/features/plans/fact';
import { formatDate, lastSignupDay, walkHours } from '@/features/plans/format';
import { levelColors, levelLabels } from '@/features/plans/level';
import { usePlanNames } from '@/features/plans/names';
import { distanceText, friendsText, pairText, planTitle } from '@/features/plans/plan-card';
import { PlanStops, StopsToggle } from '@/features/plans/plan-stops';
import { windowDates, type Times } from '@/features/plans/signup';
import { usePlanTimes } from '@/features/plans/use-plan-times';
import { useTheme } from '@/hooks/use-theme';
import { useI18n, useOtherLanguage } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { parseSlotKey } from '@/lib/slots';

/** Why a plan closed, by reason; open plans have none. */
const closedText: Partial<Record<PlanClosedReason, StringKey>> = {
  [PlanClosedReason.NO_GROUP]: 'plan.closed.noGroup',
  [PlanClosedReason.DONE]: 'plan.closed.done',
  [PlanClosedReason.WITHDRAWN]: 'plan.closed.withdrawn',
};

type Names = ReturnType<typeof usePlanNames>;

const openSignup = (id: string) => router.push({ pathname: '/signup/[id]', params: { id } });

/**
 * One plan. Before signing up: what it is, the facts, the map and stops,
 * and "Sign me up", which opens the times sheet. Once signed up, it's the
 * first stage of the outing: the thread at "Signed up", the times chosen,
 * and the details behind a link.
 */
export default function PlanScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t } = useI18n();
  const { plan, profile, saved } = usePlanTimes(id);
  const names = usePlanNames();
  // Opened directly (a link on the web), there's no screen to go back to.
  const leave = () => (router.canGoBack() ? router.back() : router.replace('/'));

  const notFound = plan.error instanceof ConnectError && plan.error.code === Code.NotFound;
  const p = plan.data?.plan;
  const closed = p ? closedText[p.closedReason] : undefined;

  let body;
  let footer = null;
  if (notFound) {
    body = <ThemedText themeColor="textSecondary">{t('plan.notFound')}</ThemedText>;
  } else if (plan.isError || profile.isError) {
    body = (
      <>
        <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
        <TextLink label={t('common.retry')} onPress={() => void Promise.all([plan.refetch(), profile.refetch()])} />
      </>
    );
  } else if (!p || !saved || !plan.data) {
    body = <ActivityIndicator />;
  } else if (p.signedUp && !closed) {
    body = <SignedUp data={plan.data} times={saved} names={names} />;
  } else {
    body = <Before data={plan.data} names={names} closed={closed} loading={plan.isPlaceholderData} />;
    if (closed) {
      footer = <PrimaryButton label={t('plan.closed.others')} onPress={() => router.navigate('/plans')} />;
    } else if (p.level !== PlanLevel.NOT_FOR_YOU) {
      footer = (
        <>
          <PrimaryButton label={t('plan.signUp')} onPress={() => openSignup(id)} />
          <ThemedText type="small" themeColor="textSecondary" style={styles.centerText}>
            {t('plan.next')}
          </ThemedText>
        </>
      );
    }
  }

  return <PlanFrame onBack={leave} body={body} footer={footer} />;
}

function PlanFrame({ onBack, body, footer }: { onBack: () => void; body: React.ReactNode; footer: React.ReactNode }) {
  const { t } = useI18n();
  const theme = useTheme();
  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <View style={[styles.fill, styles.column]}>
          <View style={styles.header}>
            <BackLink label={t('common.back')} onPress={onBack} />
            <LanguageSwitch />
          </View>
          <ScrollView contentContainerStyle={styles.content}>{body}</ScrollView>
          {footer && <View style={[styles.footer, { backgroundColor: theme.surface, borderTopColor: theme.backgroundSelected }]}>{footer}</View>}
        </View>
      </SafeAreaView>
    </ThemedView>
  );
}

function useLang() {
  const { languages, showing } = useI18n();
  return showing === 'secondary' ? languages.secondary : languages.primary;
}

function TitleBlock({ data, size = 'large' }: { data: GetPlanResponse; size?: 'large' | 'medium' }) {
  const { showing } = useI18n();
  const otherLanguage = useOtherLanguage();
  const title = planTitle(data.plan!, showing);
  return (
    <View style={styles.titleBlock}>
      <ThemedText type="subtitle" style={size === 'large' ? styles.titleLarge : undefined} accessibilityRole="header">
        {title.name}
      </ThemedText>
      {!!title.secondary && (
        <ThemedText themeColor="textSecondary" lang={otherLanguage}>
          {title.secondary}
        </ThemedText>
      )}
    </View>
  );
}

/** The plan before signing up (or closed, or not in a language the person speaks). */
function Before({ data, names, closed, loading }: { data: GetPlanResponse; names: Names; closed?: StringKey; loading?: boolean }) {
  const { t, showing } = useI18n();
  const theme = useTheme();
  const lang = useLang();
  const p = data.plan!;
  const summary = showing === 'secondary' && data.secondarySummary ? data.secondarySummary : data.summary;
  const hours = walkHours(data.steps, lang);
  const dates = windowDates(p.windowStart, p.windowDays);
  const colors = levelColors(p.level, theme);
  const language = [p.primaryLanguage, p.secondaryLanguage].filter(Boolean).map(names.language).join(' / ');
  return (
    <>
      {/* A closed plan (kept two weeks, e.g. for its "not enough people"
          push) says why instead of how well it suits them. */}
      {closed ? (
        <ThemedText type="smallBold" themeColor="textSecondary">
          {t(closed)}
        </ThemedText>
      ) : (
        <View style={styles.tagRow}>
          <View style={[styles.tag, { backgroundColor: colors?.background ?? theme.backgroundSelected }]}>
            <ThemedText style={[styles.tagText, { color: colors?.text ?? theme.textSecondary }]}>{t(levelLabels[p.level])}</ThemedText>
          </View>
          {p.friendsInterested > 0 && (
            <ThemedText type="smallBold" themeColor="goldText">
              {friendsText(t, p.friendsInterested)}
            </ThemedText>
          )}
        </View>
      )}
      <View style={styles.intro}>
        <TitleBlock data={data} />
        {!!summary && <ThemedText style={styles.summary}>{summary}</ThemedText>}
      </View>
      {/* One labelled row per detail, so they read as separate facts rather than a paragraph. */}
      <Facts>
        <Fact label={t('plan.where')} value={pairText(names.area(p.areaId))} note={closed ? undefined : distanceText(t, p.travelMinutes)} />
        {!!hours && <Fact label={t('plan.length')} value={t('plan.lengthHours', { hours })} />}
        <Fact label={t('plan.language')} value={language} />
        <Fact
          label={t('plan.when')}
          // A dash rather than a translated "{start} to {end}": machine translation
          // lost the placeholders in Japanese, and a range needs no words.
          value={`${formatDate(dates[0], lang)} – ${formatDate(dates[dates.length - 1], lang)}`}
          info={{ label: t('plan.whenInfo'), text: t('plan.whenHint') }}
        />
        {!closed && <Fact label={t('plan.signupBy')} value={lastSignupDay(p.signupDeadline, lang)} />}
        {!closed && <Fact label={t('plan.interestedLabel')} value={String(p.signups)} />}
      </Facts>
      {p.level === PlanLevel.NOT_FOR_YOU && !closed && <ThemedText>{t('plan.notForYouBody')}</ThemedText>}
      {/* While the feed's copy stands in, the stops and map are still on their way. */}
      {loading ? <ActivityIndicator /> : <PlanStops steps={data.steps} routePolyline={data.routePolyline} names={names} closed={!!closed} />}
    </>
  );
}

/** "Wed Evening, Fri Evening, Sat Afternoon": the weekly times, in week order. */
function timesText(pattern: string[], t: ReturnType<typeof useI18n>['t'], lang: string) {
  return pattern
    .map(parseSlotKey)
    .sort((a, b) => a.weekday - b.weekday || a.block - b.block)
    .map((s) => `${weekdayName(s.weekday, lang)} ${t(blockKeys[s.block])}`)
    .join(', ');
}

/** Stage one of the outing: signed up, waiting for a date. */
function SignedUp({ data, times, names }: { data: GetPlanResponse; times: Times; names: Names }) {
  const { t } = useI18n();
  const theme = useTheme();
  const lang = useLang();
  const p = data.plan!;
  const { withdraw } = useSignupActions(p.id);
  return (
    <>
      <View style={styles.titleBlock}>
        <TitleBlock data={data} size="medium" />
        <ThemedText type="small" themeColor="textSecondary">
          {pairText(names.area(p.areaId))}, {distanceText(t, p.travelMinutes)}
        </ThemedText>
      </View>
      <Thread stage={0} id={`plan:${p.id}`} />
      <View style={[styles.card, { backgroundColor: theme.surface }]}>
        <ThemedText type="subtitle" style={styles.cardTitle}>
          {t('plan.signedUp')}
        </ThemedText>
        <ThemedText>
          {t('plan.interested', { count: p.signups })}. {t('plan.whenHint')}
        </ThemedText>
        <ThemedText type="small" themeColor="textSecondary">
          {t('plan.signupBy')}: {lastSignupDay(p.signupDeadline, lang)}
        </ThemedText>
        {p.friendsInterested > 0 && (
          <ThemedText type="smallBold" themeColor="goldText">
            {friendsText(t, p.friendsInterested)}
          </ThemedText>
        )}
        <View style={styles.cardFacts}>
          <Facts>
            <Fact label={t('plan.times')} value={timesText(times.pattern, t, lang)} />
            <Fact
              label={t('plan.blockOut')}
              value={times.blocked.length ? [...times.blocked].sort().map((d) => formatDate(d, lang)).join(', ') : t('common.none')}
            />
          </Facts>
        </View>
        <TextLink label={t('plan.update')} onPress={() => openSignup(p.id)} />
      </View>
      <StopsToggle steps={data.steps} routePolyline={data.routePolyline} names={names} />
      {withdraw.isError && <ThemedText themeColor="error">{t('error.network')}</ThemedText>}
      <TextLink label={t('plan.withdraw')} muted disabled={withdraw.isPending} onPress={() => withdraw.mutate()} />
    </>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  column: { width: '100%', maxWidth: FormWidth, alignSelf: 'center' },
  header: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', paddingHorizontal: Spacing.three },
  content: { padding: Spacing.three, paddingTop: Spacing.two, gap: Spacing.four - 4, paddingBottom: Spacing.five },
  footer: { padding: Spacing.three, gap: Spacing.one + 2, borderTopWidth: 1 },
  centerText: { textAlign: 'center' },
  tagRow: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', gap: Spacing.two },
  tag: { borderRadius: 999, paddingVertical: Spacing.half, paddingHorizontal: Spacing.two },
  tagText: { fontSize: 12, lineHeight: 16, fontWeight: 500 },
  intro: { gap: Spacing.two },
  titleBlock: { gap: Spacing.half },
  titleLarge: { fontSize: 28, lineHeight: 41 },
  summary: { lineHeight: 26 },
  card: { borderRadius: 16, padding: Spacing.three, gap: Spacing.one + 2 },
  cardTitle: { fontSize: 22, lineHeight: 32 },
  cardFacts: { marginTop: Spacing.two },
});
