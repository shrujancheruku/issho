import { Code, ConnectError, CrewResponse, CrewStatus, type Crew, type CrewMember } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { ActivityIndicator, KeyboardAvoidingView, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withDelay, withSequence } from 'react-native-reanimated';
import { SafeAreaView, useSafeAreaInsets } from 'react-native-safe-area-context';

import { Avatar } from '@/components/avatar';
import { PrimaryButton } from '@/components/primary-button';
import { ResultButton } from '@/components/result-button';
import { SectionTitle } from '@/components/screen-header';
import { Segmented } from '@/components/segmented';
import { BackLink, TextLink } from '@/components/text-link';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Brand, FormWidth, Spacing } from '@/constants/theme';
import { useCrew, useRespondToCrew } from '@/features/crews/data';
import { crewName } from '@/features/crews/name';
import { useChat } from '@/features/events/chat';
import { ChatPanel, SuggestedPlan } from '@/features/events/chat-panel';
import { formatStart } from '@/features/plans/format';
import { usePlanNames } from '@/features/plans/names';
import { useMotion } from '@/hooks/use-motion';
import { useTheme } from '@/hooks/use-theme';
import { isRightToLeft } from '@/i18n/direction';
import { useI18n, useOtherLanguage } from '@/i18n/i18n-provider';
import { haptics } from '@/lib/haptics';

/** A crew starts when this many say yes. */
const Quorum = 3;

/**
 * A crew. While it's an offer: who's in it, on a thread that fills a third
 * with each yes, and yes / no. If the person's yes is the one that starts
 * it, everyone hops and the button opens the chat. Once it has formed: the
 * crew's chat and the plans Issho suggested in it.
 */
export default function CrewScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t } = useI18n();
  const crew = useCrew(id);
  const leave = () => (router.canGoBack() ? router.back() : router.replace('/'));
  // An offer that started while it was open stays on the offer's layout,
  // showing that it started, until the person opens the chat.
  const [wasOffer, setWasOffer] = useState<boolean | null>(null);
  const [showChat, setShowChat] = useState(false);
  const c = crew.data;
  if (c && wasOffer === null) setWasOffer(c.status === CrewStatus.OFFERED);

  const notFound = crew.error instanceof ConnectError && crew.error.code === Code.NotFound;
  if (!c) {
    return (
      <Plain onBack={leave}>
        {notFound ? (
          <ThemedText themeColor="textSecondary">{t('crew.notFound')}</ThemedText>
        ) : crew.isError ? (
          <>
            <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
            <TextLink label={t('common.retry')} onPress={() => void crew.refetch()} />
          </>
        ) : (
          <ActivityIndicator />
        )}
      </Plain>
    );
  }
  if (c.status === CrewStatus.OFFERED || (wasOffer && !showChat)) {
    return <Offer crew={c} onBack={leave} onOpenChat={() => setShowChat(true)} />;
  }
  return <Active crew={c} onBack={leave} />;
}

function Plain({ onBack, children }: { onBack: () => void; children: React.ReactNode }) {
  const { t } = useI18n();
  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <View style={[styles.column, styles.plain]}>
          <BackLink label={t('common.back')} onPress={onBack} />
          {children}
        </View>
      </SafeAreaView>
    </ThemedView>
  );
}

function useCrewName(crew: Crew) {
  const { t } = useI18n();
  const names = usePlanNames();
  return crewName(crew, names, (n) => t('crew.name', { name: n }));
}

/** The four of you on a thread: dashed while deciding, gold once they say yes. */
function People({ crew, hop }: { crew: Crew; hop: number }) {
  const { t, languages } = useI18n();
  const m = useMotion();
  const yes = crew.members.filter((x) => x.accepted).length;
  const fill = useSharedValue(Math.min(yes, Quorum) / Quorum);
  useEffect(() => {
    fill.value = m.draw(Math.min(yes, Quorum) / Quorum);
  }, [yes, fill, m]);
  const fillStyle = useAnimatedStyle(() => ({ transform: [{ scaleX: fill.value }] }));
  const rtl = isRightToLeft(languages.primary);
  return (
    <View style={styles.people}>
      <View style={[styles.peopleTrack, styles.peopleTrackBase]} />
      <Animated.View style={[styles.peopleTrack, { backgroundColor: Brand.gold, transformOrigin: rtl ? 'right' : 'left' }, fillStyle]} />
      {crew.members.map((member, i) => (
        <Person key={member.id || i} member={member} index={i} hop={hop} label={member.me ? t('crew.you') : member.name} />
      ))}
    </View>
  );
}

