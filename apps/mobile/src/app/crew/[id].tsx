import { type Crew, type CrewMember } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { Pressable, ScrollView, StyleSheet, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withDelay, withSequence } from 'react-native-reanimated';

import { ErrorLine } from '@/components/error-line';
import { Avatar } from '@/components/avatar';
import { PrimaryButton } from '@/components/primary-button';
import { QueryScreen } from '@/components/query-screen';
import { ResultButton } from '@/components/result-button';
import { SectionTitle } from '@/components/screen-header';
import { ScreenFrame, useLeave } from '@/components/screen-frame';
import { Segmented } from '@/components/segmented';
import { TextLink } from '@/components/text-link';
import { ThemedText } from '@/components/themed-text';
import { Brand, Spacing } from '@/constants/theme';
import { useCrew, useRespondToCrew } from '@/features/crews/data';
import { crewView } from '@/features/crews/crew-view';
import { crewName } from '@/features/crews/name';
import { ChatPanel, SuggestedPlan } from '@/features/chat/chat-panel';
import { suggestedPlans } from '@/features/chat/messages';
import { useRoom } from '@/features/chat/room';
import { usePlanCards } from '@/features/plans/data';
import { formatStart } from '@/features/plans/format';
import { usePlanNames } from '@/features/plans/names';
import { useMotion } from '@/hooks/use-motion';
import { useTheme } from '@/hooks/use-theme';
import { isRightToLeft } from '@/i18n/direction';
import { useI18n, useOtherLanguage } from '@/i18n/i18n-provider';
import { haptics } from '@/lib/haptics';

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
  const leave = useLeave('/');
  // An offer that started while it was open stays on the offer's layout,
  // showing that it started, until the person opens the chat.
  const [wasOffer, setWasOffer] = useState<boolean | null>(null);
  const [showChat, setShowChat] = useState(false);
  const c = crew.data;
  if (c && wasOffer === null) setWasOffer(crewView(c).isOffer);

  if (!c) {
    return (
      <QueryScreen onBack={leave} queries={[crew]} notFound={t('crew.notFound')}>
        {() => null}
      </QueryScreen>
    );
  }
  if (crewView(c).isOffer || (wasOffer && !showChat)) {
    return <Offer crew={c} onBack={leave} onOpenChat={() => setShowChat(true)} />;
  }
  return <Active crew={c} onBack={leave} />;
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
  const theme = useTheme();
  const { yesShown, quorum } = crewView(crew);
  const fill = useSharedValue(yesShown / quorum);
  useEffect(() => {
    fill.value = m.draw(yesShown / quorum);
  }, [yesShown, quorum, fill, m]);
  const fillStyle = useAnimatedStyle(() => ({
    transform: [{ scaleX: fill.value }],
  }));
  const rtl = isRightToLeft(languages.primary);
  return (
    <View style={styles.people}>
      <View style={[styles.peopleTrack, { backgroundColor: theme.onAccentTrack }]} />
      <Animated.View
        style={[
          styles.peopleTrack,
          {
            backgroundColor: Brand.gold,
            transformOrigin: rtl ? 'right' : 'left',
          },
          fillStyle,
        ]}
      />
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
  const style = useAnimatedStyle(() => ({
    transform: [{ translateY: lift.value }, { scale: scale.value }],
  }));
  return (
    <View style={styles.person} accessible accessibilityLabel={t(member.accepted ? 'crew.personSaidYes' : 'crew.personDeciding', { name: label })}>
      <Animated.View style={style}>
        <Avatar
          name={member.name}
          size={52}
          ring={member.accepted ? 'gold' : 'dashed'}
          fill={
            member.me && member.accepted ? { background: Brand.gold, text: Brand.ink } : { background: theme.surface, text: theme.text }
          }
        />
      </Animated.View>
      <ThemedText type="small" style={{ color: theme.onAccent }} numberOfLines={1}>
        {label}
      </ThemedText>
      <ThemedText type="caption" style={[styles.personState, { color: theme.onAccent }]}>
        {member.accepted ? t('crew.saidYes') : t('crew.deciding')}
      </ThemedText>
    </View>
  );
}

