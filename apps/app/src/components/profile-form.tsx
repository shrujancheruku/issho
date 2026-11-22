import { useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import { ProfileProblemReason, profileProblem } from '@issho/api-client';

import { Button, Choice, ErrorText, Field, Toggle } from '@/components/form';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { useApi } from '@/api/api-provider';
import { type Answers, GENDERS, isAdult, isRealPastDate, LANGUAGES, localDay, offersWomenOnly, validName } from '@/profile/answers';
import { languageName, searchLanguages } from '@/profile/languages';

type Step = 'name' | 'dateOfBirth' | 'gender' | 'languages' | 'city' | 'conduct';
/** The order the spec gives (docs/spec/people.md, "The profile"). */
const STEPS: readonly Step[] = ['name', 'dateOfBirth', 'gender', 'languages', 'city', 'conduct'];
const RULES = ['profile.conduct.rule1', 'profile.conduct.rule2', 'profile.conduct.rule3', 'profile.conduct.rule4', 'profile.conduct.rule5', 'profile.conduct.rule6'] as const;

/**
 * The languages to tick: what the search finds, right under the box (docs/spec/languages.md:
 * any language can be listed), then the likely ones, then any other already ticked.
 */
function languageChoices(ticked: string[], query: string, lang: Parameters<typeof languageName>[1], t: (key: StringKey) => string) {
  const likely = LANGUAGES.map((l) => ({ code: l.code as string, name: t(l.labelKey) }));
  const out: { code: string; name: string }[] = [];
  const have = new Set<string>();
  const add = (l: { code: string; name: string }) => {
    if (have.has(l.code)) return;
    have.add(l.code);
    out.push(l);
  };
  searchLanguages(query, lang).forEach(add);
  likely.forEach(add);
  ticked.map((code) => ({ code, name: languageName(code, lang) })).forEach(add);
  return out;
}

export const EMPTY_ANSWERS: Answers = {
  name: '',
  dateOfBirth: '',
  gender: '',
  languages: [],
  livesInCity: null,
  acceptedCodeOfConduct: false,
  onlyWomenAndNonBinary: false,
};

/**
 * The profile's questions. Given no `existing` answers it asks them one step at
 * a time, in order, and the first save sends the date of birth. Given the
 * saved ones (the You tab's "change") it shows every question but the date of
 * birth, which cannot change, on one screen. `onSaved` runs once the API has
 * saved it.
 */
export function ProfileForm({ existing, onSaved }: { existing?: Answers; onSaved: () => void }) {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const api = useApi();
  const editing = existing !== undefined;
  const [answers, setAnswers] = useState<Answers>(existing ?? EMPTY_ANSWERS);
  const steps = editing ? STEPS.filter((s) => s !== 'dateOfBirth') : STEPS;
  const [index, setIndex] = useState(0);
  const [problem, setProblem] = useState<StringKey | null>(null);
  const [busy, setBusy] = useState(false);
  const [search, setSearch] = useState('');
  const set = (patch: Partial<Answers>) => {
    setAnswers((a) => ({ ...a, ...patch }));
    setProblem(null);
  };

  /** What is wrong with a step's answer, or null. */
  const check = (step: Step): StringKey | null => {
    switch (step) {
      case 'name':
        return validName(answers.name) ? null : 'profile.name.invalid';
      case 'dateOfBirth': {
        const today = localDay();
        if (!isRealPastDate(answers.dateOfBirth, today)) return 'profile.dob.invalid';
        return isAdult(answers.dateOfBirth, today) ? null : 'profile.dob.underAge';
      }
      case 'gender':
        return answers.gender ? null : 'profile.gender.required';
      case 'languages':
        return answers.languages.length > 0 ? null : 'profile.languages.required';
      case 'city':
        return answers.livesInCity === null ? 'profile.city.required' : null;
      case 'conduct':
        return answers.acceptedCodeOfConduct ? null : 'profile.conduct.required';
    }
  };

  const save = async () => {
    setBusy(true);
    try {
      await api.profile.saveProfile({
        name: answers.name.trim(),
        // The date of birth is asked once: a later save leaves it out.
        dateOfBirth: editing ? '' : answers.dateOfBirth,
        gender: answers.gender,
        languages: answers.languages,
        livesInCity: answers.livesInCity === true,
        // Accepted by the person at the first save only; a change never accepts a version.
        acceptedCodeOfConduct: !editing,
        onlyWomenAndNonBinary: offersWomenOnly(answers.gender) && answers.onlyWomenAndNonBinary,
      });
      onSaved();
    } catch (err) {
      const reason = profileProblem(err);
      if (reason === ProfileProblemReason.UNDER_AGE) {
        // The API's clock is the city's; it stops the person even where the app's own did not.
        setProblem('profile.dob.underAge');
        if (!editing) setIndex(steps.indexOf('dateOfBirth'));
      } else {
        setProblem('profile.saveFailed');
      }
    } finally {
      setBusy(false);
    }
  };

  const last = index === steps.length - 1;
  const advance = () => {
    // On one screen (editing) every question is checked; stepping, only this one.
    const bad = (editing ? steps : [steps[index]]).map(check).find((p) => p !== null) ?? null;
    if (bad) return setProblem(bad);
    setProblem(null);
    if (editing || last) void save();
    else setIndex(index + 1);
  };

  const shown = editing ? steps : [steps[index]];
  return (
    <View style={styles.form}>
      {!editing && <Text style={[styles.help, { color: theme.textSecondary }]}>{t('profile.step', { n: index + 1, total: steps.length })}</Text>}
      {shown.map((step) => (
        <View key={step} style={styles.step}>
          {step === 'name' && (
            <>
              <Field label={t('profile.name.label')} value={answers.name} onChangeText={(name) => set({ name })} autoComplete="name" textContentType="givenName" maxLength={80} />
              <Text style={[styles.help, { color: theme.textSecondary }]}>{t('profile.name.help')}</Text>
            </>
          )}
          {step === 'dateOfBirth' && (
            <>
              <Field
                label={t('profile.dob.label')}
                value={answers.dateOfBirth}
                onChangeText={(dateOfBirth) => set({ dateOfBirth })}
                keyboardType="numbers-and-punctuation"
                autoComplete="birthdate-full"
                maxLength={10}
              />
              <Text style={[styles.help, { color: theme.textSecondary }]}>{t('profile.dob.help')}</Text>
            </>
          )}
          {step === 'gender' && (
            <>
              <Text accessibilityRole="header" style={[styles.question, { color: theme.text }]}>
                {t('profile.gender.label')}
              </Text>
              <Text style={[styles.help, { color: theme.textSecondary }]}>{t('profile.gender.why')}</Text>
              {GENDERS.map((g) => (
                <Choice
                  key={g.value}
                  kind="radio"
                  label={t(g.labelKey)}
                  selected={answers.gender === g.value}
                  // Women-only is for woman and non-binary only: it turns itself off for any other answer.
                  onPress={() => set({ gender: g.value, onlyWomenAndNonBinary: offersWomenOnly(g.value) && answers.onlyWomenAndNonBinary })}
                />
              ))}
              {offersWomenOnly(answers.gender) && (
                <Toggle
                  label={t('profile.womenOnly.label')}
                  help={t('profile.womenOnly.help')}
                  value={answers.onlyWomenAndNonBinary}
                  onValueChange={(onlyWomenAndNonBinary) => set({ onlyWomenAndNonBinary })}
                />
              )}
            </>
          )}
          {step === 'languages' && (
            <>
              <Text accessibilityRole="header" style={[styles.question, { color: theme.text }]}>
                {t('profile.languages.label')}
              </Text>
              <Field label={t('profile.languages.search')} value={search} onChangeText={setSearch} autoCapitalize="none" autoCorrect={false} />
              {search.trim() !== '' && searchLanguages(search, lang).length === 0 && (
                <Text style={[styles.help, { color: theme.textSecondary }]}>{t('profile.languages.none')}</Text>
              )}
              {languageChoices(answers.languages, search, lang, t).map((l) => (
                <Choice
                  key={l.code}
                  kind="checkbox"
                  label={l.name}
                  selected={answers.languages.includes(l.code)}
                  onPress={() => set({ languages: answers.languages.includes(l.code) ? answers.languages.filter((c) => c !== l.code) : [...answers.languages, l.code] })}
                />
              ))}
            </>
          )}
          {step === 'city' && (
            <>
              <Text accessibilityRole="header" style={[styles.question, { color: theme.text }]}>
                {t('profile.city.label')}
              </Text>
              <Text style={[styles.help, { color: theme.textSecondary }]}>{t('profile.city.help')}</Text>
              <Choice kind="radio" label={t('profile.city.yes')} selected={answers.livesInCity === true} onPress={() => set({ livesInCity: true })} />
              <Choice kind="radio" label={t('profile.city.no')} selected={answers.livesInCity === false} onPress={() => set({ livesInCity: false })} />
            </>
          )}
          {step === 'conduct' && (
            <>
              <Text accessibilityRole="header" style={[styles.question, { color: theme.text }]}>
                {t('profile.conduct.title')}
              </Text>
              {RULES.map((rule, i) => (
                <Text key={rule} style={[styles.rule, { color: theme.text }]}>
                  {i + 1}. {t(rule)}
                </Text>
              ))}
              {editing ? (
                // Said once, at the first save: a change shows it as a fact and sends no acceptance.
                <Text style={[styles.rule, { color: theme.textSecondary }]}>{t('profile.conduct.accepted')}</Text>
              ) : (
                <Choice kind="checkbox" label={t('profile.conduct.agree')} selected={answers.acceptedCodeOfConduct} onPress={() => set({ acceptedCodeOfConduct: !answers.acceptedCodeOfConduct })} />
              )}
            </>
          )}
        </View>
      ))}
      {problem && <ErrorText>{t(problem)}</ErrorText>}
      <Button label={editing || last ? t('profile.save') : t('profile.next')} onPress={advance} disabled={busy} />
      {!editing && index > 0 && <Button quiet label={t('profile.back')} onPress={() => { setProblem(null); setIndex(index - 1); }} disabled={busy} />}
    </View>
  );
}

const styles = StyleSheet.create({
  form: { gap: Spacing.three },
  step: { gap: Spacing.two },
  question: { fontSize: FontSize.heading, fontWeight: '700' },
  help: { fontSize: FontSize.small },
  rule: { fontSize: FontSize.default },
});
