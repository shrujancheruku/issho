import type { Plan } from '@issho/api-client';
import { StyleSheet, Text, View } from 'react-native';

import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { formatPrice, formatTimeRange } from '@/lib/format';

const legKeys: Record<string, StringKey> = {
  walk: 'plan.leg.walk',
  ride: 'plan.leg.ride',
  transit: 'plan.leg.transit',
};

/** Everything a plan's page says: the plan as anyone sees it, the same for a visitor and a member (docs/spec/app-shape.md). */
export function PlanDetails({ plan }: { plan: Plan }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const label = [styles.label, { color: theme.textSecondary }];
  return (
    <View style={styles.column}>
      <Text accessibilityRole="header" style={[styles.title, { color: theme.text }]}>
        {plan.title}
      </Text>
      {plan.language !== lang ? <Text style={[styles.notice, { color: theme.goldText }]}>{t('plan.notTranslated')}</Text> : null}
      <Text style={[styles.body, { color: theme.text }]}>{plan.description}</Text>

      <View style={styles.row}>
        <Text style={label}>{t('plan.price')}</Text>
        <Text style={[styles.value, { color: theme.text }]}>{plan.price === 0 ? t('plans.free') : formatPrice(plan.price, plan.currency, lang)}</Text>
      </View>
      <View style={styles.row}>
        <Text style={label}>{t('plan.when')}</Text>
        <Text style={[styles.value, { color: theme.text }]}>{formatTimeRange(plan.startsAt, plan.endsAt)}</Text>
      </View>

      <Text accessibilityRole="header" style={[styles.heading, { color: theme.text }]}>
        {t('plan.stops')}
      </Text>
      {plan.stops.map((stop) => (
        <View key={stop.position} style={[styles.stop, { backgroundColor: theme.surface, borderColor: theme.backgroundSelected }]}>
          <Text style={[styles.value, { color: theme.text }]}>{stop.name}</Text>
          <Text style={label}>{t('plan.stay', { minutes: stop.stayMinutes })}</Text>
          {stop.note ? <Text style={[styles.body, { color: theme.text }]}>{stop.note}</Text> : null}
          {stop.leg ? <Text style={label}>{t(legKeys[stop.leg.mode] ?? 'plan.leg.other', { minutes: stop.leg.minutes })}</Text> : null}
        </View>
      ))}

      <Text accessibilityRole="header" style={[styles.heading, { color: theme.text }]}>
        {t('plan.meeting')}
      </Text>
      <Text style={[styles.value, { color: theme.text }]}>{plan.meetingAddress}</Text>
      <Text style={[styles.body, { color: theme.text }]}>{plan.meetingPoint}</Text>

      {plan.whatToBring.length > 0 ? (
        <>
          <Text accessibilityRole="header" style={[styles.heading, { color: theme.text }]}>
            {t('plan.whatToBring')}
          </Text>
          {plan.whatToBring.map((item) => (
            <Text key={item} style={[styles.body, { color: theme.text }]}>
              {item}
            </Text>
          ))}
        </>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  column: { gap: Spacing.two },
  title: { fontSize: FontSize.subtitle, fontWeight: '700' },
  heading: { fontSize: FontSize.heading, fontWeight: '700', marginTop: Spacing.three },
  body: { fontSize: FontSize.default },
  value: { fontSize: FontSize.default, fontWeight: '600' },
  label: { fontSize: FontSize.small },
  notice: { fontSize: FontSize.small },
  row: { gap: Spacing.half },
  stop: { borderWidth: 1, borderRadius: 12, padding: Spacing.three, gap: Spacing.one },
});
