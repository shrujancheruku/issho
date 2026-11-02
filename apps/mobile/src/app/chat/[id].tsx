import { useLocalSearchParams } from 'expo-router';
import { StyleSheet, View } from 'react-native';

import { ScreenFrame, useLeave } from '@/components/screen-frame';
import { BackLink } from '@/components/text-link';
import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { ChatPanel } from '@/features/chat/chat-panel';
import { useI18n } from '@/i18n/i18n-provider';

/**
 * An event's group chat full screen, opened from "Open chat" on
 * the outing screen (event/[id]), where the chat also sits inline.
 */
export default function ChatScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t } = useI18n();
  const leave = useLeave({ pathname: '/event/[id]', params: { id } });
  const header = (
    <View style={styles.header}>
      <BackLink label={t('common.back')} onPress={leave} />
      <ThemedText type="smallBold" style={styles.title}>
        {t('chat.title')}
      </ThemedText>
      <View style={styles.side} />
    </View>
  );

  return (
    <ScreenFrame header={header}>
      <ChatPanel room={{ eventId: id }} />
    </ScreenFrame>
  );
}

const styles = StyleSheet.create({
  header: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: Spacing.three, paddingVertical: Spacing.one },
  title: { flex: 1, textAlign: 'center' },
  side: { width: 64 },
});
