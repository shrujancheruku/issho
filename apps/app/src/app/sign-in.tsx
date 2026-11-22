import { Redirect, useLocalSearchParams } from 'expo-router';
import { useEffect, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { useAuth } from '@/auth/auth-provider';
import { returnPath } from '@/auth/return-path';
import { RateLimitedError } from '@/auth/session-client';
import { Button, ErrorText, Field } from '@/components/form';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';

/**
 * How long after a send before the same address can be sent to again. Auth
 * limits codes per address; waiting here keeps a person from running into that
 * limit by pressing the button again.
 */
const RESEND_SECONDS = 60;

/**
 * Sign-in is a 6-digit email code, the only way in for the Pilot
 * (docs/spec/people.md). Two steps on one screen: the email, then the code.
 * `?next=` is the screen the person was sent from; they return to it.
 */
export default function SignIn() {
  const { t } = useI18n();
  const theme = useTheme();
  const auth = useAuth();
  const next = returnPath(useLocalSearchParams<{ next?: string }>().next);

  const [email, setEmail] = useState('');
  const [sentTo, setSentTo] = useState<string | null>(null);
  const [code, setCode] = useState('');
  const [problem, setProblem] = useState<StringKey | null>(null);
  const [busy, setBusy] = useState(false);
  // The last address a send was tried for, and the seconds before it may be sent to again.
  const [wait, setWait] = useState<{ email: string; left: number } | null>(null);
  const counting = wait !== null && wait.left > 0;
  useEffect(() => {
    if (!counting) return;
    const tick = setInterval(() => setWait((w) => (w ? { ...w, left: Math.max(0, w.left - 1) } : w)), 1000);
    return () => clearInterval(tick);
  }, [counting]);
  const secondsLeft = (address: string) => (wait && wait.email === address ? wait.left : 0);

  // Already signed in: nothing to ask. This is also where a sign-in lands.
  if (auth.status === 'signedIn') return <Redirect href={next as never} />;

  const sendCode = async (to?: string) => {
    const address = (typeof to === 'string' ? to : email).trim();
    if (!address) return setProblem('signIn.emailRequired');
    if (secondsLeft(address) > 0) return;
    setBusy(true);
    setProblem(null);
    // Whether it worked or Auth said no, the next send to this address waits.
    setWait({ email: address, left: RESEND_SECONDS });
    try {
      await auth.sendCode(address);
      setSentTo(address);
      setCode('');
    } catch (err) {
      setProblem(err instanceof RateLimitedError ? 'signIn.rateLimited' : 'signIn.sendFailed');
    } finally {
      setBusy(false);
    }
  };

  const verify = async () => {
    if (!sentTo) return;
    setBusy(true);
    setProblem(null);
    try {
      // On success the session changes and the redirect above takes the person on.
      if (!(await auth.verifyCode(sentTo, code.trim()))) {
        setProblem('signIn.wrongCode');
        setCode('');
      }
    } catch (err) {
      setProblem(err instanceof RateLimitedError ? 'signIn.rateLimited' : 'signIn.verifyFailed');
    } finally {
      setBusy(false);
    }
  };

  return (
    <View testID="screen" style={[styles.screen, { backgroundColor: theme.background }]}>
      <View style={styles.column}>
        <Text accessibilityRole="header" style={[styles.title, { color: theme.text }]}>
          {t('signIn.title')}
        </Text>
        {sentTo === null ? (
          <>
            <Field
              label={t('signIn.emailLabel')}
              value={email}
              onChangeText={setEmail}
              keyboardType="email-address"
              autoCapitalize="none"
              autoComplete="email"
              autoCorrect={false}
              textContentType="emailAddress"
              onSubmitEditing={() => sendCode()}
            />
            {problem && <ErrorText>{t(problem)}</ErrorText>}
            <Button label={t('signIn.sendCode')} onPress={() => sendCode()} disabled={busy || secondsLeft(email.trim()) > 0} />
          </>
        ) : (
          <>
            <Text style={[styles.body, { color: theme.textSecondary }]}>{t('signIn.codeSent', { email: sentTo })}</Text>
            <Field
              label={t('signIn.codeLabel')}
              value={code}
              onChangeText={setCode}
              keyboardType="number-pad"
              autoComplete="one-time-code"
              textContentType="oneTimeCode"
              maxLength={6}
              onSubmitEditing={verify}
            />
            {problem && <ErrorText>{t(problem)}</ErrorText>}
            <Button label={t('signIn.verify')} onPress={verify} disabled={busy} />
            <Button quiet label={t('signIn.resend')} onPress={() => sendCode(sentTo)} disabled={busy || secondsLeft(sentTo) > 0} />
            {secondsLeft(sentTo) > 0 && <Text style={[styles.body, { color: theme.textSecondary }]}>{t('signIn.resendWait', { seconds: secondsLeft(sentTo) })}</Text>}
            <Button
              quiet
              label={t('signIn.changeEmail')}
              onPress={() => {
                setSentTo(null);
                setProblem(null);
              }}
            />
          </>
        )}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, alignItems: 'center', padding: Spacing.four },
  column: { width: '100%', maxWidth: 420, gap: Spacing.three },
  title: { fontSize: FontSize.subtitle, fontWeight: '700' },
  body: { fontSize: FontSize.default },
});
