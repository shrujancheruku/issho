import { Code, ConnectError, ReportReason } from '@issho/api-client';
import { useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, Pressable } from 'react-native';

import { ChipChoice, DetailsInput, FormError, FormScreen } from '@/components/form-screen';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { useCrew, useReportCrewMember } from '@/features/crews/data';
import { useFeedback, useReport } from '@/features/events/data';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';

const reasons: [ReportReason, StringKey][] = [
  [ReportReason.DATING, 'report.reasonDating'],
  [ReportReason.HARASSMENT, 'report.reasonHarassment'],
  [ReportReason.UNSAFE, 'report.reasonUnsafe'],
  [ReportReason.NO_SHOW, 'report.reasonNoShow'],
  [ReportReason.OTHER, 'report.reasonOther'],
];

/**
 * Reports someone from the person's outing group (id = the event) or crew
 * (id = the crew, `crew=1`) to the moderators. For an outing: anyone who
 * confirmed (even if they cancelled since), until 30 days after the start.
 * The two are never grouped together again.
 */
export default function ReportScreen() {
  const { id = '', crew: inCrew } = useLocalSearchParams<{ id: string; crew?: string }>();
  const isCrew = inCrew === '1';
  const { t } = useI18n();
  // Only one of each pair is enabled: the other has no id.
  const feedbackForm = useFeedback(isCrew ? '' : id);
  const crewQuery = useCrew(isCrew ? id : '');
  const form = isCrew ? crewQuery : feedbackForm;
  const outingReport = useReport(id);
  const crewReport = useReportCrewMember(id);
  const report = isCrew ? crewReport : outingReport;
  const [mate, setMate] = useState('');
  const [reason, setReason] = useState<ReportReason>(ReportReason.UNSPECIFIED);
  const [details, setDetails] = useState('');

  const mates = isCrew
    ? (crewQuery.data?.members ?? []).filter((m) => !m.me && m.accepted)
    : (feedbackForm.data?.mates ?? []);
  let body;
  if (report.isSuccess) {
    body = <ThemedText>{t('report.sent')}</ThemedText>;
  } else if (form.isError) {
    body = (
      <Pressable onPress={() => void form.refetch()}>
        <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
        <ThemedText type="link">{t('common.retry')}</ThemedText>
      </Pressable>
    );
  } else if (!form.data) {
    body = <ActivityIndicator />;
  } else if (!mates.length) {
    body = <ThemedText themeColor="textSecondary">{t('report.nobody')}</ThemedText>;
  } else {
    body = (
      <>
        <ChipChoice label={t('report.who')} options={mates.map((m) => [m.id, m.name])} value={mate} onChange={setMate} />
        <ChipChoice
          label={t('report.reason')}
          options={reasons.map(([r, key]) => [r, t(key)])}
          value={reason}
          onChange={setReason}
        />
        <DetailsInput label={t('report.details')} value={details} onChange={setDetails} />
      </>
    );
  }

  return (
    <FormScreen
      title={t('report.title')}
      hint={t('report.hint')}
      intro={<ThemedText type="smallBold">{t('report.emergency')}</ThemedText>}
      footer={
        !report.isSuccess &&
        mates.length > 0 && (
          <>
            {report.isError && (
              <FormError>
                {report.error instanceof ConnectError && report.error.code === Code.FailedPrecondition
                  ? t('event.tooLate')
                  : t('error.network')}
              </FormError>
            )}
            <PrimaryButton
              label={t('report.send')}
              disabled={!mate || reason === ReportReason.UNSPECIFIED}
              busy={report.isPending}
              onPress={() => report.mutate({ mateId: mate, reason, details })}
            />
          </>
        )
      }>
      {body}
    </FormScreen>
  );
}
