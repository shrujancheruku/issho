import { PlanLevel, type Crew, type FeedPlan, type MyEvent } from '@issho/api-client';
import { useQuery } from '@tanstack/react-query';
import { router } from 'expo-router';
import { ActivityIndicator, Pressable, RefreshControl, ScrollView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Avatar } from '@/components/avatar';
import { ListGroup, ListRow } from '@/components/list-group';
import { PrimaryButton } from '@/components/primary-button';
import { ScreenHeader, SectionTitle } from '@/components/screen-header';
import { TextLink } from '@/components/text-link';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BottomTabInset, MaxContentWidth, Spacing } from '@/constants/theme';
import { crewName } from '@/features/crews/name';
import { formatStart, lastSignupDay } from '@/features/plans/format';
import { levelColors, levelLabels } from '@/features/plans/level';
import { usePlanNames } from '@/features/plans/names';
import { friendsText, pairText, planTitle } from '@/features/plans/plan-card';
import { useTogether, type NeedsYou } from '@/features/together/data';
import { InviteCard } from '@/features/together/invite-card';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { api } from '@/lib/api';

type Names = ReturnType<typeof usePlanNames>;

/** Developer aid: API reachability and build version. Development builds only. */
function ApiStatus() {
  const health = useQuery({ queryKey: ['health'], queryFn: () => api.health.check({}) });
  if (health.isPending) return <ThemedText type="code">API …</ThemedText>;
  if (health.isError) return <ThemedText type="code">API unreachable</ThemedText>;
  return (
    <ThemedText type="code">
      API {health.data.databaseOk ? 'ok' : 'database down'} · {health.data.version}
    </ThemedText>
  );
}

const openEvent = (id: string) => router.push({ pathname: '/event/[id]', params: { id } });
const openCrew = (id: string) => router.push({ pathname: '/crew/[id]', params: { id } });
const openPlan = (id: string) => router.push({ pathname: '/plan/[id]', params: { id } });

/**
 * Together: the people you're meeting. What needs an answer comes first
 * (one gold button), then every group chat (crews and outings you're going
 * on), then plans you signed up for that are waiting for a date. Someone
 * with none of those yet sees how Issho works and this week's picks.
 */
export default function TogetherScreen() {
  const { t } = useI18n();
  const names = usePlanNames();
  const { needsYou, groups, waiting, events, crews, feed } = useTogether();
  const loading = events.isPending || crews.isPending || feed.isPending || !names.ready;
  const failed = events.isError || crews.isError || feed.isError;
  const refreshing = events.isRefetching || crews.isRefetching || feed.isRefetching;
  const refresh = () => void Promise.all([events.refetch(), crews.refetch(), feed.refetch()]);
  const empty = !needsYou.length && !groups.length && !waiting.length;

  const invites = needsYou.filter((n): n is Extract<NeedsYou, { kind: 'invite' }> => n.kind === 'invite');
  const others = needsYou.filter((n) => n.kind !== 'invite');

  let body;
  if (failed && empty) {
    body = (
      <View style={styles.center}>
        <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
        <TextLink label={t('common.retry')} onPress={refresh} center />
      </View>
    );
  } else if (loading) {
    body = (
      <View style={styles.center}>
        <ActivityIndicator />
      </View>
    );
  } else if (empty) {
    body = <FirstWeek plans={feed.data ?? []} names={names} />;
  } else {
    body = (
      <>
        {!!needsYou.length && (
          <View style={styles.section}>
            <SectionTitle>{t('together.needsYou')}</SectionTitle>
            {invites.map((n) => (
              <InviteCard key={n.event.id} event={n.event} names={names} />
            ))}
            {!!others.length && (
              <ListGroup inset={Spacing.three + 48}>
                {others.map((n) => (
                  <NeedsYouRow key={n.kind === 'crewOffer' ? n.crew.id : n.event.id} item={n} names={names} />
                ))}
              </ListGroup>
            )}
          </View>
        )}
        {!!groups.length && (
          <View style={styles.section}>
            <SectionTitle>{t('together.groups')}</SectionTitle>
            <ListGroup inset={Spacing.three + 48}>
              {groups.map((g) =>
                g.kind === 'crew' ? (
                  <CrewRow key={g.crew.id} crew={g.crew} names={names} />
                ) : (
                  <OutingRow key={g.event.id} event={g.event} names={names} />
                ),
              )}
            </ListGroup>
          </View>
        )}
        {!!waiting.length && (
          <View style={styles.section}>
            <SectionTitle>{t('together.waiting')}</SectionTitle>
            <ListGroup>
              {waiting.map((p) => (
                <WaitingRow key={p.id} plan={p} names={names} />
              ))}
            </ListGroup>
          </View>
        )}
        <TextLink label={t('together.otherPlans')} onPress={() => router.navigate('/plans')} />
      </>
    );
  }

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill} edges={['top', 'left', 'right']}>
        <ScrollView
          contentContainerStyle={styles.content}
          refreshControl={<RefreshControl refreshing={refreshing} onRefresh={refresh} />}>
          <ScreenHeader title={t('together.title')} />
          {body}
          {__DEV__ && <ApiStatus />}
        </ScrollView>
      </SafeAreaView>
    </ThemedView>
  );
}

