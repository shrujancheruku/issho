import { useMutation } from '@tanstack/react-query';
import { Alert, Pressable, StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useAuth } from '@/features/auth/auth-provider';
import { api } from '@/lib/api';

/** Sign-out and account-deletion controls, shown wherever account settings live. */
export function AccountActions() {
  const { session, signOut } = useAuth();
  const deleteAccount = useMutation({
    mutationFn: () => api.identity.deleteAccount({}),
    // The server has deleted the auth user; clear the local session too.
    onSuccess: () => signOut(),
    onError: () => Alert.alert('Could not delete account', 'Please check your connection and try again.'),
  });

  const confirmDelete = () =>
    Alert.alert(
      'Delete your account?',
      'This permanently deletes your account, preferences and history. It cannot be undone.',
      [
        { text: 'Cancel', style: 'cancel' },
        { text: 'Delete', style: 'destructive', onPress: () => deleteAccount.mutate() },
      ],
    );

  return (
    <View style={styles.container}>
      <ThemedText type="small" themeColor="textSecondary">
        Signed in as {session?.user.email}
      </ThemedText>
      <Pressable accessibilityRole="button" onPress={signOut} style={styles.button}>
        <ThemedText>Sign out</ThemedText>
      </Pressable>
      <Pressable
        accessibilityRole="button"
        disabled={deleteAccount.isPending}
        onPress={confirmDelete}
        style={styles.button}>
        <ThemedText style={styles.destructive}>
          {deleteAccount.isPending ? 'Deleting…' : 'Delete account'}
        </ThemedText>
      </Pressable>
    </View>
  );
}

const styles = StyleSheet.create({
  container: { gap: Spacing.two, alignItems: 'center' },
  button: { paddingVertical: Spacing.two, paddingHorizontal: Spacing.three },
  destructive: { color: '#BC002D' },
});
