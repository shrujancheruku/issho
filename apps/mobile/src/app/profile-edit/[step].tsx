import { useMutation, useQueryClient } from '@tanstack/react-query';
import { router, useLocalSearchParams } from 'expo-router';
import { useEffect, useMemo, useState } from 'react';
import { ActivityIndicator, KeyboardAvoidingView, Pressable, ScrollView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { LanguageSwitch } from '@/components/language-switch';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Brand, FormWidth, Spacing } from '@/constants/theme';
import { fromProfile, steps, stepValid, toProfile, type Draft, type Step } from '@/features/profile/draft';
import { saveErrorMessage } from '@/features/profile/save-error';
import { StepView, stepTitles } from '@/features/profile/steps';
import { profileKey, useProfile } from '@/features/profile/use-profile';
import { useI18n } from '@/i18n/i18n-provider';
import { api } from '@/lib/api';

/** Edits one section of the profile and saves the whole profile. */
export default function ProfileEditScreen() {
  const { step: param } = useLocalSearchParams<{ step: string }>();
  const step = (steps as readonly string[]).includes(param) && param !== 'conduct' ? (param as Step) : 'about';
  const { t, previewLanguages } = useI18n();
  const queryClient = useQueryClient();
  const profile = useProfile();
  // The saved profile until the first edit.
  const [edited, setDraft] = useState<Draft | null>(null);
  const saved = useMemo(
    () => (profile.data?.profile ? fromProfile(profile.data.profile, profile.data.currentConductVersion) : null),
    [profile.data],
  );
  const draft = edited ?? saved;

  // While editing languages, show the screen in the ones being chosen.
  useEffect(() => {
    if (step === 'languages' && draft) previewLanguages({ primary: draft.primaryLanguage, secondary: draft.secondaryLanguage });
  }, [step, draft, previewLanguages]);
  useEffect(() => () => previewLanguages(null), [previewLanguages]);

  const save = useMutation({
    // Settings never accepts a code of conduct on the person's behalf: it
    // sends the version they accepted, and the server refuses an outdated one.
    mutationFn: (d: Draft) =>
      api.profile.saveProfile({ profile: toProfile(d, profile.data?.profile?.conductVersion ?? 0) }),
    onSuccess: async (res) => {
      queryClient.setQueryData(profileKey, (old: typeof profile.data) => (old ? { ...old, profile: res.profile } : old));
      // GetMe carries the display languages.
      await queryClient.invalidateQueries({ queryKey: ['me'] });
      router.back();
    },
  });

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <KeyboardAvoidingView behavior="padding" style={[styles.fill, styles.column]}>
          <View style={styles.header}>
            <Pressable accessibilityRole="button" onPress={() => router.back()} hitSlop={8} style={styles.grow}>
              <ThemedText type="link">‹ {t('common.back')}</ThemedText>
            </Pressable>
            <LanguageSwitch />
          </View>
          <ScrollView contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
            <ThemedText type="subtitle">{t(stepTitles[step])}</ThemedText>
            {draft ? (
              <StepView step={step} draft={draft} onChange={setDraft} />
            ) : profile.isError ? (
              <Pressable onPress={() => void profile.refetch()}>
                <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
                <ThemedText type="link">{t('common.retry')}</ThemedText>
              </Pressable>
            ) : (
              <ActivityIndicator />
            )}
          </ScrollView>
          {save.isError && <ThemedText style={styles.error}>{saveErrorMessage(save.error, t)}</ThemedText>}
          <View style={styles.footer}>
            <PrimaryButton
              label={t('common.save')}
              disabled={!draft || !stepValid(step, draft)}
              busy={save.isPending}
              onPress={() => draft && save.mutate(draft)}
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
  grow: { flex: 1 },
  header: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: Spacing.four, paddingTop: Spacing.three },
  content: { padding: Spacing.four, gap: Spacing.four },
  footer: { padding: Spacing.four },
  error: { color: Brand.red, textAlign: 'center', paddingHorizontal: Spacing.four },
});
