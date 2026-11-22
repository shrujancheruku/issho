import { useCallback, useEffect, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { useApi } from '@/api/api-provider';
import { Button } from '@/components/form';
import { NeedsYouCard, OutingRow, SignupRow } from '@/components/together-rows';
import { RequireSignIn } from '@/components/require-sign-in';
import { ScreenFrame } from '@/components/screen-frame';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { useMyOutings } from '@/outings/use-my-outings';
import { loadPlans } from '@/plans/load';
import { useMySignups } from '@/signup/use-my-signups';
import { byDay, itemKey, needsYou, outingItems, signupItems, type TogetherItem } from '@/together/items';
import type { Plan } from '@issho/api-client';

/** Everything of the person's: "Needs you" on top, then everything else by time (docs/spec/app-shape.md). */
export default function Together() {
  return (
    <RequireSignIn path="/together">
      <TogetherList />
    </RequireSignIn>
  );
}

function TogetherList() {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const api = useApi();
  const { state: signups, reload: reloadSignups } = useMySignups();
  const { state: outings, reload: reloadOutings } = useMyOutings();
  const [plans, setPlans] = useState<Plan[] | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const failed = loadFailed || signups.status === 'failed' || outings.status === 'failed';
  const ready = signups.status === 'ready' && outings.status === 'ready';

  useEffect(() => {
    if (!ready) return;
    let live = true;
    // A signup or an outing names its plan by id only; the titles come from the city's list, in the display language.
    loadPlans(api, lang).then(
      (res) => live && (setPlans(res.plans), setLoadFailed(false)),
      () => live && setLoadFailed(true),
    );
    return () => {
      live = false;
    };
  }, [ready, api, lang]);

  const retry = useCallback(() => {
    setLoadFailed(false);
    void reloadSignups();
    void reloadOutings();
  }, [reloadSignups, reloadOutings]);

  const now = new Date();
  const cards = ready && plans ? needsYou(outings.outings, plans, now) : [];
  const rows: TogetherItem[] = ready && plans ? byDay([...signupItems(signups.signups, plans), ...outingItems(outings.outings, plans, now)]) : [];

  return (
    <ScreenFrame>
      <Text accessibilityRole="header" style={[styles.title, { color: theme.text }]}>
        {t('tabs.together')}
      </Text>
      {ready && plans && cards.length + rows.length === 0 ? <Text style={[styles.body, { color: theme.textSecondary }]}>{t('together.empty')}</Text> : null}
      {failed ? (
        <>
          <Text style={[styles.body, { color: theme.error }]}>{t('together.loadFailed')}</Text>
          <Button quiet label={t('plans.retry')} onPress={retry} />
        </>
      ) : null}
      {cards.length > 0 ? (
        <View style={styles.block}>
          <Text accessibilityRole="header" style={[styles.heading, { color: theme.text }]}>
            {t('together.needsYou')}
          </Text>
          {cards.map(({ outing, title, timeZone }) => (
            <NeedsYouCard key={outing.id} outing={outing} title={title} timeZone={timeZone} />
          ))}
        </View>
      ) : null}
      {rows.map((item) =>
        item.kind === 'signup' ? (
          <SignupRow key={itemKey(item)} planId={item.planId} day={item.day} title={item.title} />
        ) : (
          <OutingRow key={itemKey(item)} outing={item.outing} title={item.title} timeZone={item.timeZone} />
        ),
      )}
    </ScreenFrame>
  );
}

const styles = StyleSheet.create({
  title: { fontSize: FontSize.subtitle, fontWeight: '700' },
  heading: { fontSize: FontSize.heading, fontWeight: '700' },
  block: { gap: Spacing.two },
  body: { fontSize: FontSize.default },
});