function Person({ member, index, hop, label }: { member: CrewMember; index: number; hop: number; label: string }) {
  const { t } = useI18n();
  const theme = useTheme();
  const m = useMotion();
  const scale = useSharedValue(1);
  const lift = useSharedValue(0);
  // Saying yes while the screen is open pops the ring.
  const [wasYes, setWasYes] = useState(member.accepted);
  const popped = member.accepted && !wasYes;
  if (popped) setWasYes(true);
  useEffect(() => {
    if (popped && !m.reduced) {
      scale.value = 0.7;
      scale.value = m.gather(1);
    }
  }, [popped, scale, m]);
  // The crew started: everyone hops, one after another.
  useEffect(() => {
    if (hop > 0 && !m.reduced) {
      lift.value = withDelay(index * 70, withSequence(m.move(-10), m.gather(0)));
    }
  }, [hop, index, lift, m]);
  const style = useAnimatedStyle(() => ({ transform: [{ translateY: lift.value }, { scale: scale.value }] }));
  return (
    <View style={styles.person} accessible accessibilityLabel={`${label}, ${member.accepted ? t('crew.saidYes') : t('crew.deciding')}`}>
      <Animated.View style={style}>
        <Avatar
          name={member.name}
          size={52}
          ring={member.accepted ? 'gold' : 'dashed'}
          fill={member.me && member.accepted ? { background: Brand.gold, text: Brand.ink } : { background: theme.surface, text: theme.text }}
        />
      </Animated.View>
      <ThemedText type="small" style={{ color: theme.onAccent }} numberOfLines={1}>
        {label}
      </ThemedText>
      <ThemedText style={[styles.personState, { color: theme.onAccent }]}>{member.accepted ? t('crew.saidYes') : t('crew.deciding')}</ThemedText>
    </View>
  );
}

