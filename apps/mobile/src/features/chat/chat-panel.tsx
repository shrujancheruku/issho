import { type FeedPlan, maxLen, SendMessageRequestSchema } from '@issho/api-client';
import { router } from 'expo-router';
import { useEffect, useRef, useState, type ReactElement } from 'react';
import { ActivityIndicator, FlatList, Pressable, StyleSheet, TextInput, View } from 'react-native';
import Animated, { FadeInDown } from 'react-native-reanimated';

import { ErrorLine, refreshFailedText } from '@/components/error-line';
import { PressableScale } from '@/components/pressable-scale';
import { LoadFailed } from '@/components/query-screen';
import { ThemedText } from '@/components/themed-text';
import { EaseOut, Motion } from '@/constants/motion';
import { BodyFont, Brand, Spacing } from '@/constants/theme';
import { usePlanCards } from '@/features/plans/data';
import { formatMoment, lastSignupDay } from '@/features/plans/format';
import { usePlanNames } from '@/features/plans/names';
import { pairText } from '@/features/plans/text';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

import { suggestedPlans, type Message } from './messages';
import { useRoom, type RoomRef } from './room';
import { TypingDots } from './typing-dots';

// The server's rule on the message body (chat.proto), not a copy of it.
const MaxMessage = maxLen(SendMessageRequestSchema, 'body');

// Messages that arrive while the chat is open rise in from the bottom; the
// history shown on opening doesn't (animate changes, not screens).
const rise = FadeInDown.duration(Motion.move).easing(EaseOut);

/**
 * A plan Issho suggested in a crew's chat: what, where and when signups
 * close. The message carries only the plan's id; the card (`plan`) comes from
 * one GetPlanCards call for all of a room's suggestions (usePlanCards), and
 * tapping opens the plan screen, which calls GetPlan.
 */
export function SuggestedPlan({ planId, plan }: { planId: string; plan?: FeedPlan }) {
  const { t, lang, pick } = useI18n();
  const theme = useTheme();
  const names = usePlanNames();
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
            {pick(plan.title, plan.secondaryTitle).name}
          </ThemedText>
          <ThemedText type="small" themeColor="textSecondary">
            {t('plan.areaSignupBy', { area: pairText(names.area(plan.areaId)), date: lastSignupDay(plan.signupDeadline, lang) })}
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
 * A chat room in place: an outing group's (open from a few days before
 * until after it, for the people going) or a crew's. What the person may do
 * is the server's decision (useRoom's status); new messages arrive over the
 * room's socket. `header` scrolls above the messages: a pinned meeting point
 * or suggested plan.
 */
export function ChatPanel({ room, header }: { room: RoomRef; header?: ReactElement }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const { messages, status, opensAt, send, typing, onTyping, retry, refreshError } = useRoom(room);
  // One GetPlanCards call for every suggestion in the room (not a GetPlan each).
  const cards = usePlanCards(suggestedPlans(messages));
  const [draft, setDraft] = useState('');
  const list = useRef<FlatList<Message>>(null);

  // The newest message when the history arrived: later ones rise in.
  const [baseline, setBaseline] = useState<number | null>(null);
  const loaded = status !== 'loading' && status !== 'error';
  if (baseline === null && loaded) setBaseline(messages.length ? messages[messages.length - 1].id : 0);

  // Keep the newest message in view.
  useEffect(() => {
    if (messages.length) list.current?.scrollToEnd({ animated: true });
  }, [messages.length]);

  const crew = 'crewId' in room;
  const open = status === 'open';
  let note = '';
  if (status === 'notMember') note = t('chat.notGoing');
  else if (status === 'notOpen' && opensAt !== undefined) note = t('chat.opensOn', { date: formatMoment(opensAt, lang) });
  else if (status === 'notOpen' || status === 'closed') note = t('chat.closed');

  const submit = () => {
    const body = draft.trim();
    if (!body || send.isPending) return;
    send.mutate(body, { onSuccess: () => setDraft('') });
  };
  const canSend = !!draft.trim() && !send.isPending;

  return (
    <>
      <ErrorLine type="small" error={refreshError} text={refreshFailedText} style={styles.refresh} />
      {status === 'loading' ? (
        <ActivityIndicator style={styles.fill} />
      ) : status === 'error' ? (
        <View style={[styles.fill, styles.failed]}>
          <LoadFailed onRetry={retry} center />
        </View>
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
              {item.planId ? <SuggestedPlan planId={item.planId} plan={cards.get(item.planId)} /> : <Bubble m={item} />}
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
      <ErrorLine error={send.error} style={styles.center} />
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
  refresh: { paddingHorizontal: Spacing.three, paddingBottom: Spacing.two },
  messages: { padding: Spacing.three, gap: Spacing.two },
  bubble: { maxWidth: '78%', paddingVertical: Spacing.two, paddingHorizontal: Spacing.three - 4 },
  // The corner nearest the sender is the sharp one.
  mine: { alignSelf: 'flex-end', borderRadius: 16, borderBottomEndRadius: 4 },
  theirs: { alignSelf: 'flex-start', borderRadius: 16, borderBottomStartRadius: 4 },
  who: { fontSize: 12, lineHeight: 16 },
  body: { fontSize: 16, lineHeight: 22 },
  suggestion: { borderWidth: 2, borderColor: Brand.gold, borderRadius: 16, padding: Spacing.three, gap: Spacing.half },
  suggestionTitle: { fontSize: 20, lineHeight: 29 },
  center: { textAlign: 'center' },
  failed: { justifyContent: 'center', padding: Spacing.three, gap: Spacing.two },
  note: { padding: Spacing.three },
  composer: { flexDirection: 'row', alignItems: 'flex-end', gap: Spacing.two, padding: Spacing.three, borderTopWidth: 1 },
  input: { fontFamily: BodyFont.regular, flex: 1, borderRadius: 16, paddingHorizontal: Spacing.three, paddingVertical: Spacing.two, maxHeight: 120, fontSize: 16 },
  send: { backgroundColor: Brand.gold, borderRadius: 16, paddingHorizontal: Spacing.three, minHeight: 44, justifyContent: 'center' },
  sendText: { color: Brand.ink, fontWeight: '700' },
  disabled: { opacity: 0.4 },
  pressed: { opacity: 0.8 },
});
