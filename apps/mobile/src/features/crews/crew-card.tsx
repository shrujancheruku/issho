import { CrewStatus, type Crew } from '@issho/api-client';
import { router } from 'expo-router';
import { Pressable, StyleSheet } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Brand, Spacing } from '@/constants/theme';
import { formatStart } from '@/features/plans/format';
import type { usePlanNames } from '@/features/plans/names';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

import { crewName } from './name';

/** One of the person's crews, or an offer (gold: needs an answer). */
export function CrewCard({ crew, names }: { crew: Crew; names: ReturnType<typeof usePlanNames> }) {
  const { t, languages, showing } = useI18n();
  const lang = showing === 'secondary' ? languages.secondary : languages.primary;
  const theme = useTheme();
  const offer = crew.status === CrewStatus.OFFERED;
  const name = crewName(crew, names, (n) => t('crew.name', { name: n }));
  return (
    <Pressable
      accessibilityRole="button"
      onPress={() => router.push({ pathname: '/crew/[id]', params: { id: crew.id } })}
      style={({ pressed }) => [
        styles.card,
        { backgroundColor: theme.backgroundElement, borderColor: offer ? Brand.gold : 'transparent', opacity: pressed ? 0.8 : 1 },
      ]}>
      {offer && (
        <ThemedText type="smallBold" style={{ color: Brand.gold }}>
          {t('crew.offerCard', { date: formatStart(crew.offerUntil, lang) })}
        </ThemedText>
      )}
      <ThemedText type="subtitle" style={styles.title}>
        {name.name}
      </ThemedText>
      <ThemedText type="small" themeColor="textSecondary">
        {crew.members.map((m) => m.name).join(', ')}
      </ThemedText>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  card: { borderRadius: 16, padding: Spacing.three, gap: Spacing.one, borderWidth: 2 },
  title: { fontSize: 20, lineHeight: 28 },
});
