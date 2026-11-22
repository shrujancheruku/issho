import { Pressable, StyleSheet, Text } from 'react-native';

import { useApi } from '@/api/api-provider';
import { PlanCard } from '@/components/plan-card';
import { ScreenFrame } from '@/components/screen-frame';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { loadPlans, type PlanList } from '@/plans/load';
import { plansForServer } from '@/plans/server-loaders';
import { useServerData } from '@/plans/server-data';
import { useLoaded } from '@/plans/use-loaded';

/** Runs on the Expo server, so the list is in the HTML a client with no JavaScript gets. */
export async function loader() {
  return plansForServer();
}

/** The city's plans, the same for a visitor and a member (docs/spec/app-shape.md). */
export default function Plans() {
  const { t } = useI18n();
  const theme = useTheme();
  const api = useApi();
  const { state, retry } = useLoaded((lang) => loadPlans(api, lang), useServerData<PlanList | null>() ?? undefined, 'plans');
  return (
    <ScreenFrame>
      <Text accessibilityRole="header" style={[styles.title, { color: theme.text }]}>
        {t('tabs.plans')}
      </Text>
      {state.status === 'failed' ? (
        <>
          <Text style={[styles.body, { color: theme.error }]}>{t('plans.loadFailed')}</Text>
          <Pressable accessibilityRole="button" onPress={retry}>
            <Text style={[styles.body, { color: theme.link }]}>{t('plans.retry')}</Text>
          </Pressable>
        </>
      ) : null}
      {state.status === 'ready' && state.data.plans.length === 0 ? <Text style={[styles.body, { color: theme.textSecondary }]}>{t('plans.empty')}</Text> : null}
      {state.status === 'ready' ? state.data.plans.map((plan) => <PlanCard key={plan.id} plan={plan} />) : null}
    </ScreenFrame>
  );
}

const styles = StyleSheet.create({
  title: { fontSize: FontSize.subtitle, fontWeight: '700' },
  body: { fontSize: FontSize.default, paddingVertical: Spacing.one },
});
