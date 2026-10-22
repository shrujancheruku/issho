import { useQuery } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { ActivityIndicator, FlatList, Pressable, RefreshControl, ScrollView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Chip } from '@/components/chip';
import { LanguageSwitch } from '@/components/language-switch';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BottomTabInset, MaxContentWidth, Spacing, TopTabInset } from '@/constants/theme';
import { EventCard } from '@/features/events/event-card';
import { useMyEvents } from '@/features/events/data';
import { useFeed } from '@/features/plans/data';
import { pinsFor } from '@/features/plans/map-data';
import { PlansMap } from '@/features/plans/plans-map';
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
  const events = useMyEvents();
  const names = usePlanNames();
  const [view, setView] = useState<'list' | 'map'>('list');
  const [selected, setSelected] = useState<string | null>(null);
  const pins = useMemo(() => pinsFor(feed.data ?? [], names.coords), [feed.data, names.coords]);
  const selectedPin = pins.find((p) => p.areaId === selected);

  const titleRow = (
    <>
      <View style={styles.titleRow}>
        <ThemedText type="subtitle" style={styles.grow}>
          {t('feed.title')}
        </ThemedText>
        <LanguageSwitch />
      </View>
      <View style={styles.toggle}>
        <Chip label={t('feed.list')} selected={view === 'list'} onPress={() => setView('list')} />
        <Chip label={t('feed.map')} selected={view === 'map'} onPress={() => setView('map')} />
      </View>
    </>
  );

  const header = (
    <View style={styles.header}>
      {titleRow}
      {/* The person's own events first: they may need an answer. */}
      {!!events.data?.length && (
        <View style={styles.events}>
          <ThemedText type="smallBold">{t('events.title')}</ThemedText>
          {events.data.map((e) => (
            <EventCard key={e.id} event={e} names={names} />
          ))}
        </View>
      )}
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
        ) : view === 'map' ? (
          // Map of every open plan: one pin per area; tap one to list its plans below.
          <View style={[styles.fill, styles.column, styles.mapScreen]}>
            <View style={styles.mapHeader}>{titleRow}</View>
            <View style={styles.map}>
              <PlansMap pins={pins} selected={selected} onSelect={setSelected} />
            </View>
            <ScrollView style={styles.fill} contentContainerStyle={styles.panel}>
              {selectedPin ? (
                <>
                  <ThemedText type="smallBold">
                    {names.area(selectedPin.areaId).name}
                  </ThemedText>
                  {selectedPin.plans.map((p) => (
                    <PlanCard key={p.id} plan={p} names={names} />
                  ))}
                </>
              ) : (
                <ThemedText type="small" themeColor="textSecondary">
                  {pins.length ? t('map.pickArea') : t('feed.empty')}
                </ThemedText>
              )}
            </ScrollView>
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
            refreshControl={
              <RefreshControl
                refreshing={feed.isRefetching || events.isRefetching}
                onRefresh={() => void Promise.all([feed.refetch(), events.refetch()])}
              />
            }
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
  events: { gap: Spacing.two, marginVertical: Spacing.two },
  titleRow: { flexDirection: 'row', alignItems: 'center' },
  toggle: { flexDirection: 'row', gap: Spacing.two },
  mapScreen: { paddingTop: TopTabInset },
  mapHeader: { paddingHorizontal: Spacing.four, paddingTop: Spacing.four, paddingBottom: Spacing.three, gap: Spacing.two },
  // Half the screen: enough to see the city, with room for the area's plans below.
  map: { height: '50%', overflow: 'hidden' },
  panel: { padding: Spacing.four, paddingBottom: BottomTabInset + Spacing.four, gap: Spacing.three },
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', gap: Spacing.two },
  empty: { gap: Spacing.one, paddingVertical: Spacing.four },
});