function Offer({ crew, onBack, onOpenChat }: { crew: Crew; onBack: () => void; onOpenChat: () => void }) {
  const { t, languages, showing } = useI18n();
  const otherLanguage = useOtherLanguage();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const theme = useTheme();
  const insets = useSafeAreaInsets();
  const names = usePlanNames();
  const name = useCrewName(crew);
  const respond = useRespondToCrew(crew.id);
  const started = crew.status === CrewStatus.ACTIVE;
  const accepted = crew.response === CrewResponse.ACCEPTED;
  const yes = crew.members.filter((m) => m.accepted).length;

  // The crew started while the screen was open: hop, and a success haptic.
  const [hop, setHop] = useState(0);
  const [wasStarted, setWasStarted] = useState(started);
  if (started && !wasStarted) {
    setWasStarted(true);
    setHop((h) => h + 1);
  }
  useEffect(() => {
    if (hop > 0) haptics.success();
  }, [hop]);

  const others = crew.members.filter((m) => !m.me);
  const said = others.filter((m) => m.accepted).map((m) => m.name);
  const deciding = others.filter((m) => !m.accepted).map((m) => m.name);
  const share = crew.activityId
    ? t('crew.shareActivity', { activity: names.activity(crew.activityId).name })
    : t('crew.shareArea', { area: names.area(crew.areaId).name });
  const tooLate = respond.error instanceof ConnectError && respond.error.code === Code.FailedPrecondition;

  return (
    <ThemedView style={styles.fill}>
      <ScrollView contentContainerStyle={styles.offerScroll} bounces={false}>
        <View style={[styles.offerHead, { backgroundColor: theme.accent, paddingTop: insets.top + Spacing.two }]}>
          <View style={styles.column}>
            <BackLink label={t('common.back')} onPress={onBack} color={theme.onAccent} />
            <View style={styles.offerTitle}>
              <ThemedText type="small" style={{ color: theme.onAccent }}>
                {started ? t('crew.yourCrew') : t('crew.offerEyebrow')}
              </ThemedText>
              <ThemedText type="subtitle" style={[styles.crewName, { color: theme.onAccent }]} accessibilityRole="header">
                {name.name}
              </ThemedText>
              {!!name.secondary && (
                <ThemedText style={[styles.dim, { color: theme.onAccent }]} lang={otherLanguage}>
                  {name.secondary}
                </ThemedText>
              )}
            </View>
            <People crew={crew} hop={hop} />
          </View>
        </View>
        <View style={[styles.column, styles.offerBody]}>
          <View style={styles.counter} accessible accessibilityLabel={t('crew.yesCount', { count: Math.min(yes, Quorum) })}>
            <ThemedText type="smallBold">{t('crew.yesCount', { count: Math.min(yes, Quorum) })}</ThemedText>
            <View style={styles.bars}>
              {Array.from({ length: Quorum }, (_, i) => (
                <View key={i} style={[styles.bar, { backgroundColor: i < yes ? Brand.gold : theme.backgroundSelected }]} />
              ))}
            </View>
          </View>
          {started ? (
            <ThemedText type="subtitle" themeColor="goldText" style={styles.started} accessibilityLiveRegion="polite">
              {t('push.crewFormed')}
            </ThemedText>
          ) : (
            <>
              <ThemedText style={styles.lead}>{t('crew.offerTitle')}</ThemedText>
              <ThemedText>{t('crew.offerBody', { date: formatStart(crew.offerUntil, lang) })}</ThemedText>
              <View style={styles.facts}>
                <ThemedText type="small" themeColor="textSecondary">
                  {t('crew.share')}
                </ThemedText>
                <ThemedText>{share}</ThemedText>
                {(said.length > 0 || deciding.length > 0) && (
                  <>
                    <ThemedText type="small" themeColor="textSecondary" style={styles.factGap}>
                      {t('crew.soFar')}
                    </ThemedText>
                    {said.length > 0 && <ThemedText>{t('crew.saidYesNames', { names: said.join(', ') })}</ThemedText>}
                    {deciding.length > 0 && <ThemedText>{t('crew.decidingNames', { names: deciding.join(', ') })}</ThemedText>}
                  </>
                )}
              </View>
            </>
          )}
          {respond.isError && <ThemedText themeColor="error">{tooLate ? t('event.tooLate') : t('error.network')}</ThemedText>}
          {started ? (
            <PrimaryButton label={t('crew.openChat')} onPress={onOpenChat} />
          ) : (
            <>
              <ResultButton
                label={t('crew.accept')}
                doneLabel={t('crew.youSaidYes')}
                done={accepted}
                busy={respond.isPending && respond.variables === true}
                onPress={() => respond.mutate(true, { onSuccess: haptics.tap })}
              />
              {/* Saying no moves plainly: no bounce, no celebration. */}
              <TextLink
                label={t('crew.decline')}
                center
                disabled={respond.isPending}
                onPress={() => respond.mutate(false, { onSuccess: onBack })}
              />
            </>
          )}
        </View>
      </ScrollView>
    </ThemedView>
  );
}

