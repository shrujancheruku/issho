import { Code, ConnectError, CrewResponse, CrewStatus } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, FlatList, KeyboardAvoidingView, Pressable, ScrollView, StyleSheet, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { LanguageSwitch } from '@/components/language-switch';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Brand, FormWidth, Spacing } from '@/constants/theme';
import { useCrew, useRespondToCrew } from '@/features/crews/data';
import { crewName } from '@/features/crews/name';
import { useChat } from '@/features/events/chat';
import type { Message } from '@/features/events/chat-messages';
import { usePlan } from '@/features/plans/data';
import { formatStart, lastSignupDay } from '@/features/plans/format';
import { usePlanNames } from '@/features/plans/names';
import { pairText, planTitle } from '@/features/plans/plan-card';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

const MaxMessage = 1000; // as the server (plans.MaxChatMessage)

/**
 * A plan Issho suggested in the chat: what, where and when signups close,
 * from GetPlan (the message itself carries only the plan's id).
 */
function SuggestedPlan({ planId }: { planId: string }) {
  const { t, languages, showing } = useI18n();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const theme = useTheme();
  const names = usePlanNames();
  const plan = usePlan(planId).data?.plan;
  return (
    <Pressable
      accessibilityRole="button"
      onPress={() => router.push({ pathname: '/plan/[id]', params: { id: planId } })}
      style={[styles.suggestion, { backgroundColor: theme.backgroundElement }]}>
      <ThemedText type="smallBold" style={{ color: Brand.gold }}>
        {t('crew.suggested')}
      </ThemedText>
      {plan && (
        <>
          <ThemedText type="subtitle" style={styles.suggestionTitle}>
            {planTitle(plan, showing).name}
          </ThemedText>
          <ThemedText type="small" themeColor="textSecondary">
            {pairText(names.area(plan.areaId))} · {t('plan.signupBy')}: {lastSignupDay(plan.signupDeadline, lang)}
          </ThemedText>
        </>
      )}
      <ThemedText type="link">{t('crew.seePlan')} ›</ThemedText>
    </Pressable>
  );
}

/** A crew's chat, with the plans Issho suggests in it. */
function CrewChat({ id }: { id: string }) {
  const { t } = useI18n();
  const theme = useTheme();
  const { messages, history, send } = useChat({ crewId: id });
  const [draft, setDraft] = useState('');
  const list = useRef<FlatList<Message>>(null);

  // Keep the newest message in view.
  useEffect(() => {
    if (messages.length) list.current?.scrollToEnd({ animated: true });
  }, [messages.length]);

  const submit = () => {
    const body = draft.trim();
    if (!body || send.isPending) return;
    send.mutate(body, { onSuccess: () => setDraft('') });
  };

  return (
    <>
      {history.isPending ? (
        <ActivityIndicator style={styles.fill} />
      ) : (
        <FlatList
          ref={list}
          style={styles.fill}
          data={messages}
          keyExtractor={(m) => String(m.id)}
          contentContainerStyle={styles.messages}
          ListEmptyComponent={
            <ThemedText type="small" themeColor="textSecondary" style={styles.center}>
              {t('chat.empty')}
            </ThemedText>
          }
          renderItem={({ item }) =>
            item.planId ? (
              <SuggestedPlan planId={item.planId} />
            ) : (
              <View style={[styles.bubble, item.mine ? styles.mine : [styles.theirs, { backgroundColor: theme.backgroundElement }]]}>
                {!item.mine && (
                  <ThemedText type="smallBold" themeColor="textSecondary">
                    {item.name}
                  </ThemedText>
                )}
                <ThemedText style={item.mine && styles.mineText}>{item.body}</ThemedText>
              </View>
            )
          }
        />
      )}
      {send.isError && <ThemedText style={styles.error}>{t('error.network')}</ThemedText>}
      <View style={[styles.composer, { borderTopColor: theme.backgroundSelected }]}>
        <TextInput
          value={draft}
          onChangeText={setDraft}
          placeholder={t('chat.placeholder')}
          placeholderTextColor={theme.textSecondary}
          maxLength={MaxMessage}
          multiline
          style={[styles.input, { color: theme.text, backgroundColor: theme.backgroundElement }]}
        />
        <Pressable
          accessibilityRole="button"
          accessibilityLabel={t('chat.send')}
          disabled={!draft.trim() || send.isPending}
          onPress={submit}
          style={[styles.send, (!draft.trim() || send.isPending) && styles.disabled]}>
          <ThemedText style={styles.sendText}>{t('chat.send')}</ThemedText>
        </Pressable>
      </View>
    </>
  );
}

/**
 * A crew: while it's an offer, who's in it and yes / no; once it has
 * formed, its members and chat, and ways to report someone or leave.
 */
