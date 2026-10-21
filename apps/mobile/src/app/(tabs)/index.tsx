import { useQuery } from '@tanstack/react-query';
import { ActivityIndicator, FlatList, Pressable, RefreshControl, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { LanguageSwitch } from '@/components/language-switch';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BottomTabInset, MaxContentWidth, Spacing, TopTabInset } from '@/constants/theme';
import { useFeed } from '@/features/plans/data';
import { usePlanNames } from '@/features/plans/names';
import { PlanCard } from '@/features/plans/plan-card';
import { useI18n } from '@/i18n/i18n-provider';
import { api } from '@/lib/api';

/** Developer aid: API reachability and build version. Development builds only. */
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

/**
 * The feed: every open plan in the person's city, best for them first, so
 * scrolling far enough shows everything (plans in languages they don't
 * speak come last, greyed out).
 */
export default function FeedScreen() {
  const { t } = useI18n();
  const feed = useFeed();
  const names = usePlanNames();

  const header = (
    <View style={styles.header}>
      <View style={styles.titleRow}>
        <ThemedText type="subtitle" style={styles.grow}>
          {t('feed.title')}
        </ThemedText>
        <LanguageSwitch />
      </View>
      <ThemedText type="small" themeColor="textSecondary">
        {t('feed.intro')}
      </ThemedText>
    </View>
  );

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill} edges={['top', 'left', 'right']}>
        {feed.isError ? (
          <View style={styles.center}>
            <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
            <Pressable onPress={() => void feed.refetch()}>
              <ThemedText type="link">{t('common.retry')}</ThemedText>
            </Pressable>
          </View>
        ) : feed.isPending || !names.ready ? (
          <View style={styles.center}>
            <ActivityIndicator />
          </View>
        ) : (
          <FlatList
            data={feed.data}
            keyExtractor={(p) => p.id}
            renderItem={({ item }) => <PlanCard plan={item} names={names} />}
            ListHeaderComponent={header}
            ListEmptyComponent={
              <View style={styles.empty}>
                <ThemedText type="smallBold">{t('feed.empty')}</ThemedText>
                <ThemedText type="small" themeColor="textSecondary">
                  {t('feed.emptyBody')}
                </ThemedText>
              </View>
            }
            ListFooterComponent={__DEV__ ? <ApiStatus /> : undefined}
            refreshControl={<RefreshControl refreshing={feed.isRefetching} onRefresh={() => void feed.refetch()} />}
            contentContainerStyle={styles.list}
            style={styles.column}
          />
        )}
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  grow: { flex: 1 },
  // Feed and map can use more width than forms (FormWidth) on tablets and the web.
  column: { width: '100%', maxWidth: MaxContentWidth, alignSelf: 'center' },
  list: { padding: Spacing.four, paddingTop: TopTabInset + Spacing.four, paddingBottom: BottomTabInset + Spacing.four, gap: Spacing.three },
  header: { gap: Spacing.two, marginBottom: Spacing.two },
  titleRow: { flexDirection: 'row', alignItems: 'center' },
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', gap: Spacing.two },
  empty: { gap: Spacing.one, paddingVertical: Spacing.four },
});
