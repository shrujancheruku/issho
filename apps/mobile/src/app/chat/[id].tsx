import { Code, ConnectError } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, FlatList, KeyboardAvoidingView, Pressable, StyleSheet, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Brand, FormWidth, Spacing } from '@/constants/theme';
import { useChat } from '@/features/events/chat';
import type { Message } from '@/features/events/chat-messages';
import { formatStart } from '@/features/plans/format';
import { useNow } from '@/hooks/use-now';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

const MaxMessage = 1000; // as the server (plans.MaxChatMessage)

/** An event's group chat, for the people going. */
export default function ChatScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t, languages, showing } = useI18n();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const theme = useTheme();
  const now = useNow();
  const { messages, history, send } = useChat(id);
  const [draft, setDraft] = useState('');
  const list = useRef<FlatList<Message>>(null);
  const leave = () => (router.canGoBack() ? router.back() : router.replace('/'));

  // Keep the newest message in view.
  useEffect(() => {
    if (messages.length) list.current?.scrollToEnd({ animated: true });
  }, [messages.length]);

  const state = history.data;
  const opensAt = state?.opensAt ? Number(state.opensAt.seconds) * 1000 : NaN;
  const closesAt = state?.closesAt ? Number(state.closesAt.seconds) * 1000 : NaN;
  // The server decides; the clock only updates the note while the screen is open.
  const open = !!state && now >= opensAt && now < closesAt;
  const denied = history.error instanceof ConnectError && history.error.code === Code.PermissionDenied;

  const submit = () => {
    const body = draft.trim();
    if (!body || send.isPending) return;
    send.mutate(body, { onSuccess: () => setDraft('') });
  };

  let note = '';
  if (denied) note = t('chat.notGoing');
  else if (state && !open) note = now < opensAt ? t('chat.opensOn', { date: formatStart(state.opensAt, lang) }) : t('chat.closed');

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <KeyboardAvoidingView behavior="padding" style={[styles.fill, styles.column]}>
          <View style={styles.header}>
            <Pressable accessibilityRole="button" onPress={leave} hitSlop={8}>
              <ThemedText type="link">‹ {t('common.back')}</ThemedText>
            </Pressable>
            <ThemedText type="smallBold" style={styles.title}>
              {t('chat.title')}
            </ThemedText>
            <View style={styles.side} />
          </View>
          {history.isPending ? (
            <ActivityIndicator style={styles.fill} />
          ) : (
            <FlatList
              ref={list}
              data={messages}
              keyExtractor={(m) => String(m.id)}
              contentContainerStyle={styles.messages}
              ListEmptyComponent={
                !note ? (
                  <ThemedText type="small" themeColor="textSecondary" style={styles.center}>
                    {t('chat.empty')}
                  </ThemedText>
                ) : undefined
              }
              renderItem={({ item }) => (
                <View style={[styles.bubble, item.mine ? styles.mine : [styles.theirs, { backgroundColor: theme.backgroundElement }]]}>
                  {!item.mine && (
                    <ThemedText type="smallBold" themeColor="textSecondary">
                      {item.name}
                    </ThemedText>
                  )}
                  <ThemedText style={item.mine && styles.mineText}>{item.body}</ThemedText>
                </View>
              )}
            />
          )}
          {!!note && (
            <ThemedText type="small" themeColor="textSecondary" style={[styles.center, styles.note]}>
              {note}
            </ThemedText>
          )}
          {send.isError && <ThemedText style={styles.error}>{t('error.network')}</ThemedText>}
          {open && (
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
          )}
        </KeyboardAvoidingView>
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  column: { width: '100%', maxWidth: FormWidth, alignSelf: 'center' },
  header: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: Spacing.four, paddingVertical: Spacing.three },
  title: { flex: 1, textAlign: 'center' },
  side: { width: 48 },
  messages: { padding: Spacing.three, gap: Spacing.two },
  bubble: { maxWidth: '80%', borderRadius: 16, paddingVertical: Spacing.two, paddingHorizontal: Spacing.three, gap: Spacing.half },
  mine: { alignSelf: 'flex-end', backgroundColor: Brand.red },
  theirs: { alignSelf: 'flex-start' },
  mineText: { color: '#ffffff' },
  center: { textAlign: 'center' },
  note: { padding: Spacing.three },
  error: { color: Brand.red, textAlign: 'center' },
  composer: { flexDirection: 'row', alignItems: 'flex-end', gap: Spacing.two, padding: Spacing.three, borderTopWidth: 1 },
  input: { flex: 1, borderRadius: 16, paddingHorizontal: Spacing.three, paddingVertical: Spacing.two, maxHeight: 120, fontSize: 16 },
  send: { backgroundColor: Brand.red, borderRadius: 16, paddingHorizontal: Spacing.three, paddingVertical: Spacing.two },
  sendText: { color: '#ffffff', fontWeight: '600' },
  disabled: { opacity: 0.4 },
});
