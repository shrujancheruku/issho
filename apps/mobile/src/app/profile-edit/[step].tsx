import { useLocalSearchParams } from 'expo-router';
import { useEffect, useMemo, useState } from 'react';
import { ScrollView, StyleSheet, View } from 'react-native';

import { PrimaryButton } from '@/components/primary-button';
import { LoadState } from '@/components/query-screen';
import { ScreenFrame, useLeave } from '@/components/screen-frame';
import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { canSave, fromProfile, nextEditStep, steps, stepValid, toProfile, type Draft, type Step } from '@/features/profile/draft';
import { saveErrorMessage } from '@/features/profile/save-error';
import { StepView, stepTitles } from '@/features/profile/steps';
import { useProfile, useSaveProfile } from '@/features/profile/data';
import { useI18n } from '@/i18n/i18n-provider';

/** Edits one section of the profile and saves the whole profile, once every section is complete. */
export default function ProfileEditScreen() {
  const { step: param } = useLocalSearchParams<{ step: string }>();
  const first = (steps as readonly string[]).includes(param) && param !== 'conduct' ? (param as Step) : 'about';
  // Starts at the step opened from settings. An edit can leave another step
  // incomplete (a new city clears the activities), so the screen moves on to
  // it in place: the draft lives in this screen's state and a route change
  // would lose it.
  const [moved, setStep] = useState<Step | null>(null);
  const step = moved ?? first;
  const { t, previewLanguages } = useI18n();
  const profile = useProfile();
  // Opened from a link or after a reload on the web, there's no settings screen behind this one.
  const leave = useLeave('/profile');
  // The saved profile until the first edit.
  const [edited, setDraft] = useState<Draft | null>(null);
  const saved = useMemo(
    () => (profile.data?.profile ? fromProfile(profile.data.profile, profile.data.currentConductVersion) : null),
    [profile.data],
  );
  const draft = edited ?? saved;
  const next = draft ? nextEditStep(step, draft) : null;

  // While editing languages, show the screen in the ones being chosen.
  useEffect(() => {
    if (step === 'languages' && draft)
      previewLanguages({
        primary: draft.primaryLanguage,
        secondary: draft.secondaryLanguage,
      });
  }, [step, draft, previewLanguages]);
  useEffect(() => () => previewLanguages(null), [previewLanguages]);

  const save = useSaveProfile();

  return (
    <ScreenFrame onBack={leave}>
      <ScrollView contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
        <ThemedText type="subtitle">{t(stepTitles[step])}</ThemedText>
        {draft ? (
          <StepView step={step} draft={draft} onChange={setDraft} />
        ) : (
          // A loaded profile that is missing (not onboarded) also waits here, as the route guards redirect.
          <View>
            <LoadState queries={[profile]} />
          </View>
        )}
      </ScrollView>
      {save.isError && (
        <ThemedText themeColor="error" style={styles.error}>
          {saveErrorMessage(save.error, t)}
        </ThemedText>
      )}
      <View style={styles.footer}>
        <PrimaryButton
          label={next ? t('common.continue') : t('common.save')}
          disabled={!draft || !stepValid(step, draft)}
          busy={save.isPending}
          onPress={() => {
            if (!draft) return;
            if (next) setStep(next);
            else if (canSave(draft))
              save.mutate(
                // Settings never accepts a code of conduct on the person's behalf: it
                // sends the version they accepted, and the server refuses an outdated one.
                toProfile(draft, profile.data?.profile?.conductVersion ?? 0),
                { onSuccess: leave },
              );
          }}
        />
      </View>
    </ScreenFrame>
  );
}

const styles = StyleSheet.create({
  content: { padding: Spacing.four, gap: Spacing.four },
  footer: { padding: Spacing.four },
  error: { textAlign: 'center', paddingHorizontal: Spacing.four },
});
