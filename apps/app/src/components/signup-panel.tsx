import type { Plan, Signup } from '@issho/api-client';
import { useRouter } from 'expo-router';
import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { useApi } from '@/api/api-provider';
import { useAuth } from '@/auth/auth-provider';
import { Button, Choice, ErrorText } from '@/components/form';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { SignUpRefusal, useSignUp } from '@/join/sign-up';
import { formatCalendarDate } from '@/lib/format';
import { useProfileGate } from '@/profile/use-profile-gate';
import { lastSignup, PARTS_OF_DAY, prefill, signupDates, tickKey } from '@/signup/times';
import { useMySignups } from '@/signup/use-my-signups';

const partKeys: Record<(typeof PARTS_OF_DAY)[number], StringKey> = {
  morning: 'signup.part.morning',
  afternoon: 'signup.part.afternoon',
  evening: 'signup.part.evening',
};

/**
 * What a plan's page adds for the person looking at it: the sign-up action, the
 * times to tick, and once signed up the day they will hear by and how many are
 * interested (docs/spec/app-shape.md). It draws nothing until the person is
 * known on this device, so the server-rendered, publicly cached page holds
 * nothing per visitor.
 *
 * A signup runs in this order: signed in, then the profile (`useProfileGate`
 * opens its questions and returns here), then the times, then `useSignUp`,
 * whose refusal says an account that is not admitted needs a join link. The API
 * checks admission before it reads the profile or the ticks, but only the
 * signup call itself tells the app, and it is the one entry point.
 */
export function SignupPanel({ plan }: { plan: Plan }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const api = useApi();
  const auth = useAuth();
  const router = useRouter();
  const gate = useProfileGate();
  const { state, reload } = useMySignups();
  const { signUp, refusal } = useSignUp();
  const [picking, setPicking] = useState<Set<string> | null>(null);
  const [empty, setEmpty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<StringKey | null>(null);
  // The page's count is from the cached copy; after a change the count is asked for again.
  const [fresh, setFresh] = useState<number | null>(null);
  const count = fresh ?? plan.signedUp;

  if (auth.status !== 'signedIn') {
    if (auth.status === 'loading') return null;
    return <Button label={t('signup.signInFirst')} onPress={() => router.push({ pathname: '/sign-in', params: { next: `/plans/${plan.id}` } })} />;
  }
  // The same error state as the rest of the page, and the same way out.
  if (state.status === 'failed') {
    return (
      <View style={styles.action}>
        <ErrorText>{t('plan.loadFailed')}</ErrorText>
        <Button quiet label={t('plans.retry')} onPress={reload} />
      </View>
    );
  }
  if (state.status !== 'ready') return null;

  const mine: Signup | undefined = state.signups.find((s) => s.planId === plan.id);
  const dates = signupDates(new Date(), plan.timeZone, plan.roundEnds);
  const allowed = PARTS_OF_DAY.filter((p) => plan.partsOfDay.includes(p));
  // From this plan's own ticks when changing them; else from the person's last signup, the newest by when it was made.
  const startFrom = () => prefill(mine ? mine.ticks : (lastSignup(state.signups)?.ticks ?? []), dates, allowed);

  const begin = async () => {
    setBusy(true);
    setProblem(null);
    try {
      // Returns false after opening the profile's questions: the person comes back to this page and starts again.
      if (!(await gate(`/plans/${plan.id}`))) return;
      setEmpty(false);
      setPicking(startFrom());
    } catch {
      setProblem('signUp.failed');
    } finally {
      setBusy(false);
    }
  };

  const toggle = (key: string) => {
    setEmpty(false);
    setPicking((cur) => {
      const next = new Set(cur);
      if (!next.delete(key)) next.add(key);
      return next;
    });
  };

  const save = async () => {
    if (!picking || picking.size === 0) return setEmpty(true);
    setBusy(true);
    const ticks = dates.flatMap((date) => allowed.filter((part) => picking.has(tickKey(date, part))).map((part) => ({ date, part })));
    const done = await signUp(plan.id, ticks);
    if (done) {
      setPicking(null);
      await reload();
      // The page's count is from the cached copy; ask again so it includes this signup.
      api.plans.getPlan({ id: plan.id, displayLanguage: lang }).then(
        (res) => res.plan && setFresh(res.plan.signedUp),
        () => {},
      );
    }
    setBusy(false);
  };

  const withdraw = async () => {
    setBusy(true);
    setProblem(null);
    try {
      await api.signup.withdrawSignup({ planId: plan.id });
      await reload();
      api.plans.getPlan({ id: plan.id, displayLanguage: lang }).then(
        (res) => res.plan && setFresh(res.plan.signedUp),
        () => {},
      );
    } catch {
      setProblem('signup.withdrawFailed');
    } finally {
      setBusy(false);
    }
  };

  if (picking) {
    return (
      <View style={[styles.box, { backgroundColor: theme.surface, borderColor: theme.backgroundSelected }]}>
        <Text accessibilityRole="header" style={[styles.heading, { color: theme.text }]}>
          {t('signup.timesTitle')}
        </Text>
        <Text style={[styles.body, { color: theme.textSecondary }]}>{t('signup.timesHelp')}</Text>
        {dates.map((date) => (
          <View key={date} style={styles.day}>
            <Text style={[styles.body, { color: theme.text }]}>{formatCalendarDate(date, lang)}</Text>
            <View style={styles.parts}>
              {allowed.map((part) => {
                const key = tickKey(date, part);
                return (
                  <View key={part} style={styles.part}>
                    <Choice kind="checkbox" label={`${formatCalendarDate(date, lang)} ${t(partKeys[part])}`} selected={picking.has(key)} onPress={() => toggle(key)} />
                  </View>
                );
              })}
            </View>
          </View>
        ))}
        {empty ? <ErrorText>{t('signup.pickOne')}</ErrorText> : null}
        <SignUpRefusal refusal={refusal} />
        <Button label={mine ? t('signup.save') : t('signup.action')} onPress={save} disabled={busy} />
        <Button quiet label={t('signup.cancel')} onPress={() => setPicking(null)} disabled={busy} />
      </View>
    );
  }

  if (mine) {
    return (
      <View style={[styles.box, { backgroundColor: theme.surface, borderColor: theme.backgroundSelected }]}>
        <Text style={[styles.signedUp, { color: theme.text }]}>{t('signup.signedUp', { date: formatCalendarDate(mine.hearBy, lang) })}</Text>
        <Text style={[styles.body, { color: theme.textSecondary }]}>{t('signup.interested', { count })}</Text>
        {problem ? <ErrorText>{t(problem)}</ErrorText> : null}
        <Button label={t('signup.change')} onPress={begin} disabled={busy} />
        <Button quiet label={t('signup.withdraw')} onPress={withdraw} disabled={busy} />
      </View>
    );
  }

  return (
    <View style={styles.action}>
      {problem ? <ErrorText>{t(problem)}</ErrorText> : null}
      <SignUpRefusal refusal={refusal} />
      <Button label={t('signup.action')} onPress={begin} disabled={busy} />
    </View>
  );
}

const styles = StyleSheet.create({
  box: { borderWidth: 1, borderRadius: 12, padding: Spacing.three, gap: Spacing.two },
  action: { gap: Spacing.two },
  heading: { fontSize: FontSize.heading, fontWeight: '700' },
  signedUp: { fontSize: FontSize.default, fontWeight: '700' },
  body: { fontSize: FontSize.default },
  day: { gap: Spacing.one },
  parts: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.one },
  part: { flexGrow: 1 },
});
