import { PlanLevel, type FeedPlan } from '@issho/api-client';
import { router } from 'expo-router';
import { Pressable, StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Brand, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

import { levelStyles } from './level';
import type { usePlanNames } from './names';

/** Short text side by side, as everywhere (docs/architecture.md#languages). */
export function pairText(n: { name: string; secondary: string }) {
  return n.secondary ? `${n.name} · ${n.secondary}` : n.name;
}

/**
 * The plan's title in the language being shown first: the server sends it in
 * the primary display language with the secondary below.
 */
export function planTitle(plan: FeedPlan, showing: 'primary' | 'secondary') {
  return showing === 'secondary' && plan.secondaryTitle
    ? { name: plan.secondaryTitle, secondary: plan.title }
    : { name: plan.title, secondary: plan.secondaryTitle };
}

/** How far a plan is from the person's home area. */
export function distanceText(t: ReturnType<typeof useI18n>['t'], minutes: number) {
  if (minutes < 0) return t('plan.noRoute');
  return minutes === 0 ? t('plan.nearby') : t('plan.minutes', { minutes });
}

/** One plan in the feed: what, where, how it suits you, how many are interested. */
export function PlanCard({ plan, names }: { plan: FeedPlan; names: ReturnType<typeof usePlanNames> }) {
  const { t, showing } = useI18n();
  const theme = useTheme();
  const level = levelStyles[plan.level];
  const greyed = plan.level === PlanLevel.NOT_FOR_YOU;
  const title = planTitle(plan, showing);
  const area = names.area(plan.areaId);
  const distance = distanceText(t, plan.travelMinutes);
  const language = [plan.primaryLanguage, plan.secondaryLanguage].filter(Boolean).map(names.language).join(' / ');

  return (
    // router.push rather than <Link asChild>: Link doesn't pass a function
    // style through to the Pressable, which lost the card's look on the web.
    <Pressable
      accessibilityRole="button"
      onPress={() => router.push({ pathname: '/plan/[id]', params: { id: plan.id } })}
      style={({ pressed }) => [
        styles.card,
        { backgroundColor: theme.backgroundElement, opacity: greyed ? 0.55 : pressed ? 0.8 : 1 },
        plan.signedUp && styles.signedUp,
      ]}>
      <View style={styles.top}>
        <View style={[styles.pill, { backgroundColor: level.background ?? theme.backgroundSelected }]}>
          <ThemedText type="small" style={{ color: level.text ?? theme.textSecondary }}>
            {t(level.label)}
          </ThemedText>
        </View>
        {plan.signedUp && (
          <ThemedText type="smallBold" style={styles.signedUpText}>
            ✓ {t('plan.signedUp')}
          </ThemedText>
        )}
      </View>
      <ThemedText type="subtitle" style={styles.title}>
        {title.name}
      </ThemedText>
      {!!title.secondary && <ThemedText themeColor="textSecondary">{title.secondary}</ThemedText>}
      <ThemedText type="small" themeColor="textSecondary">
        {pairText(area)} · {distance}
      </ThemedText>
      <View style={styles.bottom}>
        <ThemedText type="small" themeColor="textSecondary" style={styles.grow}>
          {language}
        </ThemedText>
        <ThemedText type="smallBold">{t('plan.interested', { count: plan.signups })}</ThemedText>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: { borderRadius: 16, padding: Spacing.three, gap: Spacing.one, borderWidth: 2, borderColor: 'transparent' },
  signedUp: { borderColor: Brand.red },
  signedUpText: { color: Brand.red },
  top: { flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center', marginBottom: Spacing.one },
  pill: { borderRadius: 999, paddingVertical: Spacing.half, paddingHorizontal: Spacing.two },
  title: { fontSize: 22, lineHeight: 30 },
  bottom: { flexDirection: 'row', alignItems: 'center', marginTop: Spacing.one, gap: Spacing.two },
  grow: { flex: 1 },
});
