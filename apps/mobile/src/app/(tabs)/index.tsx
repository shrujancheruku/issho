import { useQuery } from '@tanstack/react-query';
import { StyleSheet } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { AnimatedIcon } from '@/components/animated-icon';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BottomTabInset, MaxContentWidth, Spacing } from '@/constants/theme';
import { useI18n } from '@/i18n/i18n-provider';
import { api } from '@/lib/api';

/** Developer aid: API reachability and build version. */
function ApiStatus() {
  const health = useQuery({ queryKey: ['health'], queryFn: () => api.health.check({}) });
  if (health.isPending) return <ThemedText type="code">API …</ThemedText>;
  if (health.isError) return <ThemedText type="code">API unreachable</ThemedText>;
  return (
    <ThemedText type="code">
      API {health.data.databaseOk ? 'ok' : 'database down'} · {health.data.version}
    </ThemedText>
  );
}

/** Placeholder until the plan feed (roadmap M6). */
export default function HomeScreen() {
  const { t } = useI18n();
  return (
    <ThemedView style={styles.container}>
      <SafeAreaView style={styles.safeArea}>
        <AnimatedIcon />
        <ThemedText type="subtitle" style={styles.center}>
          {t('home.welcome')}
        </ThemedText>
        <ThemedText themeColor="textSecondary" style={styles.center}>
          {t('home.welcomeBody')}
        </ThemedText>
        <ApiStatus />
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  container: { flex: 1, flexDirection: 'row', justifyContent: 'center' },
  safeArea: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    gap: Spacing.three,
    padding: Spacing.four,
    paddingBottom: BottomTabInset + Spacing.three,
    maxWidth: MaxContentWidth,
  },
  center: { textAlign: 'center' },
});
