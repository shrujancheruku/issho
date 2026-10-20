import { StyleSheet } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Spacing } from '@/constants/theme';
import { AccountActions } from '@/features/account/account-actions';

/**
 * Shown to signed-in users without a profile. The real onboarding flow
 * (area, languages, availability, interests …) arrives in roadmap M5.
 */
export default function OnboardingScreen() {
  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={[styles.fill, styles.content]}>
        <ThemedText type="title">ようこそ!</ThemedText>
        <ThemedText themeColor="textSecondary" style={styles.center}>
          Welcome to Issho. Setting up your profile is coming soon.
        </ThemedText>
        <AccountActions />
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  content: { justifyContent: 'center', alignItems: 'center', padding: Spacing.four, gap: Spacing.four },
  center: { textAlign: 'center' },
});
