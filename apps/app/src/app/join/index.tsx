import { JoinLinkProblemReason } from '@issho/api-client';
import { Redirect, useRouter } from 'expo-router';
import { useCallback, useEffect, useRef, useState } from 'react';
import { StyleSheet, Text } from 'react-native';

import { useApi } from '@/api/api-provider';
import { useAuth } from '@/auth/auth-provider';
import { Button } from '@/components/form';
import { ScreenFrame } from '@/components/screen-frame';
import { FontSize } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { clearPendingJoin, peekPendingJoin } from '@/join/pending-join';
import { joinLinkReason } from '@/join/problems';

type Outcome = 'redeeming' | 'admitted' | 'unusable' | 'alreadyIn' | 'failed';

/**
 * What opening a join link does (docs/spec/people.md, "Join links"): sign the
 * person in if they are not, make their account, then redeem the link. The code
 * was left in memory by `/join/[code]`; it is forgotten as soon as the API has
 * answered either way, and kept only when no answer came, so "try again" has
 * something to try.
 */
export default function Join() {
  const auth = useAuth();
  // Read once: the answer clears it, and the screen must not then mistake that for "no link".
  const [code] = useState(peekPendingJoin);

  if (!code) return <Redirect href="/plans" />;
  if (auth.status === 'loading') return null;
  if (auth.status === 'signedOut') return <Redirect href={{ pathname: '/sign-in', params: { next: '/join' } }} />;
  return <Redeem code={code} />;
}

const MESSAGE = {
  redeeming: 'join.redeeming',
  admitted: 'join.admittedTitle',
  unusable: 'join.unusable',
  alreadyIn: 'join.alreadyIn',
  failed: 'join.failed',
} as const;

function Redeem({ code }: { code: string }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const api = useApi();
  const router = useRouter();
  const [outcome, setOutcome] = useState<Outcome>('redeeming');
  const started = useRef(false);

  const redeem = useCallback(async () => {
    setOutcome('redeeming');
    try {
      // The account is made by being asked for, and redeeming needs it to exist.
      await api.account.getAccount({ displayLanguage: lang });
      await api.joinLink.redeemJoinLink({ code });
      clearPendingJoin();
      setOutcome('admitted');
    } catch (err) {
      const reason = joinLinkReason(err);
      if (reason === JoinLinkProblemReason.LINK_UNUSABLE) {
        clearPendingJoin();
        setOutcome('unusable');
      } else if (reason === JoinLinkProblemReason.ALREADY_ADMITTED) {
        clearPendingJoin();
        setOutcome('alreadyIn');
      } else {
        setOutcome('failed');
      }
    }
    // The language only matters to the call that makes the account.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [api, code]);

  useEffect(() => {
    // Once, even when the screen mounts twice in development.
    if (started.current) return;
    started.current = true;
    void redeem();
  }, [redeem]);

  const bad = outcome === 'unusable' || outcome === 'failed';
  return (
    <ScreenFrame>
      <Text
        accessibilityRole={outcome === 'admitted' ? 'header' : bad ? 'alert' : undefined}
        style={[outcome === 'admitted' ? styles.title : styles.body, { color: bad ? theme.error : theme.text }]}>
        {t(MESSAGE[outcome])}
      </Text>
      {outcome === 'admitted' ? <Text style={[styles.body, { color: theme.textSecondary }]}>{t('join.admittedBody')}</Text> : null}
      {outcome === 'failed' ? <Button label={t('join.retry')} onPress={redeem} /> : null}
      {outcome !== 'redeeming' && outcome !== 'failed' ? <Button label={t('join.seePlans')} onPress={() => router.replace('/plans')} /> : null}
    </ScreenFrame>
  );
}

const styles = StyleSheet.create({
  title: { fontSize: FontSize.subtitle, fontWeight: '700' },
  body: { fontSize: FontSize.default },
});