export default function CrewScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t, languages, showing } = useI18n();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const crew = useCrew(id);
  const names = usePlanNames();
  const respond = useRespondToCrew(id);
  const leave = () => (router.canGoBack() ? router.back() : router.replace('/'));

  const c = crew.data;
  const notFound = crew.error instanceof ConnectError && crew.error.code === Code.NotFound;
  const name = c ? crewName(c, names, (n) => t('crew.name', { name: n })) : null;
  const members = c?.members
    .map((m) =>
      m.me ? `${m.name} (${t('crew.you')})` : m.accepted || c.status === CrewStatus.ACTIVE ? m.name : `${m.name} (${t('crew.deciding')})`,
    )
    .join(', ');

  let body;
  if (notFound) {
    body = (
      <View style={styles.content}>
        <ThemedText themeColor="textSecondary">{t('crew.notFound')}</ThemedText>
      </View>
    );
  } else if (crew.isError) {
    body = (
      <Pressable onPress={() => void crew.refetch()} style={styles.content}>
        <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
        <ThemedText type="link">{t('common.retry')}</ThemedText>
      </Pressable>
    );
  } else if (!c || !name) {
    body = <ActivityIndicator style={styles.fill} />;
  } else if (c.status === CrewStatus.OFFERED) {
    const accepted = c.response === CrewResponse.ACCEPTED;
    body = (
      <>
        <ScrollView contentContainerStyle={styles.content}>
          <View style={styles.titleBlock}>
            <ThemedText type="subtitle">{name.name}</ThemedText>
            {!!name.secondary && <ThemedText themeColor="textSecondary">{name.secondary}</ThemedText>}
          </View>
          <ThemedText type="smallBold">{t('crew.offerTitle')}</ThemedText>
          <ThemedText>{t('crew.offerBody', { date: formatStart(c.offerUntil, lang) })}</ThemedText>
          <View style={styles.section}>
            <ThemedText type="smallBold">{t('crew.members')}</ThemedText>
            <ThemedText themeColor="textSecondary">{members}</ThemedText>
          </View>
          {accepted && (
            <ThemedText type="smallBold" style={{ color: Brand.red }}>
              {t('crew.accepted', { date: formatStart(c.offerUntil, lang) })}
            </ThemedText>
          )}
        </ScrollView>
        <View style={styles.footer}>
          {respond.isError && (
            <ThemedText style={styles.error}>
              {respond.error instanceof ConnectError && respond.error.code === Code.FailedPrecondition ? t('event.tooLate') : t('error.network')}
            </ThemedText>
          )}
          {!accepted && (
            <PrimaryButton label={t('crew.accept')} busy={respond.isPending && respond.variables} onPress={() => respond.mutate(true)} />
          )}
          <Pressable
            accessibilityRole="button"
            disabled={respond.isPending}
            onPress={() => respond.mutate(false, { onSuccess: leave })}
            style={styles.link}>
            <ThemedText type="link">{t('crew.decline')}</ThemedText>
          </Pressable>
        </View>
      </>
    );
  } else {
    body = (
      <>
        <View style={styles.activeHeader}>
          <ThemedText type="subtitle">{name.name}</ThemedText>
          <ThemedText type="small" themeColor="textSecondary">
            {members}
          </ThemedText>
          <View style={styles.links}>
            <Pressable accessibilityRole="button" onPress={() => router.push({ pathname: '/report/[id]', params: { id, crew: '1' } })}>
              <ThemedText type="small" themeColor="textSecondary">
                {t('event.report')}
              </ThemedText>
            </Pressable>
            <Pressable accessibilityRole="button" onPress={() => router.push({ pathname: '/crew-leave/[id]', params: { id } })}>
              <ThemedText type="small" themeColor="textSecondary">
                {t('crew.leave')}
              </ThemedText>
            </Pressable>
          </View>
        </View>
        <CrewChat id={id} />
      </>
    );
  }

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <KeyboardAvoidingView behavior="padding" style={[styles.fill, styles.column]}>
          <View style={styles.header}>
            <Pressable accessibilityRole="button" onPress={leave} hitSlop={8} style={styles.fill}>
              <ThemedText type="link">‹ {t('common.back')}</ThemedText>
            </Pressable>
            <LanguageSwitch />
          </View>
          {body}
        </KeyboardAvoidingView>
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  column: { width: '100%', maxWidth: FormWidth, alignSelf: 'center' },
  header: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: Spacing.four, paddingVertical: Spacing.three },
  content: { padding: Spacing.four, gap: Spacing.four },
  titleBlock: { gap: Spacing.one },
  section: { gap: Spacing.two },
  activeHeader: { paddingHorizontal: Spacing.four, gap: Spacing.one },
  links: { flexDirection: 'row', gap: Spacing.four, paddingTop: Spacing.one },
  footer: { padding: Spacing.four, gap: Spacing.two },
  link: { alignItems: 'center', paddingVertical: Spacing.two },
  messages: { padding: Spacing.three, gap: Spacing.two },
  bubble: { maxWidth: '80%', borderRadius: 16, paddingVertical: Spacing.two, paddingHorizontal: Spacing.three, gap: Spacing.half },
  mine: { alignSelf: 'flex-end', backgroundColor: Brand.red },
  theirs: { alignSelf: 'flex-start' },
  mineText: { color: '#ffffff' },
  suggestion: { borderWidth: 2, borderColor: Brand.gold, borderRadius: 16, padding: Spacing.three, gap: Spacing.one },
  suggestionTitle: { fontSize: 20, lineHeight: 28 },
  center: { textAlign: 'center' },
  error: { color: Brand.red, textAlign: 'center' },
  composer: { flexDirection: 'row', alignItems: 'flex-end', gap: Spacing.two, padding: Spacing.three, borderTopWidth: 1 },
  input: { flex: 1, borderRadius: 16, paddingHorizontal: Spacing.three, paddingVertical: Spacing.two, maxHeight: 120, fontSize: 16 },
  send: { backgroundColor: Brand.red, borderRadius: 16, paddingHorizontal: Spacing.three, paddingVertical: Spacing.two },
  sendText: { color: '#ffffff', fontWeight: '600' },
  disabled: { opacity: 0.4 },
});
