import { PlanLevel, type FeedPlan } from '@issho/api-client';
import { router } from 'expo-router';
import { StyleSheet, View } from 'react-native';

import { PressableScale } from '@/components/pressable-scale';
import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n, useOtherLanguage } from '@/i18n/i18n-provider';

import { levelColors, levelLabels } from './level';
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

/** "Someone you get on with is interested", or how many. */
export function friendsText(t: ReturnType<typeof useI18n>['t'], count: number) {
  return t('plan.friends', { count });
}

/**
 * One plan in Plans: what, where and how far, and how many are interested.
 * Its section already says how it suits the person; `showLevel` adds the
 * level's tag where it doesn't (the map's area list). People you get on
 * with come as a count, never who.
 */
export function PlanCard({
  plan,
  names,
  showLevel = false,
  showFriends = false,
}: {
  plan: FeedPlan;
  names: ReturnType<typeof usePlanNames>;
  showLevel?: boolean;
  showFriends?: boolean;
}) {
  const { t, showing } = useI18n();
  const otherLanguage = useOtherLanguage();
  const theme = useTheme();
  const greyed = plan.level === PlanLevel.NOT_FOR_YOU;
  const title = planTitle(plan, showing);
  const colors = showLevel ? levelColors(plan.level, theme) : null;
  const language = plan.secondaryLanguage
    ? [plan.primaryLanguage, plan.secondaryLanguage].map(names.language).join(' / ')
    : '';

  return (
    // router.push rather than <Link asChild>: Link doesn't pass a function
    // style through to the Pressable, which lost the card's look on the web.
    <PressableScale
      accessibilityRole="button"
      onPress={() => router.push({ pathname: '/plan/[id]', params: { id: plan.id } })}
      style={[styles.card, { backgroundColor: theme.surface }, greyed && styles.greyed]}>
      {colors && (
        <View style={[styles.tag, { backgroundColor: colors.background }]}>
          <ThemedText style={[styles.tagText, { color: colors.text }]}>{t(levelLabels[plan.level])}</ThemedText>
        </View>
      )}
      {showFriends && plan.friendsInterested > 0 && (
        <ThemedText type="smallBold" themeColor="goldText">
          {friendsText(t, plan.friendsInterested)}
        </ThemedText>
      )}
      <ThemedText type="subtitle" style={styles.title}>
        {title.name}
      </ThemedText>
      {!!title.secondary && (
        <ThemedText type="small" themeColor="textSecondary" lang={otherLanguage}>
          {title.secondary}
        </ThemedText>
      )}
      <View style={styles.bottom}>
        <ThemedText type="small" themeColor="textSecondary" style={styles.grow}>
          {pairText(names.area(plan.areaId))}, {distanceText(t, plan.travelMinutes)}
          {language ? `. ${language}` : ''}
        </ThemedText>
        <ThemedText type="smallBold">{t('plan.interested', { count: plan.signups })}</ThemedText>
      </View>
      {plan.signedUp && (
        <ThemedText type="smallBold" themeColor="link">
          {t('plan.signedUp')}
        </ThemedText>
      )}
    </PressableScale>
  );
}

const styles = StyleSheet.create({
  card: { borderRadius: 16, paddingVertical: Spacing.three - 2, paddingHorizontal: Spacing.three, gap: Spacing.half },
  greyed: { opacity: 0.55 },
  tag: { alignSelf: 'flex-start', borderRadius: 999, paddingVertical: Spacing.half, paddingHorizontal: Spacing.two, marginBottom: Spacing.one },
  tagText: { fontSize: 12, lineHeight: 16, fontWeight: 500 },
  title: { fontSize: 20, lineHeight: 28 },
  bottom: { flexDirection: 'row', alignItems: 'flex-end', gap: Spacing.two, marginTop: Spacing.one },
  grow: { flex: 1 },
});
