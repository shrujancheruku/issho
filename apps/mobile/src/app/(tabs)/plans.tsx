import { useMemo, useState } from 'react';
import { ActivityIndicator, RefreshControl, ScrollView, SectionList, StyleSheet, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import Svg, { Circle, Path } from 'react-native-svg';

import { Chip } from '@/components/chip';
import { MapSheet } from '@/components/map-sheet';
import { PressableScale } from '@/components/pressable-scale';
import { ScreenHeader, SectionTitle } from '@/components/screen-header';
import { TabIcon } from '@/components/tab-icons';
import { TextLink } from '@/components/text-link';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BodyFont, BottomTabInset, MaxContentWidth, Spacing } from '@/constants/theme';
import { useFeed } from '@/features/plans/data';
import { levelColors } from '@/features/plans/level';
import { pinsFor } from '@/features/plans/map-data';
import { usePlanNames } from '@/features/plans/names';
import { PlanCard } from '@/features/plans/plan-card';
import { PlansMap } from '@/features/plans/plans-map';
import { planCategories, planSections, type PlanFilter, type PlanSection } from '@/features/plans/sections';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { english, type StringKey } from '@/i18n/strings';
import { haptics } from '@/lib/haptics';

/**
 * Plans: finding the next outing. The feed's ranking shows as sections
 * (people you get on with, then by interest level), with search and filters
 * on the device. The map is the same plans by area.
 */
export default function PlansScreen() {
  const { t } = useI18n();
  const feed = useFeed();
  const names = usePlanNames();
  const [view, setView] = useState<'list' | 'map'>('list');

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill} edges={['top', 'left', 'right']}>
        {feed.isError ? (
          <View style={styles.center}>
            <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
            <TextLink label={t('common.retry')} onPress={() => void feed.refetch()} center />
          </View>
        ) : feed.isPending || !names.ready ? (
          <View style={styles.center}>
            <ActivityIndicator />
          </View>
        ) : view === 'map' ? (
          <MapView onList={() => setView('list')} />
        ) : (
          <ListView onMap={() => setView('map')} />
        )}
      </SafeAreaView>
    </ThemedView>
  );
}

function ViewToggle({ to, onPress }: { to: 'map' | 'list'; onPress: () => void }) {
  const theme = useTheme();
  const { t } = useI18n();
  return (
    <PressableScale accessibilityRole="button" onPress={onPress} style={[styles.toggle, { backgroundColor: theme.backgroundElement }]}>
      {to === 'map' && <TabIcon name="plans" color={theme.text} size={18} />}
      <ThemedText type="smallBold">{t(to === 'map' ? 'feed.map' : 'feed.list')}</ThemedText>
    </PressableScale>
  );
}

