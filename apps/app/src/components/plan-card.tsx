import type { Plan } from '@issho/api-client';
import { Link } from 'expo-router';
import { Pressable, StyleSheet, Text } from 'react-native';

import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { formatPrice, formatTimeRange } from '@/lib/format';

/** One plan in the list: its title, what it costs and when it runs; the whole card opens the plan's page. */
export function PlanCard({ plan }: { plan: Plan }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  return (
    <Link href={{ pathname: '/plans/[id]', params: { id: plan.id } }} asChild>
      <Pressable accessibilityRole="link" style={StyleSheet.flatten([styles.card, { backgroundColor: theme.surface, borderColor: theme.backgroundSelected }])}>
        <Text style={[styles.title, { color: theme.text }]}>{plan.title}</Text>
        <Text style={[styles.body, { color: theme.textSecondary }]}>{plan.description}</Text>
        <Text style={[styles.meta, { color: theme.textSecondary }]}>
          {plan.price === 0 ? t('plans.free') : formatPrice(plan.price, plan.currency, lang)}
        </Text>
        <Text style={[styles.meta, { color: theme.textSecondary }]}>{formatTimeRange(plan.startsAt, plan.endsAt)}</Text>
      </Pressable>
    </Link>
  );
}

const styles = StyleSheet.create({
  card: { borderWidth: 1, borderRadius: 12, padding: Spacing.three, gap: Spacing.one },
  title: { fontSize: FontSize.heading, fontWeight: '700' },
  body: { fontSize: FontSize.default },
  meta: { fontSize: FontSize.small },
});
