import { SeatState, type Outing } from '@issho/api-client';
import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';

import { Button, ErrorText } from '@/components/form';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { deviceTimeZone, formatInstantDay, formatInstantWhen, formatOutingWhen } from '@/lib/format';
import { canSayYes, stampDate, type OutingRefusal } from '@/outings/model';
import { languagesText } from '@/outings/text';
import { useMyOutings } from '@/outings/use-my-outings';

const refusalKeys: Record<OutingRefusal, StringKey> = {
  timeTaken: 'outing.refused.timeTaken',
  ranOut: 'outing.refused.ranOut',
  full: 'outing.refused.full',
  failed: 'outing.refused.failed',
  checkedIn: 'outing.cancelRefused.checkedIn',
  cannotCancel: 'outing.cancelRefused.cannot',
  cancelFailed: 'outing.cancelRefused.failed',
  checkInEarly: 'outing.checkInRefused.early',
  checkInOver: 'outing.checkInRefused.over',
  checkInFailed: 'outing.checkInRefused.failed',
};

/** Why the person's last try on this outing was refused, in words; nothing when it was not. */
export function RefusalText({ outingId }: { outingId: string }) {
  const { t } = useI18n();
  const { refusals } = useMyOutings();
  const refusal = refusals[outingId];
  return refusal ? <ErrorText>{t(refusalKeys[refusal])}</ErrorText> : null;
}

/** The date and time on the city's clock, with the device's beside it when it reads differently (docs/spec/outings.md). */
export function OutingWhen({ outing, timeZone }: { outing: Outing; timeZone: string }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const when = formatOutingWhen({ date: outing.date, startsAt: outing.startsAt, timeZone, lang, deviceZone: deviceTimeZone() });
  return (
    <View style={styles.tight}>
      <Text style={[styles.value, { color: theme.text }]}>{when.city}</Text>
      {when.device ? <Text style={[styles.small, { color: theme.textSecondary }]}>{t('outing.deviceTime', { when: when.device })}</Text> : null}
    </View>
  );
}

/** The device's own reading of a time, beside the city's, when it reads differently. */
function DeviceLine({ device }: { device: string | null }) {
  const { t } = useI18n();
  const theme = useTheme();
  return device ? <Text style={[styles.small, { color: theme.textSecondary }]}>{t('outing.deviceTime', { when: device })}</Text> : null;
}

/** When check-in opens, on the city's clock with the device's beside it (docs/spec/city-settings.md). */
export function CheckInOpens({ outing, timeZone }: { outing: Outing; timeZone: string }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const when = formatInstantWhen(stampDate(outing.checkInOpensAt), { timeZone, lang, deviceZone: deviceTimeZone() });
  return (
    <View style={styles.tight}>
      <Text style={[styles.body, { color: theme.textSecondary }]}>{t('outing.checkIn.opens', { when: when.city })}</Text>
      <DeviceLine device={when.device} />
    </View>
  );
}

/** The day feedback closes, on the city's clock with the device's beside it when that is another day. `sentence` is the string it is written in. */
export function FeedbackCloses({ outing, timeZone, sentence, size }: { outing: Outing; timeZone: string; sentence: 'feedback.closes' | 'feedback.card'; size: 'body' | 'small' }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const when = formatInstantDay(stampDate(outing.feedbackClosesAt), { timeZone, lang, deviceZone: deviceTimeZone() });
  const color = sentence === 'feedback.card' ? theme.text : theme.textSecondary;
  return (
    <View style={styles.tight}>
      <Text style={[styles[size], { color }]}>{t(sentence, { when: when.city })}</Text>
      <DeviceLine device={when.device} />
    </View>
  );
}

/** The count of yeses and the languages the group shares: all an invited person sees of the group. */
export function OutingGroupLines({ outing }: { outing: Outing }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  return (
    <View style={styles.tight}>
      <Text style={[styles.body, { color: theme.text }]}>{outing.yesCount === 1 ? t('outing.yesOne') : t('outing.yesMany', { count: outing.yesCount })}</Text>
      <Text style={[styles.body, { color: theme.text }]}>{t('outing.languages', { languages: languagesText(outing, lang) })}</Text>
    </View>
  );
}

/** Yes and no on an invitation, answered where the person is; a refusal is said in words under them. */
export function AnswerButtons({ outing }: { outing: Outing }) {
  const { t } = useI18n();
  const { answer } = useMyOutings();
  const [busy, setBusy] = useState(false);
  const now = new Date();
  const yes = canSayYes(outing, now);
  // A no is for an invitation not yet answered; a seat that said no can only change to yes.
  const no = outing.seatState === SeatState.INVITED;
  const send = async (value: boolean) => {
    setBusy(true);
    await answer(outing.id, value);
    setBusy(false);
  };
  if (!yes) return null;
  return (
    <View style={styles.tight}>
      <View style={styles.buttons}>
        <View style={styles.button}>
          <Button label={t('outing.yes')} onPress={() => send(true)} disabled={busy} />
        </View>
        {no ? (
          <View style={styles.button}>
            <Button quiet label={t('outing.no')} onPress={() => send(false)} disabled={busy} />
          </View>
        ) : null}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  tight: { gap: Spacing.half },
  value: { fontSize: FontSize.default, fontWeight: '700' },
  body: { fontSize: FontSize.default },
  small: { fontSize: FontSize.small },
  buttons: { flexDirection: 'row', gap: Spacing.two },
  button: { flex: 1 },
});
