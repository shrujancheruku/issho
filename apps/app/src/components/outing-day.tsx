import type { Feedback, Outing } from '@issho/api-client';
import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { Button, Choice, ErrorText } from '@/components/form';
import { CheckInOpens, FeedbackCloses } from '@/components/outing-parts';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { canCheckIn, checkInComingUp, feedbackClosed, feedbackOpen, type FeedbackRefusal } from '@/outings/model';
import { useMyOutings } from '@/outings/use-my-outings';

/**
 * Check-in on the outing's strip (docs/spec/outings.md, "On the day"): "I'm
 * here" inside the window the server enforces, then who else has checked in.
 * Before the window it says when it opens. It shows nothing to a person who is
 * not going, and nothing once the plan has ended and they never checked in.
 */
export function CheckInSection({ outing, timeZone }: { outing: Outing; timeZone: string }) {
  const { t } = useI18n();
  const theme = useTheme();
  const { checkIn } = useMyOutings();
  const [busy, setBusy] = useState(false);
  const now = new Date();

  const send = async () => {
    setBusy(true);
    try {
      await checkIn(outing.id);
    } finally {
      // The button never stays disabled behind a refusal or a failure.
      setBusy(false);
    }
  };

  if (outing.checkedIn) {
    return (
      <View style={styles.tight}>
        <Text style={[styles.body, { color: theme.text }]}>{t('outing.checkedIn')}</Text>
        <Text style={[styles.body, { color: theme.text }]}>
          {outing.checkedInNames.length > 0 ? t('outing.checkIn.alsoHere', { names: outing.checkedInNames.join(', ') }) : t('outing.checkIn.noOthers')}
        </Text>
      </View>
    );
  }
  if (canCheckIn(outing, now)) {
    return (
      <View style={styles.tight}>
        <Button label={t('outing.checkIn.button')} onPress={send} disabled={busy} />
      </View>
    );
  }
  if (checkInComingUp(outing, now)) {
    return <CheckInOpens outing={outing} timeZone={timeZone} />;
  }
  return null;
}

const refusalKeys: Record<FeedbackRefusal, StringKey> = {
  notOpen: 'feedback.refused.notOpen',
  closed: 'feedback.refused.closed',
  failed: 'feedback.refused.failed',
};

/**
 * Feedback on the outing's strip (docs/spec/outings.md, "Feedback"): from the
 * plan's end for 7 days, the three questions the server asks so far. Answers
 * are reopened and changed until it closes. The others are named by first name
 * and a handle that means something only inside this outing, and the picks in
 * "who would you meet again?" are private: they live in this form's state until
 * saved, and are never put in an address, a log or a cache key.
 */
export function FeedbackSection({ outing, timeZone }: { outing: Outing; timeZone: string }) {
  const { t } = useI18n();
  const theme = useTheme();
  const { feedbacks, loadFeedback } = useMyOutings();
  const now = new Date();
  if (feedbackClosed(outing, now)) return <Text style={[styles.body, { color: theme.textSecondary }]}>{t('feedback.closed')}</Text>;
  if (!feedbackOpen(outing, now)) return null;
  const read = feedbacks[outing.id];
  if (!read || read.status === 'loading') return <Text style={[styles.body, { color: theme.textSecondary }]}>{t('feedback.loading')}</Text>;
  if (read.status === 'failed') {
    return (
      <View style={styles.tight}>
        <ErrorText>{t('feedback.loadFailed')}</ErrorText>
        <Button quiet label={t('plans.retry')} onPress={() => loadFeedback(outing.id)} />
      </View>
    );
  }
  return <FeedbackForm outing={outing} timeZone={timeZone} feedback={read.feedback} />;
}

/** The answers as the form holds them: handles only, which are the one thing that names another person inside the outing. */
type Draft = { went: boolean | null; notThere: Set<string>; meetAgain: Set<string> };
const draftOf = (f: Feedback): Draft => ({ went: f.answered ? f.went : null, notThere: new Set(f.notThere), meetAgain: new Set(f.meetAgain) });

