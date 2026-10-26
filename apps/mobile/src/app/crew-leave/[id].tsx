import { LeaveReason } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { Pressable } from 'react-native';

import { ChipChoice, DetailsInput, FormError, FormScreen } from '@/components/form-screen';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { useLeaveCrew } from '@/features/crews/data';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';

const reasons: [LeaveReason, StringKey][] = [
  [LeaveReason.UNCOMFORTABLE, 'leave.reasonUncomfortable'],
  [LeaveReason.NOT_INTERESTED, 'leave.reasonNotInterested'],
  [LeaveReason.INACTIVE, 'leave.reasonInactive'],
  [LeaveReason.TOO_MANY_MESSAGES, 'leave.reasonTooManyMessages'],
  [LeaveReason.OTHER, 'leave.reasonOther'],
];

/**
 * Leaving a crew, with why. Always possible; the crew isn't told why or
 * that they left. Someone who felt uncomfortable is pointed to reporting.
 */
export default function LeaveCrewScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t } = useI18n();
  const leaveCrew = useLeaveCrew(id);
  const [reason, setReason] = useState<LeaveReason>(LeaveReason.UNSPECIFIED);
  const [details, setDetails] = useState('');

  return (
    <FormScreen
      title={t('leave.title')}
      hint={t('leave.hint')}
      footer={
        <>
          {leaveCrew.isError && <FormError>{t('error.network')}</FormError>}
          <PrimaryButton
            label={t('leave.confirm')}
            disabled={reason === LeaveReason.UNSPECIFIED}
            busy={leaveCrew.isPending}
            // Back to the feed: the crew is no longer theirs to open.
            onPress={() => leaveCrew.mutate({ reason, details }, { onSuccess: () => router.replace('/') })}
          />
        </>
      }>
      <ChipChoice
        label={t('leave.reason')}
        options={reasons.map(([r, key]) => [r, t(key)])}
        value={reason}
        onChange={setReason}>
        {reason === LeaveReason.UNCOMFORTABLE && (
          <Pressable
            accessibilityRole="button"
            onPress={() => router.push({ pathname: '/report/[id]', params: { id, crew: '1' } })}>
            <ThemedText type="small">{t('leave.uncomfortableHint')}</ThemedText>
            <ThemedText type="link">{t('event.report')}</ThemedText>
          </Pressable>
        )}
      </ChipChoice>
      <DetailsInput label={t('leave.details')} value={details} onChange={setDetails} />
    </FormScreen>
  );
}
