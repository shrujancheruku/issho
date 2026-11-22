import { Link, useLocalSearchParams } from 'expo-router';
import { StyleSheet, Text } from 'react-native';

import { useApi } from '@/api/api-provider';
import { PlanDetails } from '@/components/plan-details';
import { SignupPanel } from '@/components/signup-panel';
import { ScreenFrame } from '@/components/screen-frame';
import { FontSize } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { loadPlan, type PlanPageData } from '@/plans/load';
import { planForServer, planMetadataForServer } from '@/plans/server-loaders';
import { useServerData } from '@/plans/server-data';
import { useLoaded } from '@/plans/use-loaded';

/** Runs on the Expo server, so the plan's text and its link card are in the HTML a client with no JavaScript gets. */
export async function loader(_request: unknown, params: Record<string, string | string[]>) {
  return planForServer(String(params.id));
}

/** Runs on the Expo server too: the tab title and link-card tags. */
export async function generateMetadata(_request: unknown, params: Record<string, string | string[]>) {
  return planMetadataForServer(String(params.id));
}

/** The one screen a plan is ever drawn on (docs/spec/app-shape.md). */
export default function PlanPage() {
  const { t } = useI18n();
  const theme = useTheme();
  const api = useApi();
  const { id } = useLocalSearchParams<{ id: string }>();
  const seed = useServerData<PlanPageData | null>() ?? undefined;
  const { state, retry } = useLoaded((lang) => loadPlan(api, id, lang), seed, id);
  const plan = state.status === 'ready' ? state.data.plan : null;
  return (
    <ScreenFrame>
      <Link href="/plans" style={[styles.body, { color: theme.link }]}>
        {t('plan.seeAll')}
      </Link>
      {state.status === 'ready' && plan ? (
        <>
          <PlanDetails plan={plan} />
          {/* Client-only: it asks who the visitor is after hydration, so the cached page holds nothing per visitor. */}
          <SignupPanel plan={plan} />
        </>
      ) : null}
      {state.status === 'ready' && !plan ? <Text style={[styles.body, { color: theme.text }]}>{t('plan.notFound')}</Text> : null}
      {state.status === 'failed' ? (
        <>
          <Text style={[styles.body, { color: theme.error }]}>{t('plan.loadFailed')}</Text>
          <Text accessibilityRole="button" onPress={retry} style={[styles.body, { color: theme.link }]}>
            {t('plans.retry')}
          </Text>
        </>
      ) : null}
    </ScreenFrame>
  );
}

const styles = StyleSheet.create({
  body: { fontSize: FontSize.default },
});
