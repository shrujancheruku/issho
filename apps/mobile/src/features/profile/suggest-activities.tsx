import { Code, ConnectError, type Activity, type Interest } from '@issho/api-client';
import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { Pressable, StyleSheet, TextInput, View } from 'react-native';

import { Chip } from '@/components/chip';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { Brand, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { api } from '@/lib/api';

import { addSuggestions, cycleSuggestion, type Draft } from './draft';
import { interestStyles } from './interest';

type Suggestion = { activityId: string; interest: Interest };

/**
 * Free-text interests → suggested tiles (CatalogService.SuggestActivities,
 * Claude on the server). The person reviews the suggestions, can change or
 * drop each one, and only "Add these" puts them in the draft.
 */
export function SuggestActivities({
  draft,
  onChange,
  activities,
}: {
  draft: Draft;
  onChange: (d: Draft) => void;
  activities: Activity[];
}) {
  const { t } = useI18n();
  const theme = useTheme();
  const [text, setText] = useState('');
  const [suggestions, setSuggestions] = useState<Suggestion[] | null>(null);
  const [added, setAdded] = useState<number | null>(null);
  const byId = new Map(activities.map((a) => [a.id, a]));

  const suggest = useMutation({
    mutationFn: () => api.catalog.suggestActivities({ cityId: draft.cityId, text }),
    onSuccess: (res) => {
      setAdded(null);
      // Only activities this screen can show (the list is per city).
      setSuggestions(res.suggestions.filter((s) => byId.has(s.activityId)));
    },
  });
  const unavailable = suggest.error instanceof ConnectError && suggest.error.code === Code.FailedPrecondition;

  const add = () => {
    const result = addSuggestions(draft, suggestions ?? []);
    onChange(result.draft);
    setAdded(result.added);
    setSuggestions(null);
    setText('');
  };

  return (
    <View style={styles.box}>
      <ThemedText type="smallBold">{t('suggest.title')}</ThemedText>
      <TextInput
        style={[styles.input, { color: theme.text, backgroundColor: theme.backgroundElement }]}
        value={text}
        onChangeText={setText}
        placeholder={t('suggest.placeholder')}
        placeholderTextColor={theme.textSecondary}
        multiline
        maxLength={500}
      />
      <PrimaryButton
        label={t('suggest.button')}
        onPress={() => suggest.mutate()}
        disabled={!text.trim() || !draft.cityId}
        busy={suggest.isPending}
      />
      {suggest.isError && (
        <ThemedText type="small" style={styles.error}>
          {unavailable ? t('suggest.unavailable') : t('error.network')}
        </ThemedText>
      )}
      {added !== null && (
        <ThemedText type="small" themeColor="textSecondary">
          {t('suggest.added', { count: added })}
        </ThemedText>
      )}
      {suggestions?.length === 0 && (
        <ThemedText type="small" themeColor="textSecondary">
          {t('suggest.none')}
        </ThemedText>
      )}
      {!!suggestions?.length && (
        <View style={styles.results}>
          <ThemedText type="small" themeColor="textSecondary">
            {t('suggest.results')}
          </ThemedText>
          <View style={styles.wrap}>
            {suggestions.map((s) => {
              const a = byId.get(s.activityId)!;
              const st = interestStyles[s.interest as keyof typeof interestStyles];
              return (
                <Chip
                  key={s.activityId}
                  label={a.name}
                  secondary={a.secondaryName}
                  selected
                  selectedColors={st}
                  badge={t(st.label)}
                  onPress={() => setSuggestions(cycleSuggestion(suggestions, s.activityId))}
                />
              );
            })}
          </View>
          <View style={styles.actions}>
            <Pressable accessibilityRole="button" onPress={() => setSuggestions(null)} hitSlop={8}>
              <ThemedText type="link">{t('suggest.dismiss')}</ThemedText>
            </Pressable>
            <PrimaryButton label={t('suggest.add')} onPress={add} />
          </View>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  box: { gap: Spacing.two },
  input: { borderRadius: 12, padding: Spacing.three, fontSize: 16, minHeight: 88, textAlignVertical: 'top' },
  error: { color: Brand.red },
  results: { gap: Spacing.two },
  wrap: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.two },
  actions: { flexDirection: 'row', alignItems: 'center', gap: Spacing.three },
});
