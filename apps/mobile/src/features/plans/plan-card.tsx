import { PlanLevel, type FeedPlan } from '@issho/api-client';
import { router } from 'expo-router';
import { StyleSheet, View } from 'react-native';

import { PressableScale } from '@/components/pressable-scale';
import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n, useOtherLanguage } from '@/i18n/i18n-provider';

import { LevelTag } from './level-tag';
import type { usePlanNames } from './names';
import { distanceText, friendsText, pairText } from './text';

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
  const { t, pick } = useI18n();
  const otherLanguage = useOtherLanguage();
  const theme = useTheme();
  const greyed = plan.level === PlanLevel.NOT_FOR_YOU;
  const title = pick(plan.title, plan.secondaryTitle);
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
      {showLevel && <LevelTag level={plan.level} style={styles.tag} />}
      {showFriends && plan.friendsInterested > 0 && (
        <ThemedText type="smallBold" themeColor="goldText">
          {friendsText(t, plan.friendsInterested)}
        </ThemedText>
      )}
      <ThemedText type="subtitle">
        {title.name}
      </ThemedText>
      {!!title.secondary && (
        <ThemedText type="small" themeColor="textSecondary" lang={otherLanguage}>
          {title.secondary}
        </ThemedText>
      )}
      <View style={styles.bottom}>
        <ThemedText type="small" themeColor="textSecondary" style={styles.grow}>
          {t(language ? 'plan.areaDistanceLanguage' : 'plan.areaDistance', {
            area: pairText(names.area(plan.areaId)),
            distance: distanceText(t, plan.travelMinutes),
            language,
          })}
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
  tag: { alignSelf: 'flex-start', marginBottom: Spacing.one },
  bottom: { flexDirection: 'row', alignItems: 'flex-end', gap: Spacing.two, marginTop: Spacing.one },
  grow: { flex: 1 },
});
