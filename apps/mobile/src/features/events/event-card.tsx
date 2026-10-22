import type { MyEvent } from '@issho/api-client';
import { router } from 'expo-router';
import { Pressable, StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Brand, Spacing } from '@/constants/theme';
import { useNow } from '@/hooks/use-now';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { formatStart, lastSignupDay } from '@/features/plans/format';
import type { usePlanNames } from '@/features/plans/names';
import { pairText } from '@/features/plans/plan-card';

import { eventState, type EventState } from './state';

/** Status colours: gold needs an answer, red is going, grey is over or out. */
export function toneColor(tone: EventState['tone'], muted: string) {
  return tone === 'action' ? Brand.gold : tone === 'good' ? Brand.red : muted;
}

/** The status line for an event, with the answer-by date filled in. */
export function stateText(t: ReturnType<typeof useI18n>['t'], state: EventState, e: MyEvent, lang: string) {
  // Answer by the day before confirm_by (local midnight), as for signups.
  return t(state.message, { date: lastSignupDay(e.confirmBy, lang) });
}

/** One of the person's events: what, when, and where they stand. */
export function EventCard({ event, names }: { event: MyEvent; names: ReturnType<typeof usePlanNames> }) {
  const { t, languages, showing } = useI18n();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const theme = useTheme();
  const now = useNow();
  const state = eventState(event, now);
  const color = toneColor(state.tone, theme.textSecondary);
  const title = showing === 'secondary' && event.secondaryTitle ? event.secondaryTitle : event.title;
  return (
    <Pressable
      accessibilityRole="button"
      onPress={() => router.push({ pathname: '/event/[id]', params: { id: event.id } })}
      style={({ pressed }) => [
        styles.card,
        { backgroundColor: theme.backgroundElement, borderColor: state.tone === 'muted' ? 'transparent' : color },
        { opacity: state.tone === 'muted' ? 0.6 : pressed ? 0.8 : 1 },
      ]}>
      <ThemedText type="smallBold" style={{ color }}>
        {stateText(t, state, event, lang)}
      </ThemedText>
      <ThemedText type="subtitle" style={styles.title}>
        {title}
      </ThemedText>
      <View style={styles.row}>
        <ThemedText type="small" themeColor="textSecondary" style={styles.grow}>
          {formatStart(event.startsAt, lang)} · {pairText(names.area(event.areaId))}
        </ThemedText>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: { borderRadius: 16, padding: Spacing.three, gap: Spacing.one, borderWidth: 2 },
  title: { fontSize: 20, lineHeight: 28 },
  row: { flexDirection: 'row', alignItems: 'center' },
  grow: { flex: 1 },
});