/** Two lines beside an avatar: a name, and what's new or when. */
function RowText({ title, detail, detailColor, extra }: { title: string; detail: string; detailColor?: 'goldText' | 'textSecondary'; extra?: string }) {
  return (
    <View style={styles.rowText}>
      <ThemedText style={styles.rowTitle} numberOfLines={1}>
        {title}
      </ThemedText>
      <ThemedText type="small" themeColor={detailColor ?? 'textSecondary'} numberOfLines={1}>
        {detail}
      </ThemedText>
      {!!extra && (
        <ThemedText type="smallBold" themeColor="link" style={styles.extra}>
          {extra}
        </ThemedText>
      )}
    </View>
  );
}

function useLang() {
  const { languages, showing } = useI18n();
  return showing === 'secondary' ? languages.secondary : languages.primary;
}

function eventTitle(e: MyEvent, showing: 'primary' | 'secondary') {
  return showing === 'secondary' && e.secondaryTitle ? e.secondaryTitle : e.title;
}

function NeedsYouRow({ item, names }: { item: Exclude<NeedsYou, { kind: 'invite' }>; names: Names }) {
  const { t, showing } = useI18n();
  const lang = useLang();
  if (item.kind === 'crewOffer') {
    const name = crewName(item.crew, names, (n) => t('crew.name', { name: n })).name;
    return (
      <ListRow onPress={() => openCrew(item.crew.id)}>
        <Avatar name={name} ring="gold" />
        <RowText title={name} detail={t('crew.offerCard', { date: formatStart(item.crew.offerUntil, lang) })} detailColor="goldText" />
      </ListRow>
    );
  }
  const e = item.event;
  if (item.kind === 'checkIn') {
    return (
      <ListRow onPress={() => openEvent(e.id)}>
        <Avatar name={eventTitle(e, showing)} />
        <RowText title={eventTitle(e, showing)} detail={t('together.checkIn')} detailColor="goldText" />
      </ListRow>
    );
  }
  return (
    <ListRow onPress={() => router.push({ pathname: '/feedback/[id]', params: { id: e.id } })}>
      <Avatar name={eventTitle(e, showing)} />
      <RowText title={t('event.feedback')} detail={`${eventTitle(e, showing)}, ${formatStart(e.startsAt, lang)}`} detailColor="goldText" />
    </ListRow>
  );
}

function CrewRow({ crew, names }: { crew: Crew; names: Names }) {
  const { t } = useI18n();
  const name = crewName(crew, names, (n) => t('crew.name', { name: n })).name;
  const members = crew.members.filter((m) => !m.me).map((m) => m.name);
  return (
    <ListRow onPress={() => openCrew(crew.id)} chevron={false}>
      <Avatar name={name} ring="gold" />
      <RowText title={name} detail={members.join(', ')} />
    </ListRow>
  );
}

function OutingRow({ event, names }: { event: MyEvent; names: Names }) {
  const { t, showing } = useI18n();
  const lang = useLang();
  const title = eventTitle(event, showing);
  return (
    <ListRow onPress={() => openEvent(event.id)} chevron={false}>
      <Avatar name={title} />
      <RowText
        title={title}
        detail={t('event.confirmedCount', { count: event.confirmed })}
        extra={`${formatStart(event.startsAt, lang)}, ${names.area(event.areaId).name}`}
      />
    </ListRow>
  );
}

function WaitingRow({ plan, names }: { plan: FeedPlan; names: Names }) {
  const { t, showing } = useI18n();
  const lang = useLang();
  return (
    <ListRow onPress={() => openPlan(plan.id)}>
      <View style={styles.rowText}>
        <ThemedText style={styles.rowTitle}>{planTitle(plan, showing).name}</ThemedText>
        <ThemedText type="small" themeColor="textSecondary">
          {pairText(names.area(plan.areaId))}. {t('plan.interested', { count: plan.signups })}
        </ThemedText>
        {plan.friendsInterested > 0 && (
          <ThemedText type="smallBold" themeColor="goldText">
            {friendsText(t, plan.friendsInterested)}
          </ThemedText>
        )}
        <ThemedText type="small" themeColor="textSecondary" style={styles.extra}>
          {t('plan.signupBy')}: {lastSignupDay(plan.signupDeadline, lang)}
        </ThemedText>
      </View>
    </ListRow>
  );
}

