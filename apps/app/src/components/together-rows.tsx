import type { Outing } from '@issho/api-client';
import { Link } from 'expo-router';
import { Pressable, StyleSheet, Text, View } from 'react-native';

import { AnswerButtons, OutingGroupLines, OutingWhen, RefusalText } from '@/components/outing-parts';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { formatCalendarDate } from '@/lib/format';
import { muted } from '@/outings/model';
import { statusKey } from '@/outings/text';

/** An invitation that needs an answer, answered in place (docs/spec/app-shape.md, "Together"). */
export function NeedsYouCard({ outing, title, timeZone }: { outing: Outing; title: string; timeZone: string }) {
  const theme = useTheme();
  const testID = `needs-you-${outing.id}`;
  return (
    <View testID={testID} style={[styles.card, { backgroundColor: theme.surface, borderColor: theme.accent }]}>
      <Link href={{ pathname: '/plans/[id]', params: { id: outing.planId } }} asChild>
        <Pressable accessibilityRole="link">
          <Text style={[styles.value, { color: theme.link }]}>{title}</Text>
        </Pressable>
      </Link>
      <OutingWhen outing={outing} timeZone={timeZone} />
      <OutingGroupLines outing={outing} />
      <RefusalText outingId={outing.id} />
      <AnswerButtons outing={outing} />
    </View>
  );
}

/** A row for the day a signup will hear. */
export function SignupRow({ planId, day, title }: { planId: string; day: string; title: string }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  return (
    <Link href={{ pathname: '/plans/[id]', params: { id: planId } }} asChild>
      <Pressable accessibilityRole="link" style={StyleSheet.flatten([styles.row, { backgroundColor: theme.surface, borderColor: theme.backgroundSelected }])}>
        <Text style={[styles.day, { color: theme.textSecondary }]}>{formatCalendarDate(day, lang)}</Text>
        <Text style={[styles.body, { color: theme.text }]}>{t('together.hearAbout', { plan: title })}</Text>
      </Pressable>
    </Link>
  );
}

/** A row for an outing on its date: the outing's page is the plan's, so it opens the plan. Muted when the outing is not on or the person is out of it. */
export function OutingRow({ outing, title, timeZone }: { outing: Outing; title: string; timeZone: string }) {
  const { t } = useI18n();
  const theme = useTheme();
  const testID = `together-row-${outing.id}`;
  return (
    <Link href={{ pathname: '/plans/[id]', params: { id: outing.planId } }} asChild>
      <Pressable
        testID={testID}
        accessibilityRole="link"
        style={StyleSheet.flatten([styles.row, { backgroundColor: theme.surface, borderColor: theme.backgroundSelected }, muted(outing) && styles.muted])}
      >
        <Text style={[styles.value, { color: theme.text }]}>{title}</Text>
        <OutingWhen outing={outing} timeZone={timeZone} />
        <Text style={[styles.body, { color: theme.textSecondary }]}>{t(statusKey(outing))}</Text>
        <RefusalText outingId={outing.id} />
      </Pressable>
    </Link>
  );
}

const styles = StyleSheet.create({
  card: { borderWidth: 2, borderRadius: 12, padding: Spacing.three, gap: Spacing.two },
  row: { borderWidth: 1, borderRadius: 12, padding: Spacing.three, gap: Spacing.one },
  muted: { opacity: 0.6 },
  day: { fontSize: FontSize.small },
  value: { fontSize: FontSize.default, fontWeight: '700' },
  body: { fontSize: FontSize.default },
});
