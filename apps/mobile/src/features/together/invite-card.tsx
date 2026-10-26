import type { MyEvent } from '@issho/api-client';
import { router } from 'expo-router';
import { StyleSheet, View } from 'react-native';

import { ResultButton } from '@/components/result-button';
import { TextLink } from '@/components/text-link';
import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useRespond } from '@/features/events/data';
import { formatStart, lastSignupDay } from '@/features/plans/format';
import type { usePlanNames } from '@/features/plans/names';
import { pairText } from '@/features/plans/plan-card';
import { useTheme } from '@/hooks/use-theme';
import { useI18n, useOtherLanguage } from '@/i18n/i18n-provider';
import { haptics } from '@/lib/haptics';

/**
 * An invitation to answer, right on Together: what, when, how many have
 * confirmed, and the answer itself. "I can go" becomes "You are going"
 * where it was tapped; the outing then moves to Your groups.
 */
export function InviteCard({ event, names }: { event: MyEvent; names: ReturnType<typeof usePlanNames> }) {
  const { t, languages, showing } = useI18n();
  const otherLanguage = useOtherLanguage();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const theme = useTheme();
  // Refresh after the button has shown "You are going"; the outing then moves to Your groups.
  const respond = useRespond(event.id, { refreshAfter: 1400 });
  const going = respond.isSuccess && respond.variables === 'confirm';
  const [title, secondary] =
    showing === 'secondary' && event.secondaryTitle ? [event.secondaryTitle, event.title] : [event.title, event.secondaryTitle];
  const open = () => router.push({ pathname: '/event/[id]', params: { id: event.id } });
  return (
    <View style={[styles.card, { backgroundColor: theme.surface }]}>
      <ThemedText type="smallBold" themeColor="goldText">
        {t('event.state.answer', { date: lastSignupDay(event.confirmBy, lang) })}
      </ThemedText>
      <ThemedText type="subtitle" style={styles.title}>
        {title}
      </ThemedText>
      {!!secondary && (
        <ThemedText type="small" themeColor="textSecondary" lang={otherLanguage}>
          {secondary}
        </ThemedText>
      )}
      <ThemedText style={styles.when}>
        {formatStart(event.startsAt, lang)}, {pairText(names.area(event.areaId))}
      </ThemedText>
      <ThemedText type="small" themeColor="textSecondary" style={styles.count}>
        {t('event.confirmedCount', { count: event.confirmed })}
      </ThemedText>
      {respond.isError && (
        <ThemedText type="small" themeColor="error">
          {t('error.network')}
        </ThemedText>
      )}
      <ResultButton
        label={t('event.confirm')}
        doneLabel={t('event.state.going')}
        done={going}
        busy={respond.isPending && respond.variables === 'confirm'}
        onPress={() => respond.mutate('confirm', { onSuccess: haptics.success })}
      />
      <View style={styles.links}>
        {/* Once they've said yes, the card is about to move to their groups. */}
        {going ? <View /> : <TextLink label={t('event.decline')} disabled={respond.isPending} onPress={() => respond.mutate('decline')} />}
        <TextLink label={t('crew.seePlan')} onPress={open} />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  card: { borderRadius: 16, padding: Spacing.three, gap: Spacing.one },
  title: { fontSize: 22, lineHeight: 32, marginTop: Spacing.one },
  when: { marginTop: Spacing.two },
  count: { marginBottom: Spacing.two },
  links: { flexDirection: 'row', justifyContent: 'space-between' },
});
