import { SeatState, type Plan } from '@issho/api-client';
import { useState, type ReactNode } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { useAuth } from '@/auth/auth-provider';
import { Button, ErrorText } from '@/components/form';
import { AnswerButtons, OutingGroupLines, OutingWhen, RefusalText } from '@/components/outing-parts';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { muted, outingOfPlan } from '@/outings/model';
import { stateKey, statusKey, languagesText } from '@/outings/text';
import { useMyOutings } from '@/outings/use-my-outings';

/**
 * The private strip above a plan on the outing's page (docs/spec/app-shape.md):
 * the date and time, the languages the group shares, the first names of the
 * others going, the stage, and cancel. `children` is where the check-in and
 * feedback (#621) go, after the rest. It is drawn only for a signed-in person
 * who has a seat in an outing of this plan, from the outings loaded on the
 * client, so the server-rendered, publicly cached page never holds it.
 */
export function OutingStrip({ plan, children }: { plan: Plan; children?: ReactNode }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const auth = useAuth();
  const { state, reload, cancel } = useMyOutings();
  const [asking, setAsking] = useState(false);
  const [busy, setBusy] = useState(false);

  if (auth.status !== 'signedIn') return null;
  if (state.status === 'failed') {
    return (
      <View style={[styles.box, { backgroundColor: theme.surface, borderColor: theme.backgroundSelected }]}>
        <ErrorText>{t('together.loadFailed')}</ErrorText>
        <Button quiet label={t('plans.retry')} onPress={reload} />
      </View>
    );
  }
  if (state.status !== 'ready') return null;
  const outing = outingOfPlan(state.outings, plan.id, new Date());
  if (!outing) return null;

  const isMuted = muted(outing);
  const going = outing.seatState === SeatState.GOING && !isMuted;
  const confirmCancel = async () => {
    setBusy(true);
    await cancel(outing.id);
    setAsking(false);
    setBusy(false);
  };

  return (
    <View testID="outing-strip" style={[styles.box, { backgroundColor: theme.surface, borderColor: theme.backgroundSelected }, isMuted && styles.muted]}>
      <Text accessibilityRole="header" style={[styles.heading, { color: theme.text }]}>
        {t('outing.stripTitle')}
      </Text>
      <OutingWhen outing={outing} timeZone={plan.timeZone} />
      {going ? (
        <>
          <Text style={[styles.body, { color: theme.text }]}>{t('outing.languages', { languages: languagesText(outing, lang) })}</Text>
          {outing.othersGoing.length > 0 ? <Text style={[styles.body, { color: theme.text }]}>{t('outing.othersGoing', { names: outing.othersGoing.join(', ') })}</Text> : null}
        </>
      ) : (
        <OutingGroupLines outing={outing} />
      )}
      <Text style={[styles.body, { color: theme.textSecondary }]}>{t('outing.stage', { stage: t(stateKey(outing.outingState)) })}</Text>
      {isMuted || outing.seatState !== SeatState.INVITED ? <Text style={[styles.body, { color: theme.textSecondary }]}>{t(statusKey(outing))}</Text> : null}
      {outing.checkedIn ? <Text style={[styles.body, { color: theme.text }]}>{t('outing.checkedIn')}</Text> : null}
      <RefusalText outingId={outing.id} />
      <AnswerButtons outing={outing} />
      {going && !outing.checkedIn ? (
        asking ? (
          <View style={styles.tight}>
            <Text style={[styles.body, { color: theme.text }]}>{t('outing.cancelAsk')}</Text>
            <Button label={t('outing.cancelYes')} onPress={confirmCancel} disabled={busy} />
            <Button quiet label={t('outing.cancelKeep')} onPress={() => setAsking(false)} disabled={busy} />
          </View>
        ) : (
          <Button quiet label={t('outing.cancel')} onPress={() => setAsking(true)} />
        )
      ) : null}
      {children}
    </View>
  );
}

const styles = StyleSheet.create({
  box: { borderWidth: 1, borderRadius: 12, padding: Spacing.three, gap: Spacing.two },
  muted: { opacity: 0.6 },
  tight: { gap: Spacing.two },
  heading: { fontSize: FontSize.heading, fontWeight: '700' },
  body: { fontSize: FontSize.default },
});