function SearchField({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const theme = useTheme();
  const { t } = useI18n();
  return (
    <View style={[styles.search, { backgroundColor: theme.surface, borderColor: theme.backgroundSelected }]}>
      <Svg width={18} height={18} viewBox="0 0 20 20" fill="none" stroke={theme.textSecondary} strokeWidth={2}>
        <Circle cx={8.5} cy={8.5} r={6} />
        <Path d="M13 13l5 5" />
      </Svg>
      <TextInput
        value={value}
        onChangeText={onChange}
        accessibilityLabel={t('plans.search')}
        placeholder={t('plans.searchPlaceholder')}
        placeholderTextColor={theme.textSecondary}
        returnKeyType="search"
        autoCorrect={false}
        style={[styles.searchInput, { color: theme.text }]}
      />
    </View>
  );
}

function categoryLabel(t: (k: StringKey) => string, category: string) {
  const key = `category.${category}`;
  // A category added on the server before this build knows it shows its id.
  return key in english ? t(key as StringKey) : category;
}

function ListView({ onMap }: { onMap: () => void }) {
  const { t } = useI18n();
  const theme = useTheme();
  const feed = useFeed();
  const names = usePlanNames();
  const [filter, setFilter] = useState<PlanFilter>({ query: '', nearby: false, category: null });
  const sections = useMemo(() => planSections(feed.data ?? [], filter, names), [feed.data, filter, names]);
  const categories = useMemo(() => planCategories(feed.data ?? [], names.category), [feed.data, names]);
  const filtering = !!filter.query.trim() || filter.nearby || !!filter.category;

  const header = (
    <View style={styles.header}>
      <ScreenHeader title={t('plans.title')} end={<ViewToggle to="map" onPress={onMap} />} />
      <SearchField value={filter.query} onChange={(query) => setFilter((f) => ({ ...f, query }))} />
      <View style={styles.chips}>
        <Chip
          label={t('plans.nearby')}
          selected={filter.nearby}
          onPress={() => {
            haptics.selection();
            setFilter((f) => ({ ...f, nearby: !f.nearby }));
          }}
        />
        {categories.map((c) => (
          <Chip
            key={c}
            label={categoryLabel(t, c)}
            selected={filter.category === c}
            onPress={() => {
              haptics.selection();
              setFilter((f) => ({ ...f, category: f.category === c ? null : c }));
            }}
          />
        ))}
      </View>
    </View>
  );

  return (
    <SectionList<PlanSection['data'][number], PlanSection>
      sections={sections}
      keyExtractor={(p) => p.id}
      renderItem={({ item, section }) => (
        <PlanCard plan={item} names={names} showFriends={section.key === 'friends'} />
      )}
      renderSectionHeader={({ section }) => {
        const mark = section.level ? levelColors(section.level, theme) : null;
        return (
          <View style={[styles.sectionHeader, { backgroundColor: theme.background }]}>
            <SectionTitle accessory={mark ? <View style={[styles.swatch, { backgroundColor: mark.background }]} /> : undefined}>
              {t(section.title)}
            </SectionTitle>
          </View>
        );
      }}
      ListHeaderComponent={header}
      ListEmptyComponent={
        <View style={styles.empty}>
          <ThemedText style={styles.bold}>{filtering ? t('plans.noMatch') : t('feed.empty')}</ThemedText>
          {!filtering && (
            <ThemedText type="small" themeColor="textSecondary">
              {t('feed.emptyBody')}
            </ThemedText>
          )}
        </View>
      }
      // Headers would cover the search field as they stick; sections are short.
      stickySectionHeadersEnabled={false}
      keyboardDismissMode="on-drag"
      keyboardShouldPersistTaps="handled"
      ItemSeparatorComponent={Gap}
      refreshControl={<RefreshControl refreshing={feed.isRefetching} onRefresh={() => void feed.refetch()} />}
      contentContainerStyle={styles.list}
      style={styles.column}
      initialNumToRender={8}
      windowSize={7}
    />
  );
}

function Gap() {
  return <View style={styles.gap} />;
}

/**
 * The same plans by area: one pin per area over the whole space, and the
 * picked area's plans in a sheet over the map that drags between low, half
 * and nearly full.
 */
function MapView({ onList }: { onList: () => void }) {
  const { t } = useI18n();
  const feed = useFeed();
  const names = usePlanNames();
  const [selected, setSelected] = useState<string | null>(null);
  // The space under the header, for the sheet's stops; 0 until measured.
  const [height, setHeight] = useState(0);
  const pins = useMemo(() => pinsFor(feed.data ?? [], names.coords), [feed.data, names.coords]);
  const pin = pins.find((p) => p.areaId === selected);
  return (
    <View style={[styles.fill, styles.column]}>
      <View style={styles.mapHeader}>
        <ScreenHeader title={t('plans.title')} end={<ViewToggle to="list" onPress={onList} />} />
      </View>
      <View style={styles.fill} onLayout={(e) => setHeight(e.nativeEvent.layout.height)}>
        {height > 0 && (
          <>
            <PlansMap
              pins={pins}
              selected={selected}
              onSelect={(id) => {
                haptics.selection();
                setSelected(id);
              }}
              home={names.home}
              // The sheet starts at half: frame the pins in the half above it.
              bottomInset={height / 2}
            />
            <MapSheet
              height={height}
              bottom={BottomTabInset}
              raiseOn={selected}
              label={pin ? names.area(pin.areaId).name : t('map.sheet')}
              handle={
                pin ? (
                  <ThemedText style={styles.bold} accessibilityRole="header">
                    {names.area(pin.areaId).name}
                    {!!names.area(pin.areaId).secondary && (
                      <ThemedText themeColor="textSecondary"> {names.area(pin.areaId).secondary}</ThemedText>
                    )}
                  </ThemedText>
                ) : (
                  <ThemedText type="small" themeColor="textSecondary">
                    {pins.length ? t('map.pickArea') : t('feed.empty')}
                  </ThemedText>
                )
              }>
              <ScrollView style={styles.fill} contentContainerStyle={styles.panel}>
                {pin?.plans.map((p) => (
                  <PlanCard key={p.id} plan={p} names={names} showLevel />
                ))}
              </ScrollView>
            </MapSheet>
          </>
        )}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  bold: { fontWeight: 700 },
  // Plans can use more width than forms (FormWidth) on tablets and the web.
  column: { width: '100%', maxWidth: MaxContentWidth, alignSelf: 'center' },
  list: { padding: Spacing.three, paddingTop: Spacing.four, paddingBottom: BottomTabInset + Spacing.four },
  header: { gap: Spacing.three, marginBottom: Spacing.two },
  toggle: { flexDirection: 'row', alignItems: 'center', gap: Spacing.one + 2, minHeight: 44, paddingHorizontal: Spacing.three, borderRadius: 999 },
  search: { flexDirection: 'row', alignItems: 'center', gap: Spacing.two, minHeight: 48, borderRadius: 12, borderWidth: 1, paddingHorizontal: Spacing.three },
  searchInput: { flex: 1, fontFamily: BodyFont.regular, fontSize: 16, paddingVertical: Spacing.two },
  chips: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.two },
  sectionHeader: { paddingTop: Spacing.four, paddingBottom: Spacing.two },
  swatch: { width: 14, height: 14, borderRadius: 4 },
  gap: { height: Spacing.two },
  empty: { gap: Spacing.one, paddingVertical: Spacing.four },
  center: { flex: 1, justifyContent: 'center', alignItems: 'center', gap: Spacing.two },
  mapHeader: { paddingHorizontal: Spacing.three, paddingTop: Spacing.four, paddingBottom: Spacing.three },
  panel: { paddingHorizontal: Spacing.three, paddingBottom: BottomTabInset + Spacing.four, gap: Spacing.two },
});