const steps = [
  ['together.step1', 'together.step1Body'],
  ['together.step2', 'together.step2Body'],
  ['together.step3', 'together.step3Body'],
] as const;

/**
 * Before the first signup: how an outing becomes friends, on the gold
 * thread, then this week's two best plans and the way to the rest.
 */
function FirstWeek({ plans, names }: { plans: FeedPlan[]; names: Names }) {
  const { t, showing } = useI18n();
  const theme = useTheme();
  const picks = plans.filter((p) => p.level === PlanLevel.TOP || p.level === PlanLevel.GOOD).slice(0, 2);
  return (
    <>
      <ThemedText style={styles.intro}>{t('together.intro')}</ThemedText>
      <View accessibilityRole="list">
        {steps.map(([title, body], i) => (
          <View key={title} style={styles.step}>
            <View style={styles.stepRail}>
              <View style={[styles.stepKnot, { borderColor: theme.goldText, backgroundColor: theme.background }]} />
              {i < steps.length - 1 && <View style={[styles.stepLine, { backgroundColor: theme.goldText }]} />}
            </View>
            <View style={[styles.stepText, i < steps.length - 1 && styles.stepGap]}>
              <ThemedText style={styles.rowTitle}>{t(title)}</ThemedText>
              <ThemedText type="small" themeColor="textSecondary">
                {t(body)}
              </ThemedText>
            </View>
          </View>
        ))}
      </View>
      {!!picks.length && (
        <View style={styles.section}>
          <SectionTitle>{t('together.picks')}</SectionTitle>
          <ListGroup>
            {picks.map((p) => {
              const colors = levelColors(p.level, theme)!;
              return (
                <Pressable
                  key={p.id}
                  accessibilityRole="button"
                  onPress={() => openPlan(p.id)}
                  style={({ pressed }) => [styles.pickRow, pressed && { backgroundColor: theme.backgroundElement }]}>
                  <View style={[styles.levelTag, { backgroundColor: colors.background }]}>
                    <ThemedText style={[styles.levelTagText, { color: colors.text }]}>{t(levelLabels[p.level])}</ThemedText>
                  </View>
                  <View style={styles.rowText}>
                    <ThemedText style={styles.rowTitle}>{planTitle(p, showing).name}</ThemedText>
                    <ThemedText type="small" themeColor="textSecondary">
                      {pairText(names.area(p.areaId))}. {t('plan.interested', { count: p.signups })}
                    </ThemedText>
                  </View>
                </Pressable>
              );
            })}
          </ListGroup>
        </View>
      )}
      <PrimaryButton label={t('together.seePlans')} onPress={() => router.navigate('/plans')} />
    </>
  );
}

const KNOT = 14;

const styles = StyleSheet.create({
  fill: { flex: 1 },
  content: {
    padding: Spacing.three,
    paddingTop: Spacing.four,
    paddingBottom: BottomTabInset + Spacing.four,
    gap: Spacing.four + 4,
    width: '100%',
    maxWidth: MaxContentWidth,
    alignSelf: 'center',
  },
  center: { paddingVertical: Spacing.six, alignItems: 'center', gap: Spacing.two },
  section: { gap: Spacing.two },
  rowText: { flex: 1, minWidth: 0 },
  rowTitle: { fontWeight: 700 },
  extra: { fontSize: 12, lineHeight: 16 },
  intro: { maxWidth: 420, lineHeight: 26 },
  step: { flexDirection: 'row', gap: Spacing.three },
  stepRail: { width: KNOT, alignItems: 'center' },
  stepKnot: { width: KNOT, height: KNOT, borderRadius: KNOT / 2, borderWidth: 2, marginTop: 5 },
  stepLine: { width: 2, flex: 1, marginTop: -2, marginBottom: -7 },
  stepText: { flex: 1 },
  stepGap: { paddingBottom: Spacing.three + 2 },
  pickRow: { flexDirection: 'row', alignItems: 'center', gap: Spacing.three - 4, padding: Spacing.three - 4, paddingHorizontal: Spacing.three },
  levelTag: { width: 76, paddingVertical: Spacing.one, borderRadius: 12, alignItems: 'center' },
  levelTagText: { fontSize: 12, lineHeight: 16, fontWeight: 500, textAlign: 'center' },
});
