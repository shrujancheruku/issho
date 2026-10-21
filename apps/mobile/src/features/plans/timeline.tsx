import type { PlanStep } from '@issho/api-client';
import { Linking, Pressable, StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Brand, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

import type { usePlanNames } from './names';

/**
 * The plan's stops in order, as a timeline: a dot per stop on a vertical
 * line, with the walk between them on the line. Each stop is the activity,
 * then the venue (Google's name in both display languages, fetched live),
 * its address and a link to it in Google Maps.
 */
export function Timeline({ steps, names }: { steps: PlanStep[]; names: ReturnType<typeof usePlanNames> }) {
  const { t, showing } = useI18n();
  const theme = useTheme();
  return (
    <View>
      {steps.map((step, i) => {
        const activity = names.activity(step.activityId);
        const last = i === steps.length - 1;
        // Venue names come primary first; follow the language switch like the rest.
        const [venue, venueOther] =
          showing === 'secondary' && step.venueSecondaryName
            ? [step.venueSecondaryName, step.venueName]
            : [step.venueName, step.venueSecondaryName];
        const next = steps[i + 1];
        return (
          <View key={i} style={styles.stop}>
            <View style={styles.rail}>
              <View style={[styles.dot, { backgroundColor: Brand.red }]} />
              {!last && <View style={[styles.line, { backgroundColor: theme.backgroundSelected }]} />}
            </View>
            <View style={[styles.body, !last && styles.bodyGap]}>
              <ThemedText type="smallBold" style={styles.activity}>
                {activity.name}
              </ThemedText>
              {venue ? (
                <>
                  <ThemedText>
                    {venue}
                    {!!venueOther && <ThemedText themeColor="textSecondary"> · {venueOther}</ThemedText>}
                  </ThemedText>
                  {!!step.venueAddress && (
                    <ThemedText type="small" themeColor="textSecondary">
                      {step.venueAddress}
                    </ThemedText>
                  )}
                  {!!step.venueMapsUrl && (
                    <Pressable accessibilityRole="link" onPress={() => void Linking.openURL(step.venueMapsUrl)} hitSlop={6}>
                      <ThemedText type="small" style={styles.link}>
                        {t('plan.openInMaps')} ↗
                      </ThemedText>
                    </Pressable>
                  )}
                </>
              ) : (
                i === 0 && (
                  <ThemedText type="small" themeColor="textSecondary">
                    {t('plan.meetingPointLater')}
                  </ThemedText>
                )
              )}
              {!!next?.walkMinutes && (
                <ThemedText type="small" themeColor="textSecondary" style={styles.walk}>
                  {t('plan.walk', { minutes: next.walkMinutes })}
                </ThemedText>
              )}
            </View>
          </View>
        );
      })}
    </View>
  );
}

const DOT = 12;

const styles = StyleSheet.create({
  stop: { flexDirection: 'row', gap: Spacing.three },
  // The rail is as tall as the stop: dot at the top, line filling the rest.
  rail: { width: DOT, alignItems: 'center' },
  dot: { width: DOT, height: DOT, borderRadius: DOT / 2, marginTop: 4 },
  line: { width: 2, flex: 1, marginTop: Spacing.one },
  body: { flex: 1, gap: Spacing.half },
  bodyGap: { paddingBottom: Spacing.four },
  activity: { fontSize: 16, lineHeight: 22 },
  link: { color: Brand.red, marginTop: Spacing.half },
  walk: { marginTop: Spacing.three, fontStyle: 'italic' },
});
