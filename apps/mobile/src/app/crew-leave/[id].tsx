import { LeaveReason } from '@issho/api-client';
import { router, useLocalSearchParams } from 'expo-router';
import { useState } from 'react';
import { KeyboardAvoidingView, Pressable, ScrollView, StyleSheet, TextInput, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Chip } from '@/components/chip';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BodyFont, FormWidth, Spacing } from '@/constants/theme';
import { useLeaveCrew } from '@/features/crews/data';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';

const MaxDetails = 1000; // as the server (plans.MaxReportDetails)

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
  const theme = useTheme();
  const leaveCrew = useLeaveCrew(id);
  const [reason, setReason] = useState<LeaveReason>(LeaveReason.UNSPECIFIED);
  const [details, setDetails] = useState('');
  const back = () => (router.canGoBack() ? router.back() : router.replace('/'));

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <KeyboardAvoidingView behavior="padding" style={[styles.fill, styles.column]}>
          <View style={styles.header}>
            <Pressable accessibilityRole="button" onPress={back} hitSlop={8}>
              <ThemedText type="link">‹ {t('common.back')}</ThemedText>
            </Pressable>
          </View>
          <ScrollView contentContainerStyle={styles.content}>
            <View style={styles.titleBlock}>
              <ThemedText type="subtitle">{t('leave.title')}</ThemedText>
              <ThemedText type="small" themeColor="textSecondary">
                {t('leave.hint')}
              </ThemedText>
            </View>
            <View style={styles.section}>
              <ThemedText type="smallBold">{t('leave.reason')}</ThemedText>
              <View style={styles.wrap}>
                {reasons.map(([r, key]) => (
                  <Chip key={r} label={t(key)} selected={reason === r} onPress={() => setReason(r)} />
                ))}
              </View>
              {reason === LeaveReason.UNCOMFORTABLE && (
                <Pressable
                  accessibilityRole="button"
                  onPress={() => router.push({ pathname: '/report/[id]', params: { id, crew: '1' } })}>
                  <ThemedText type="small">{t('leave.uncomfortableHint')}</ThemedText>
                  <ThemedText type="link">{t('event.report')}</ThemedText>
                </Pressable>
              )}
            </View>
            <View style={styles.section}>
              <ThemedText type="smallBold">{t('leave.details')}</ThemedText>
              <TextInput
                value={details}
                onChangeText={setDetails}
                maxLength={MaxDetails}
                multiline
                style={[styles.input, { color: theme.text, backgroundColor: theme.backgroundElement }]}
              />
            </View>
          </ScrollView>
          <View style={styles.footer}>
            {leaveCrew.isError && <ThemedText themeColor="error" style={styles.error}>{t('error.network')}</ThemedText>}
            <PrimaryButton
              label={t('leave.confirm')}
              disabled={reason === LeaveReason.UNSPECIFIED}
              busy={leaveCrew.isPending}
              // Back to the feed: the crew is no longer theirs to open.
              onPress={() => leaveCrew.mutate({ reason, details }, { onSuccess: () => router.replace('/') })}
            />
          </View>
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
  input: { fontFamily: BodyFont.regular, minHeight: 96, borderRadius: 12, padding: Spacing.three, textAlignVertical: 'top' },
  footer: { padding: Spacing.four, gap: Spacing.two },
  error: { textAlign: 'center' },
});