function Active({ crew, onBack }: { crew: Crew; onBack: () => void }) {
  const { t } = useI18n();
  const otherLanguage = useOtherLanguage();
  const theme = useTheme();
  const insets = useSafeAreaInsets();
  const name = useCrewName(crew);
  const [tab, setTab] = useState<'chat' | 'plans'>('chat');
  const [menu, setMenu] = useState(false);
  const { messages } = useChat({ crewId: crew.id });
  // Suggested plans come as chat messages: newest first.
  const suggested = [...new Set(messages.filter((m) => m.planId).map((m) => m.planId!))].reverse();
  const members = crew.members.filter((m) => !m.me).map((m) => m.name).join(', ');

  return (
    <ThemedView style={styles.fill}>
      <View style={[styles.activeHead, { backgroundColor: theme.accent, paddingTop: insets.top + Spacing.two }]}>
        <View style={[styles.column, styles.activeHeadInner]}>
          <View style={styles.headRow}>
            <BackLink label={t('common.back')} onPress={onBack} color={theme.onAccent} />
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={t('crew.settings')}
              accessibilityState={{ expanded: menu }}
              onPress={() => setMenu(!menu)}
              hitSlop={8}
              style={styles.more}>
              <ThemedText style={[styles.moreDots, { color: theme.onAccent }]}>⋯</ThemedText>
            </Pressable>
          </View>
          <View>
            <ThemedText type="subtitle" style={{ color: theme.onAccent }} accessibilityRole="header">
              {name.name}
            </ThemedText>
            {!!name.secondary && (
              <ThemedText type="small" style={[styles.dim, { color: theme.onAccent }]} lang={otherLanguage}>
                {name.secondary}
              </ThemedText>
            )}
            <ThemedText type="small" style={[styles.dim, { color: theme.onAccent }]}>
              {members}
            </ThemedText>
          </View>
          {menu && (
            <View style={styles.menu}>
              <TextLink label={t('event.report')} onPress={() => router.push({ pathname: '/report/[id]', params: { id: crew.id, crew: '1' } })} />
              <TextLink label={t('crew.leave')} onPress={() => router.push({ pathname: '/crew-leave/[id]', params: { id: crew.id } })} />
            </View>
          )}
          <Segmented
            onAccent
            label={t('crew.tabs')}
            value={tab}
            onChange={setTab}
            options={[
              { value: 'chat', label: t('crew.chat') },
              { value: 'plans', label: t('tabs.plans') },
            ]}
          />
        </View>
      </View>
      <SafeAreaView style={styles.fill} edges={['bottom', 'left', 'right']}>
        <KeyboardAvoidingView behavior="padding" style={[styles.fill, styles.column]}>
          {tab === 'chat' ? (
            // Suggestions already sit in the chat where they were made; the Plans tab gathers them.
            <ChatPanel target={{ crewId: crew.id }} />
          ) : (
            <ScrollView contentContainerStyle={styles.plans}>
              {suggested.length ? (
                <>
                  <View style={styles.section}>
                    <SectionTitle>{t('crew.suggested')}</SectionTitle>
                    <SuggestedPlan planId={suggested[0]} />
                  </View>
                  {suggested.length > 1 && (
                    <View style={styles.section}>
                      <SectionTitle>{t('crew.suggestedBefore')}</SectionTitle>
                      {suggested.slice(1).map((p) => (
                        <SuggestedPlan key={p} planId={p} />
                      ))}
                    </View>
                  )}
                </>
              ) : (
                <ThemedText themeColor="textSecondary">{t('crew.noSuggestion')}</ThemedText>
              )}
            </ScrollView>
          )}
        </KeyboardAvoidingView>
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  column: { width: '100%', maxWidth: FormWidth, alignSelf: 'center' },
  plain: { padding: Spacing.three, gap: Spacing.three },
  dim: { opacity: 0.85 },
  offerScroll: { flexGrow: 1 },
  offerHead: { paddingHorizontal: Spacing.three, paddingBottom: Spacing.four + 4 },
  offerTitle: { gap: Spacing.half, marginTop: Spacing.two, marginBottom: Spacing.four },
  crewName: { fontSize: 36, lineHeight: 52 },
  people: { flexDirection: 'row', justifyContent: 'space-between', paddingHorizontal: Spacing.two },
  // From the first avatar's centre to the last one's (52 wide, 8 in).
  peopleTrack: { position: 'absolute', start: 34, end: 34, top: 25, height: 3 },
  peopleTrackBase: { backgroundColor: 'rgba(255,255,255,0.28)' },
  person: { alignItems: 'center', gap: Spacing.one, width: 72 },
  personState: { fontSize: 11, lineHeight: 14, opacity: 0.85 },
  offerBody: { padding: Spacing.three, paddingTop: Spacing.four, gap: Spacing.three },
  counter: { flexDirection: 'row', alignItems: 'center', gap: Spacing.two },
  bars: { flexDirection: 'row', gap: Spacing.one },
  bar: { width: 18, height: 6, borderRadius: 3 },
  started: { fontSize: 22, lineHeight: 32 },
  lead: { fontWeight: 700 },
  facts: { gap: Spacing.half },
  factGap: { marginTop: Spacing.two },
  activeHead: { paddingHorizontal: Spacing.three, paddingBottom: Spacing.three - 4 },
  activeHeadInner: { gap: Spacing.three - 4 },
  headRow: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' },
  more: { minWidth: 44, minHeight: 44, alignItems: 'center', justifyContent: 'center' },
  moreDots: { fontSize: 24, lineHeight: 28 },
  menu: { flexDirection: 'row', gap: Spacing.four, backgroundColor: 'rgba(255,255,255,0.92)', borderRadius: 12, paddingHorizontal: Spacing.three },
  plans: { padding: Spacing.three, gap: Spacing.four },
  section: { gap: Spacing.two },
});