function FeedbackForm({ outing, timeZone, feedback }: { outing: Outing; timeZone: string; feedback: Feedback }) {
  const outingId = outing.id;
  const { t } = useI18n();
  const theme = useTheme();
  const { saveFeedback } = useMyOutings();
  const [editing, setEditing] = useState(!feedback.answered);
  const [draft, setDraft] = useState<Draft>(() => draftOf(feedback));
  const [busy, setBusy] = useState(false);
  const [refusal, setRefusal] = useState<FeedbackRefusal | null>(null);

  const flip = (set: Set<string>, handle: string) => {
    const next = new Set(set);
    if (!next.delete(handle)) next.add(handle);
    return next;
  };
  const tickThere = (handle: string) =>
    setDraft((d) => {
      const notThere = flip(d.notThere, handle);
      // A person who was not there cannot be one the person would meet again.
      const meetAgain = new Set(d.meetAgain);
      if (notThere.has(handle)) meetAgain.delete(handle);
      return { ...d, notThere, meetAgain };
    });

  const save = async () => {
    if (draft.went === null) return;
    setBusy(true);
    setRefusal(null);
    try {
      // A "no" to "did you go?" ends it there: nothing after it is sent.
      const result = await saveFeedback(outingId, draft.went ? { went: true, notThere: [...draft.notThere].sort(), meetAgain: [...draft.meetAgain].sort() } : { went: false, notThere: [], meetAgain: [] });
      if (result) setRefusal(result);
      else setEditing(false);
    } finally {
      setBusy(false);
    }
  };

  if (!editing) {
    return (
      <View style={styles.tight}>
        <Text style={[styles.body, { color: theme.text }]}>{t(feedback.went ? 'feedback.saved.went' : 'feedback.saved.didNotGo')}</Text>
        <FeedbackCloses outing={outing} timeZone={timeZone} sentence="feedback.closes" size="small" />
        <Button
          quiet
          label={t('feedback.change')}
          onPress={() => {
            setDraft(draftOf(feedback));
            setRefusal(null);
            setEditing(true);
          }}
        />
      </View>
    );
  }

  const others = feedback.others;
  const there = others.filter((o) => !draft.notThere.has(o.handle));
  return (
    <View style={styles.form}>
      <Text accessibilityRole="header" style={[styles.heading, { color: theme.text }]}>{t('feedback.title')}</Text>
      <FeedbackCloses outing={outing} timeZone={timeZone} sentence="feedback.closes" size="small" />

      <Text style={[styles.question, { color: theme.text }]}>{t('feedback.q1')}</Text>
      <View style={styles.row}>
        <View style={styles.half}>
          <Choice kind="radio" label={t('outing.yes')} selected={draft.went === true} onPress={() => setDraft((d) => ({ ...d, went: true }))} />
        </View>
        <View style={styles.half}>
          <Choice kind="radio" label={t('outing.no')} selected={draft.went === false} onPress={() => setDraft((d) => ({ ...d, went: false }))} />
        </View>
      </View>

      {draft.went ? (
        others.length === 0 ? (
          <Text style={[styles.body, { color: theme.textSecondary }]}>{t('feedback.noOthers')}</Text>
        ) : (
          <>
            <Text style={[styles.question, { color: theme.text }]}>{t('feedback.q2')}</Text>
            <Text style={[styles.small, { color: theme.textSecondary }]}>{t('feedback.q2.help')}</Text>
            <View testID="feedback-there" style={styles.tight}>
              {others.map((o) => (
                <Choice key={o.handle} kind="checkbox" label={o.firstName} selected={!draft.notThere.has(o.handle)} onPress={() => tickThere(o.handle)} />
              ))}
            </View>

            <Text style={[styles.question, { color: theme.text }]}>{t('feedback.q3')}</Text>
            <Text style={[styles.small, { color: theme.textSecondary }]}>{t('feedback.q3.help')}</Text>
            <View testID="feedback-again" style={styles.tight}>
              {there.map((o) => (
                <Choice key={o.handle} kind="checkbox" label={o.firstName} selected={draft.meetAgain.has(o.handle)} onPress={() => setDraft((d) => ({ ...d, meetAgain: flip(d.meetAgain, o.handle) }))} />
              ))}
            </View>
          </>
        )
      ) : null}

      {refusal ? <ErrorText>{t(refusalKeys[refusal])}</ErrorText> : null}
      <Button label={t('feedback.save')} onPress={save} disabled={busy || draft.went === null} />
    </View>
  );
}

const styles = StyleSheet.create({
  tight: { gap: Spacing.two },
  form: { gap: Spacing.two },
  row: { flexDirection: 'row', gap: Spacing.two },
  half: { flex: 1 },
  heading: { fontSize: FontSize.heading, fontWeight: '700' },
  question: { fontSize: FontSize.default, fontWeight: '700' },
  body: { fontSize: FontSize.default },
  small: { fontSize: FontSize.small },
});