function Offer({ crew, onBack, onOpenChat }: { crew: Crew; onBack: () => void; onOpenChat: () => void }) {
  const { t, lang } = useI18n();
  const otherLanguage = useOtherLanguage();
  const theme = useTheme();
  const names = usePlanNames();
  const name = useCrewName(crew);
  const respond = useRespondToCrew(crew.id);
  const { isActive: started, accepted, yes, yesShown, quorum } = crewView(crew);

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
    ? t('crew.shareActivity', {
        activity: names.activity(crew.activityId).name,
      })
    : t('crew.shareArea', { area: names.area(crew.areaId).name });

  const head = (
    <>
      <View style={styles.offerTitle}>
        <ThemedText type="small" style={{ color: theme.onAccent }}>
          {started ? t('crew.yourCrew') : t('crew.offerEyebrow')}
        </ThemedText>
        <ThemedText type="title" style={{ color: theme.onAccent }} accessibilityRole="header">
          {name.name}
        </ThemedText>
        {!!name.secondary && (
          <ThemedText style={[styles.dim, { color: theme.onAccent }]} lang={otherLanguage}>
            {name.secondary}
          </ThemedText>
        )}
      </View>
      <People crew={crew} hop={hop} />
    </>
  );

  return (
    <ScreenFrame accent onBack={onBack} header={head}>
      <ScrollView contentContainerStyle={styles.offerBody} bounces={false}>
        <View
          style={styles.counter}
          accessible
          accessibilityLabel={t('crew.yesCount', {
            count: yesShown,
          })}>
          <ThemedText type="smallBold">{t('crew.yesCount', { count: yesShown })}</ThemedText>
          <View style={styles.bars}>
            {Array.from({ length: quorum }, (_, i) => (
              <View
                key={i}
                style={[
                  styles.bar,
                  {
                    backgroundColor: i < yes ? Brand.gold : theme.backgroundSelected,
                  },
                ]}
              />
            ))}
          </View>
        </View>
        {started ? (
          <ThemedText type="heading" themeColor="goldText" accessibilityLiveRegion="polite">
            {t('push.crewFormed')}
          </ThemedText>
        ) : (
          <>
            <ThemedText style={styles.lead}>{t('crew.offerTitle')}</ThemedText>
            <ThemedText>
              {t('crew.offerBody', {
                date: formatStart(crew.offerUntil, lang),
              })}
            </ThemedText>
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
        <ErrorLine error={respond.error} />
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
      </ScrollView>
    </ScreenFrame>
  );
}

function Active({ crew, onBack }: { crew: Crew; onBack: () => void }) {
  const { t } = useI18n();
  const otherLanguage = useOtherLanguage();
  const theme = useTheme();
  const name = useCrewName(crew);
  const [tab, setTab] = useState<'chat' | 'plans'>('chat');
  const [menu, setMenu] = useState(false);
  // The same room (and socket) as the chat tab's: suggested plans are chat messages.
  const { messages } = useRoom({ crewId: crew.id });
  const suggested = suggestedPlans(messages);
  const cards = usePlanCards(suggested);
  const members = crew.members
    .filter((m) => !m.me)
    .map((m) => m.name)
    .join(', ');

  const settings = (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={t('crew.settings')}
      accessibilityState={{ expanded: menu }}
      onPress={() => setMenu(!menu)}
      hitSlop={8}
      style={styles.more}>
      <ThemedText type="heading" style={{ color: theme.onAccent }}>⋯</ThemedText>
    </Pressable>
  );
  const head = (
    <>
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
        <View style={[styles.menu, { backgroundColor: theme.menuSurface }]}>
          <TextLink
            label={t('event.report')}
            onPress={() =>
              router.push({
                pathname: '/report/[id]',
                params: { id: crew.id, crew: '1' },
              })
            }
          />
          <TextLink
            label={t('crew.leave')}
            onPress={() =>
              router.push({
                pathname: '/crew-leave/[id]',
                params: { id: crew.id },
              })
            }
          />
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
    </>
  );

  return (
    <ScreenFrame accent onBack={onBack} end={settings} header={head}>
      {tab === 'chat' ? (
        // Suggestions already sit in the chat where they were made; the Plans tab gathers them.
        <ChatPanel room={{ crewId: crew.id }} />
      ) : (
        <ScrollView contentContainerStyle={styles.plans}>
          {suggested.length ? (
            <>
              <View style={styles.section}>
                <SectionTitle>{t('crew.suggested')}</SectionTitle>
                <SuggestedPlan planId={suggested[0]} plan={cards.get(suggested[0])} />
              </View>
              {suggested.length > 1 && (
                <View style={styles.section}>
                  <SectionTitle>{t('crew.suggestedBefore')}</SectionTitle>
                  {suggested.slice(1).map((p) => (
                    <SuggestedPlan key={p} planId={p} plan={cards.get(p)} />
                  ))}
                </View>
              )}
            </>
          ) : (
            <ThemedText themeColor="textSecondary">{t('crew.noSuggestion')}</ThemedText>
          )}
        </ScrollView>
      )}
    </ScreenFrame>
  );
}

const styles = StyleSheet.create({
  dim: { opacity: 0.85 },
  offerTitle: {
    gap: Spacing.half,
    marginTop: Spacing.two,
    marginBottom: Spacing.four,
  },
  people: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingHorizontal: Spacing.two,
  },
  // From the first avatar's centre to the last one's (52 wide, 8 in).
  peopleTrack: { position: 'absolute', start: 34, end: 34, top: 25, height: 3 },
  person: { alignItems: 'center', gap: Spacing.one, width: 72 },
  personState: { opacity: 0.85 },
  offerBody: {
    flexGrow: 1,
    padding: Spacing.three,
    paddingTop: Spacing.four,
    gap: Spacing.three,
  },
  counter: { flexDirection: 'row', alignItems: 'center', gap: Spacing.two },
  bars: { flexDirection: 'row', gap: Spacing.one },
  bar: { width: 18, height: 6, borderRadius: 3 },
  lead: { fontWeight: 700 },
  facts: { gap: Spacing.half },
  factGap: { marginTop: Spacing.two },
  more: {
    minWidth: 44,
    minHeight: 44,
    alignItems: 'center',
    justifyContent: 'center',
  },
  menu: {
    flexDirection: 'row',
    gap: Spacing.four,
    borderRadius: 12,
    paddingHorizontal: Spacing.three,
  },
  plans: { padding: Spacing.three, gap: Spacing.four },
  section: { gap: Spacing.two },
});
