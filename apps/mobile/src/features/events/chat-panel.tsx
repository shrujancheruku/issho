import { Code, ConnectError } from '@issho/api-client';
import { router } from 'expo-router';
import { useEffect, useRef, useState, type ReactElement } from 'react';
import { ActivityIndicator, FlatList, Pressable, StyleSheet, TextInput, View } from 'react-native';
import Animated, { FadeInDown } from 'react-native-reanimated';

import { PressableScale } from '@/components/pressable-scale';
import { ThemedText } from '@/components/themed-text';
import { EaseOut, Motion } from '@/constants/motion';
import { BodyFont, Brand, Spacing } from '@/constants/theme';
import { usePlan } from '@/features/plans/data';
import { formatStart, lastSignupDay } from '@/features/plans/format';
import { usePlanNames } from '@/features/plans/names';
import { pairText, planTitle } from '@/features/plans/plan-card';
import { useNow } from '@/hooks/use-now';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

import { useChat, type ChatTarget } from './chat';
import type { Message } from './chat-messages';
import { TypingDots } from './typing-dots';

const MaxMessage = 1000; // as the server (plans.MaxChatMessage)

// Messages that arrive while the chat is open rise in from the bottom; the
// history shown on opening doesn't (animate changes, not screens).
const rise = FadeInDown.duration(Motion.move).easing(EaseOut);

/**
 * A plan Issho suggested in a crew's chat: what, where and when signups
 * close, from GetPlan (the message itself carries only the plan's id).
 */
export function SuggestedPlan({ planId }: { planId: string }) {
  const { t, languages, showing } = useI18n();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const theme = useTheme();
  const names = usePlanNames();
  const plan = usePlan(planId).data?.plan;
  return (
    <PressableScale
      accessibilityRole="button"
      onPress={() => router.push({ pathname: '/plan/[id]', params: { id: planId } })}
      style={[styles.suggestion, { backgroundColor: theme.surface }]}>
      <ThemedText type="smallBold" themeColor="goldText">
        {t('crew.suggested')}
      </ThemedText>
      {plan && (
        <>
          <ThemedText type="subtitle" style={styles.suggestionTitle}>
            {planTitle(plan, showing).name}
          </ThemedText>
          <ThemedText type="small" themeColor="textSecondary">
            {pairText(names.area(plan.areaId))}. {t('plan.signupBy')}: {lastSignupDay(plan.signupDeadline, lang)}
          </ThemedText>
          {plan.signedUp && (
            <ThemedText type="smallBold" themeColor="link">
              {t('plan.signedUp')}
            </ThemedText>
          )}
        </>
      )}
    </PressableScale>
  );
}

function Bubble({ m }: { m: Message }) {
  const theme = useTheme();
  return m.mine ? (
    <View style={[styles.bubble, styles.mine, { backgroundColor: theme.accent }]}>
      <ThemedText style={[styles.body, { color: theme.onAccent }]}>{m.body}</ThemedText>
    </View>
  ) : (
    <View style={[styles.bubble, styles.theirs, { backgroundColor: theme.backgroundElement }]}>
      <ThemedText type="smallBold" themeColor="textSecondary" style={styles.who}>
        {m.name}
      </ThemedText>
      <ThemedText style={styles.body}>{m.body}</ThemedText>
    </View>
  );
}

/**
 * A group chat in place: an outing group's (open from a few days before
 * until after it, for the people going) or a crew's (always open). New
 * messages arrive over the socket (useChat). `header` scrolls above the
 * messages: a pinned meeting point or suggested plan.
 */
