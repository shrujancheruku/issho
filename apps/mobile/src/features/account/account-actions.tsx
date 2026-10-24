import { useMutation } from '@tanstack/react-query';
import { Alert, Pressable, StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useAuth } from '@/features/auth/auth-provider';
import { useI18n } from '@/i18n/i18n-provider';
import { api } from '@/lib/api';

/** Sign-out and account-deletion controls, shown wherever account settings live. */
export function AccountActions() {
  const { session, signOut } = useAuth();
  const { t } = useI18n();
  const deleteAccount = useMutation({
    mutationFn: () => api.identity.deleteAccount({}),
    // The server has deleted the auth user; clear the local session too.
    onSuccess: () => signOut(),
    onError: () => Alert.alert(t('account.deleteFailed'), t('account.deleteFailedBody')),
  });

  const confirmDelete = () =>
    Alert.alert(
      t('account.deleteTitle'),
      t('account.deleteBody'),
      [
        { text: t('common.cancel'), style: 'cancel' },
        { text: t('account.deleteConfirm'), style: 'destructive', onPress: () => deleteAccount.mutate() },
      ],
    );

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
