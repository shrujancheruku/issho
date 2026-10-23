import { Code, ConnectError, ReportReason } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { ActivityIndicator, KeyboardAvoidingView, Pressable, ScrollView, StyleSheet, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Chip } from '@/components/chip';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Brand, FormWidth, Spacing } from '@/constants/theme';
import { useFeedback, useReport } from '@/features/events/data';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';

const MaxDetails = 1000; // as the server (plans.MaxReportDetails)

const reasons: [ReportReason, StringKey][] = [
  [ReportReason.DATING, 'report.reasonDating'],
  [ReportReason.HARASSMENT, 'report.reasonHarassment'],
  [ReportReason.UNSAFE, 'report.reasonUnsafe'],
  [ReportReason.NO_SHOW, 'report.reasonNoShow'],
  [ReportReason.OTHER, 'report.reasonOther'],
];

/**
 * Reports someone from the person's group to the moderators. Open to anyone
 * who confirmed (even if they cancelled since) until 30 days after the
 * start; the two are never grouped together again.
 */
export default function ReportScreen() {
  const { id = '' } = useLocalSearchParams<{ id: string }>();
  const { t } = useI18n();
  const theme = useTheme();
  const form = useFeedback(id);
  const report = useReport(id);
  const [mate, setMate] = useState('');
  const [reason, setReason] = useState<ReportReason>(ReportReason.UNSPECIFIED);
  const [details, setDetails] = useState('');
  const leave = () => (router.canGoBack() ? router.back() : router.replace('/'));

  const mates = form.data?.mates ?? [];
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
        <View style={styles.section}>
          <ThemedText type="smallBold">{t('report.who')}</ThemedText>
          <View style={styles.wrap}>
            {mates.map((m) => (
              <Chip key={m.id} label={m.name} selected={mate === m.id} onPress={() => setMate(m.id)} />
            ))}
          </View>
        </View>
        <View style={styles.section}>
          <ThemedText type="smallBold">{t('report.reason')}</ThemedText>
          <View style={styles.wrap}>
            {reasons.map(([r, key]) => (
              <Chip key={r} label={t(key)} selected={reason === r} onPress={() => setReason(r)} />
            ))}
          </View>
        </View>
        <View style={styles.section}>
          <ThemedText type="smallBold">{t('report.details')}</ThemedText>
          <TextInput
            value={details}
            onChangeText={setDetails}
            maxLength={MaxDetails}
            multiline
            style={[styles.input, { color: theme.text, backgroundColor: theme.backgroundElement }]}
          />
        </View>
      </>
    );
  }

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <KeyboardAvoidingView behavior="padding" style={[styles.fill, styles.column]}>
          <View style={styles.header}>
            <Pressable accessibilityRole="button" onPress={leave} hitSlop={8}>
              <ThemedText type="link">‹ {t('common.back')}</ThemedText>
            </Pressable>
          </View>
          <ScrollView contentContainerStyle={styles.content}>
            <View style={styles.titleBlock}>
              <ThemedText type="subtitle">{t('report.title')}</ThemedText>
              <ThemedText type="small" themeColor="textSecondary">
                {t('report.hint')}
              </ThemedText>
              <ThemedText type="smallBold">{t('report.emergency')}</ThemedText>
            </View>
            {body}
          </ScrollView>
          {!report.isSuccess && mates.length > 0 && (
            <View style={styles.footer}>
              {report.isError && (
                <ThemedText style={styles.error}>
                  {report.error instanceof ConnectError && report.error.code === Code.FailedPrecondition
                    ? t('event.tooLate')
                    : t('error.network')}
                </ThemedText>
              )}
              <PrimaryButton
                label={t('report.send')}
                disabled={!mate || reason === ReportReason.UNSPECIFIED}
                busy={report.isPending}
                onPress={() => report.mutate({ mateId: mate, reason, details })}
              />
            </View>
          )}
        </KeyboardAvoidingView>
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  column: { width: '100%', maxWidth: FormWidth, alignSelf: 'center' },
  header: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: Spacing.four, paddingTop: Spacing.three },
  content: { padding: Spacing.four, gap: Spacing.four },
  titleBlock: { gap: Spacing.two },
  section: { gap: Spacing.two },
  wrap: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.two },
  input: { minHeight: 96, borderRadius: 12, padding: Spacing.three, textAlignVertical: 'top' },
  footer: { padding: Spacing.four, gap: Spacing.two },
  error: { color: Brand.red, textAlign: 'center' },
});
