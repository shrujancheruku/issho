import { useMutation } from '@tanstack/react-query';
import { Pressable, StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useAuth } from '@/features/auth/auth-provider';
import { useI18n } from '@/i18n/i18n-provider';
import { api } from '@/lib/api';
import { confirm, notify } from '@/lib/dialogs';

/** Sign-out and account-deletion controls, shown wherever account settings live. */
export function AccountActions() {
  const { session, signOut } = useAuth();
  const { t } = useI18n();
  const deleteAccount = useMutation({
    mutationFn: () => api.identity.deleteAccount({}),
    // The server has deleted the auth user; clear the local session too.
    onSuccess: () => signOut(),
    onError: () => notify(t('account.deleteFailed'), t('account.deleteFailedBody')),
  });

  const confirmDelete = () =>
    confirm({
      title: t('account.deleteTitle'),
      body: t('account.deleteBody'),
      confirm: t('account.deleteConfirm'),
      cancel: t('common.cancel'),
      onConfirm: () => deleteAccount.mutate(),
    });

  return (
    <View style={styles.container}>
      <ThemedText type="small" themeColor="textSecondary">
        {t('account.signedInAs', { email: session?.user.email ?? '' })}
      </ThemedText>
      <Pressable accessibilityRole="button" onPress={signOut} style={styles.button}>
        <ThemedText>{t('account.signOut')}</ThemedText>
      </Pressable>
      <Pressable
        accessibilityRole="button"
        disabled={deleteAccount.isPending}
        onPress={confirmDelete}
        style={styles.button}>
        {/* Plain text, not red: red is for errors; the confirmation guards it. */}
        <ThemedText>
          {deleteAccount.isPending ? t('account.deleting') : t('account.delete')}
        </ThemedText>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { gap: Spacing.two, alignItems: 'center' },
  button: { paddingVertical: Spacing.two, paddingHorizontal: Spacing.three },
});