export function ChatPanel({ target, header }: { target: ChatTarget; header?: ReactElement }) {
  const { t, languages, showing } = useI18n();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const theme = useTheme();
  const now = useNow();
  const { messages, history, send, typing, onTyping } = useChat(target);
  const [draft, setDraft] = useState('');
  const list = useRef<FlatList<Message>>(null);

  // The newest message when the history arrived: later ones rise in.
  const [baseline, setBaseline] = useState<number | null>(null);
  if (baseline === null && history.data) setBaseline(messages.length ? messages[messages.length - 1].id : 0);

  // Keep the newest message in view.
  useEffect(() => {
    if (messages.length) list.current?.scrollToEnd({ animated: true });
  }, [messages.length]);

  const state = history.data;
  const crew = 'crewId' in target;
  const opensAt = state?.opensAt ? Number(state.opensAt.seconds) * 1000 : NaN;
  const closesAt = state?.closesAt ? Number(state.closesAt.seconds) * 1000 : NaN;
  // The server decides; the clock only updates the note while the screen is open.
  const open = !!state && (crew || (now >= opensAt && now < closesAt));
  const denied = history.error instanceof ConnectError && history.error.code === Code.PermissionDenied;

  let note = '';
  if (denied) note = t('chat.notGoing');
  else if (state && !open) note = now < opensAt ? t('chat.opensOn', { date: formatStart(state.opensAt, lang) }) : t('chat.closed');

  const submit = () => {
    const body = draft.trim();
    if (!body || send.isPending) return;
    send.mutate(body, { onSuccess: () => setDraft('') });
  };
  const canSend = !!draft.trim() && !send.isPending;

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
          ListHeaderComponent={header}
          ListEmptyComponent={
            !note ? (
              <ThemedText type="small" themeColor="textSecondary" style={styles.center}>
                {t('chat.empty')}
              </ThemedText>
            ) : undefined
          }
          renderItem={({ item }) => (
            <Animated.View entering={baseline !== null && item.id > baseline ? rise : undefined}>
              {item.planId ? <SuggestedPlan planId={item.planId} /> : <Bubble m={item} />}
            </Animated.View>
          )}
          keyboardDismissMode="interactive"
        />
      )}
      <TypingDots names={typing} />
      {!!note && (
        <ThemedText type="small" themeColor="textSecondary" style={[styles.center, styles.note]}>
          {note}
        </ThemedText>
      )}
      {send.isError && (
        <ThemedText themeColor="error" style={styles.center}>
          {t('error.network')}
        </ThemedText>
      )}
      {open && (
        <View style={[styles.composer, { borderTopColor: theme.backgroundSelected, backgroundColor: theme.surface }]}>
          <TextInput
            value={draft}
            onChangeText={(text) => {
              setDraft(text);
              if (text.trim()) onTyping();
            }}
            accessibilityLabel={crew ? t('chat.toCrew') : t('chat.toGroup')}
            placeholder={t('chat.placeholder')}
            placeholderTextColor={theme.textSecondary}
            maxLength={MaxMessage}
            multiline
            style={[styles.input, { color: theme.text, backgroundColor: theme.backgroundElement }]}
          />
          <Pressable
            accessibilityRole="button"
            accessibilityLabel={t('chat.send')}
            disabled={!canSend}
            onPress={submit}
            style={({ pressed }) => [styles.send, !canSend && styles.disabled, pressed && styles.pressed]}>
            <ThemedText style={styles.sendText}>{t('chat.send')}</ThemedText>
          </Pressable>
        </View>
      )}
    </>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  messages: { padding: Spacing.three, gap: Spacing.two },
  bubble: { maxWidth: '78%', paddingVertical: Spacing.two, paddingHorizontal: Spacing.three - 4 },
  // The corner nearest the sender is the sharp one.
  mine: { alignSelf: 'flex-end', borderRadius: 16, borderBottomEndRadius: 4 },
  theirs: { alignSelf: 'flex-start', borderRadius: 16, borderBottomStartRadius: 4 },
  who: { fontSize: 12, lineHeight: 16 },
  body: { fontSize: 16, lineHeight: 22 },
  suggestion: { borderWidth: 2, borderColor: Brand.gold, borderRadius: 16, padding: Spacing.three, gap: Spacing.half },
  suggestionTitle: { fontSize: 20, lineHeight: 28 },
  center: { textAlign: 'center' },
  note: { padding: Spacing.three },
  composer: { flexDirection: 'row', alignItems: 'flex-end', gap: Spacing.two, padding: Spacing.three, borderTopWidth: 1 },
  input: { fontFamily: BodyFont.regular, flex: 1, borderRadius: 16, paddingHorizontal: Spacing.three, paddingVertical: Spacing.two, maxHeight: 120, fontSize: 16 },
  send: { backgroundColor: Brand.gold, borderRadius: 16, paddingHorizontal: Spacing.three, minHeight: 44, justifyContent: 'center' },
  sendText: { color: Brand.ink, fontWeight: '700' },
  disabled: { opacity: 0.4 },
  pressed: { opacity: 0.8 },
});
