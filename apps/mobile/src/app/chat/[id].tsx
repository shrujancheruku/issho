import { router, useLocalSearchParams } from 'expo-router';
import { KeyboardAvoidingView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { BackLink } from '@/components/text-link';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { FormWidth, Spacing } from '@/constants/theme';
import { ChatPanel } from '@/features/chat/chat-panel';
import { useI18n } from '@/i18n/i18n-provider';

/**
 * An event's group chat full screen, opened from "Open chat" on
 * the outing screen (event/[id]), where the chat also sits inline.
 */
export default function ChatScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t } = useI18n();
  const leave = () => (router.canGoBack() ? router.back() : router.replace({ pathname: '/event/[id]', params: { id } }));

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <KeyboardAvoidingView behavior="padding" style={[styles.fill, styles.column]}>
          <View style={styles.header}>
            <BackLink label={t('common.back')} onPress={leave} />
            <ThemedText type="smallBold" style={styles.title}>
              {t('chat.title')}
            </ThemedText>
            <View style={styles.side} />
          </View>
          <ChatPanel room={{ eventId: id }} />
        </KeyboardAvoidingView>
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  column: { width: '100%', maxWidth: FormWidth, alignSelf: 'center' },
  header: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: Spacing.three, paddingVertical: Spacing.one },
  title: { flex: 1, textAlign: 'center' },
  side: { width: 64 },
});
