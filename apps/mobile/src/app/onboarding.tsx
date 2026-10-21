import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useEffect, useRef, useState } from 'react';
import { KeyboardAvoidingView, Pressable, ScrollView, StyleSheet, View, type ScrollViewInstance } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { LanguageSwitch } from '@/components/language-switch';
import { PrimaryButton } from '@/components/primary-button';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { Brand, MaxContentWidth, Spacing } from '@/constants/theme';
import { useAuth } from '@/features/auth/auth-provider';
import { ConductVersion, emptyDraft, steps, stepValid, toProfile } from '@/features/profile/draft';
import { saveErrorMessage } from '@/features/profile/save-error';
import { StepView, stepTitles } from '@/features/profile/steps';
import { useI18n } from '@/i18n/i18n-provider';
import { api } from '@/lib/api';

/**
 * Shown to signed-in users without a profile: one step at a time, saved in a
 * single SaveProfile at the end. Saving flips `onboarded` (GetMe), and the
 * route guards in app/_layout.tsx move on to the tabs.
 */
export default function OnboardingScreen() {
  const { t, languages, previewLanguages } = useI18n();
  const { signOut } = useAuth();
  const queryClient = useQueryClient();
  // Until the person chooses, they're in the device language.
  const [draft, setDraft] = useState(() => emptyDraft(languages.primary));
  const [index, setIndex] = useState(0);
  const scroll = useRef<ScrollViewInstance>(null);
  const step = steps[index];

  // The rest of onboarding is shown in the languages just picked.
  useEffect(() => {
    previewLanguages({ primary: draft.primaryLanguage, secondary: draft.secondaryLanguage });
  }, [draft.primaryLanguage, draft.secondaryLanguage, previewLanguages]);
  useEffect(() => () => previewLanguages(null), [previewLanguages]);

  const save = useMutation({
    mutationFn: () => api.profile.saveProfile({ profile: toProfile(draft, ConductVersion) }),
    // GetMe now reports onboarded (and the saved languages).
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['me'] }),
  });

  const go = (to: number) => {
    setIndex(to);
    scroll.current?.scrollTo({ y: 0, animated: false });
  };
  const last = index === steps.length - 1;
  const valid = stepValid(step, draft);

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill}>
        <KeyboardAvoidingView behavior="padding" style={styles.fill}>
          <View style={styles.header}>
            <ThemedText type="small" themeColor="textSecondary" style={styles.grow}>
              {t('onboarding.step', { step: index + 1, total: steps.length })}
            </ThemedText>
            <LanguageSwitch />
          </View>
          <ScrollView ref={scroll} contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
            {index === 0 && <ThemedText type="subtitle">{t('onboarding.welcome')}</ThemedText>}
            <ThemedText type={index === 0 ? 'smallBold' : 'subtitle'}>{t(stepTitles[step])}</ThemedText>
            <StepView step={step} draft={draft} onChange={setDraft} />
            {index === 0 && (
              <Pressable accessibilityRole="button" onPress={signOut} style={styles.signOut}>
                <ThemedText type="link">{t('account.signOut')}</ThemedText>
              </Pressable>
            )}
          </ScrollView>
          {save.isError && <ThemedText style={styles.error}>{saveErrorMessage(save.error, t)}</ThemedText>}
          <View style={styles.footer}>
            {index > 0 && (
              <Pressable accessibilityRole="button" onPress={() => go(index - 1)} style={styles.back} disabled={save.isPending}>
                <ThemedText>{t('common.back')}</ThemedText>
              </Pressable>
            )}
            <PrimaryButton
              label={last ? t('onboarding.finish') : t('common.continue')}
              disabled={!valid}
              busy={save.isPending}
              onPress={() => (last ? save.mutate() : go(index + 1))}
            />
          </View>
        </KeyboardAvoidingView>
      </SafeAreaView>
    </ThemedView>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  grow: { flex: 1 },
  header: { flexDirection: 'row', alignItems: 'center', paddingHorizontal: Spacing.four, paddingTop: Spacing.three },
  content: { padding: Spacing.four, gap: Spacing.four, width: '100%', maxWidth: MaxContentWidth, alignSelf: 'center' },
  footer: { flexDirection: 'row', gap: Spacing.three, padding: Spacing.four, alignItems: 'center' },
  back: { paddingVertical: Spacing.three, paddingHorizontal: Spacing.three },
  signOut: { alignSelf: 'center' },
  error: { color: Brand.red, textAlign: 'center', paddingHorizontal: Spacing.four },
});
