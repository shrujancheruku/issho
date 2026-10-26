import { Gender, GroupSize } from '@issho/api-client';
import { useMemo, useState, type ReactNode } from 'react';
import { ActivityIndicator, Pressable, StyleSheet, Switch, TextInput, View } from 'react-native';

import { AvailabilityGrid } from '@/components/availability-grid';
import { Chip } from '@/components/chip';
import { ThemedText } from '@/components/themed-text';
import { BodyFont, Brand, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { english, type StringKey } from '@/i18n/strings';

import { languageLabel, useActivities, useCities, useLanguages } from './data';
import {
  addSpoken,
  cycleInterest,
  MinActivities,
  removeSpoken,
  setCity,
  setGender,
  setPrimaryLanguage,
  toggleSlot,
  travelChoices,
  updateSpoken,
  type Draft,
  type Step,
} from './draft';
import { categoryOrder, interestLevels, interestStyle } from './interest';
import { LanguagePicker } from './language-picker';
import { SuggestActivities } from './suggest-activities';

export type StepProps = { draft: Draft; onChange: (d: Draft) => void };

export const stepTitles: Record<Step, StringKey> = {
  languages: 'profile.languages',
  about: 'profile.about',
  area: 'profile.area',
  availability: 'profile.availability',
  activities: 'profile.activities',
  style: 'profile.style',
  conduct: 'conduct.title',
};

export function StepView({ step, ...props }: StepProps & { step: Step }) {
  switch (step) {
    case 'languages':
      return <LanguagesStep {...props} />;
    case 'about':
      return <AboutStep {...props} />;
    case 'area':
      return <AreaStep {...props} />;
    case 'availability':
      return <AvailabilityStep {...props} />;
    case 'activities':
      return <ActivitiesStep {...props} />;
    case 'style':
      return <StyleStep {...props} />;
    case 'conduct':
      return <ConductStep {...props} />;
  }
}

function Section({ title, hint, children }: { title: string; hint?: string; children: ReactNode }) {
  return (
    <View style={styles.section}>
      <ThemedText type="smallBold">{title}</ThemedText>
      {!!hint && (
        <ThemedText type="small" themeColor="textSecondary">
          {hint}
        </ThemedText>
      )}
      {children}
    </View>
  );
}

function Wrap({ children }: { children: ReactNode }) {
  return <View style={styles.wrap}>{children}</View>;
}

function LoadState({ query }: { query: { isPending: boolean; isError: boolean; refetch: () => unknown } }) {
  const { t } = useI18n();
  if (query.isPending) return <ActivityIndicator />;
  if (query.isError)
    return (
      <Pressable onPress={() => void query.refetch()}>
        <ThemedText themeColor="textSecondary">{t('common.loadFailed')}</ThemedText>
        <ThemedText type="link">{t('common.retry')}</ThemedText>
      </Pressable>
    );
  return null;
}

function LanguagesStep({ draft, onChange }: StepProps) {
  const { t } = useI18n();
  // Display languages are named from the list that tells Simplified and
  // Traditional Chinese apart; spoken ones from the list with Chinese once.
  const languages = useLanguages();
  const displayLanguages = useLanguages(true);
  const names = useMemo(() => new Map((languages.data ?? []).map((l) => [l.code, l])), [languages.data]);
  const displayNames = useMemo(
    () => new Map((displayLanguages.data ?? []).map((l) => [l.code, l])),
    [displayLanguages.data],
  );
  const [picking, setPicking] = useState<null | 'primary' | 'secondary' | 'spoken'>(null);
  const label = (code: string) => languageLabel(displayNames, code);
  const spokenLabel = (code: string) => languageLabel(names, code);

  const pick = (code: string) => {
    if (picking === 'primary') onChange(setPrimaryLanguage(draft, code));
    if (picking === 'secondary') onChange({ ...draft, secondaryLanguage: code });
    if (picking === 'spoken') onChange(addSpoken(draft, code));
    setPicking(null);
  };

  return (
    <>
      <Section title={t('language.primary')} hint={t('language.primaryHint')}>
        <Wrap>
          <Chip label={label(draft.primaryLanguage)} selected onPress={() => setPicking('primary')} />
        </Wrap>
      </Section>
      <Section title={t('language.secondary')} hint={t('language.secondaryHint')}>
        <Wrap>
          <Chip
            label={draft.secondaryLanguage ? label(draft.secondaryLanguage) : t('language.pick')}
            selected={!!draft.secondaryLanguage}
            onPress={() => setPicking('secondary')}
          />
          {!!draft.secondaryLanguage && (
            <Chip label={t('common.none')} selected={false} onPress={() => onChange({ ...draft, secondaryLanguage: '' })} />
          )}
        </Wrap>
      </Section>
      <Section title={t('language.spoken')} hint={t('language.spokenHint')}>
        {draft.spoken.map((l) => (
          <View key={l.code} style={styles.spoken}>
            <View style={styles.spokenHeader}>
              <ThemedText style={styles.grow}>{spokenLabel(l.code)}</ThemedText>
              <Pressable accessibilityRole="button" onPress={() => onChange(removeSpoken(draft, l.code))} hitSlop={8}>
                <ThemedText type="link">{t('language.remove')}</ThemedText>
              </Pressable>
            </View>
            <Wrap>
              <Chip label={t('language.fluent')} selected={l.fluent} onPress={() => onChange(updateSpoken(draft, l.code, { fluent: true }))} />
              <Chip label={t('language.learning')} selected={!l.fluent} onPress={() => onChange(updateSpoken(draft, l.code, { fluent: false }))} />
            </Wrap>
            {!l.fluent && (
              <Wrap>
                {([1, 2, 3] as const).map((level) => (
                  <Chip
                    key={level}
                    label={t(`language.level.${level}`)}
                    selected={l.level === level}
                    onPress={() => onChange(updateSpoken(draft, l.code, { level }))}
                  />
                ))}
              </Wrap>
            )}
          </View>
        ))}
        <Pressable accessibilityRole="button" onPress={() => setPicking('spoken')}>
          <ThemedText type="link">+ {t('language.add')}</ThemedText>
        </Pressable>
      </Section>
      <LanguagePicker
        visible={picking !== null}
        display={picking !== 'spoken'}
        exclude={
          picking === 'spoken'
            ? draft.spoken.map((l) => l.code)
            : picking === 'secondary'
              ? [draft.primaryLanguage]
              : []
        }
        onPick={pick}
        onClose={() => setPicking(null)}
      />
    </>
  );
}

function AboutStep({ draft, onChange }: StepProps) {
  const { t } = useI18n();
  const theme = useTheme();
  const input = [styles.input, { color: theme.text, backgroundColor: theme.backgroundElement }];
  const genders: [Gender, StringKey][] = [
    [Gender.WOMAN, 'profile.gender.woman'],
    [Gender.MAN, 'profile.gender.man'],
    [Gender.NONBINARY, 'profile.gender.nonbinary'],
    [Gender.PREFER_NOT_TO_SAY, 'profile.gender.preferNotToSay'],
  ];
  return (
    <>
      <Section title={t('profile.displayName')} hint={t('profile.displayNameHint')}>
        <TextInput
          style={input}
          value={draft.displayName}
          onChangeText={(displayName) => onChange({ ...draft, displayName })}
          maxLength={40}
          autoComplete="given-name"
          textContentType="givenName"
        />
      </Section>
      <Section title={t('profile.birthYear')} hint={t('profile.birthYearHint')}>
        <TextInput
          style={input}
          value={draft.birthYear}
          onChangeText={(v) => onChange({ ...draft, birthYear: v.replace(/\D/g, '').slice(0, 4) })}
          keyboardType="number-pad"
          maxLength={4}
          placeholder="1995"
          placeholderTextColor={theme.textSecondary}
        />
      </Section>
      <Section title={t('profile.gender')}>
        <Wrap>
          {genders.map(([g, key]) => (
            <Chip key={g} label={t(key)} selected={draft.gender === g} onPress={() => onChange(setGender(draft, g))} />
          ))}
        </Wrap>
        <View style={styles.switchRow}>
          <ThemedText style={styles.grow}>{t('profile.sameGenderOnly')}</ThemedText>
          <Switch
            value={draft.sameGenderOnly}
            disabled={draft.gender === Gender.PREFER_NOT_TO_SAY || draft.gender === Gender.UNSPECIFIED}
            onValueChange={(sameGenderOnly) => onChange({ ...draft, sameGenderOnly })}
            trackColor={{ true: theme.accent }}
          />
        </View>
      </Section>
    </>
  );
}

function AreaStep({ draft, onChange }: StepProps) {
  const { t } = useI18n();
  const cities = useCities();
  const city = cities.data?.find((c) => c.id === draft.cityId);
  return (
    <>
      <Section title={t('profile.city')}>
        <LoadState query={cities} />
        <Wrap>
          {cities.data?.map((c) => (
            <Chip
              key={c.id}
              label={c.name}
              secondary={c.secondaryName}
              selected={c.id === draft.cityId}
              onPress={() => onChange(setCity(draft, c.id))}
            />
          ))}
        </Wrap>
      </Section>
      {city && (
        <Section title={t('profile.homeArea')} hint={t('profile.areaHint')}>
          <Wrap>
            {city.areas.map((a) => (
              <Chip
                key={a.id}
                label={a.name}
                secondary={a.secondaryName}
                selected={a.id === draft.homeAreaId}
                onPress={() => onChange({ ...draft, homeAreaId: a.id })}
              />
            ))}
          </Wrap>
        </Section>
      )}
      <Section title={t('profile.maxTravel')}>
        <Wrap>
          {travelChoices.map((m) => (
            <Chip
              key={m}
              label={t('profile.minutes', { count: m })}
              selected={draft.maxTravelMin === m}
              onPress={() => onChange({ ...draft, maxTravelMin: m })}
            />
          ))}
        </Wrap>
      </Section>
    </>
  );
}

function AvailabilityStep({ draft, onChange }: StepProps) {
  const { t } = useI18n();
  return (
    // The screen's heading is already "When you're free" (stepTitles).
    <View style={styles.section}>
      <ThemedText type="small" themeColor="textSecondary">
        {t('profile.availabilityHint')}
      </ThemedText>
      <AvailabilityGrid selected={draft.availability} onToggle={(day, b) => onChange(toggleSlot(draft, day, b))} />
    </View>
  );
}

function ActivitiesStep({ draft, onChange }: StepProps) {
  const { t } = useI18n();
  const theme = useTheme();
  const activities = useActivities(draft.cityId);
  const data = activities.data;
  const byCategory = useMemo(() => {
    const groups = new Map<string, NonNullable<typeof data>>();
    for (const a of data ?? []) groups.set(a.category, [...(groups.get(a.category) ?? []), a]);
    const rank = (c: string) => (categoryOrder.includes(c) ? categoryOrder.indexOf(c) : categoryOrder.length);
    return [...groups].sort(([a], [b]) => rank(a) - rank(b));
  }, [data]);
  const picked = Object.keys(draft.activities).length;

  return (
    <>
      <HowToChoose />
      {!!data?.length && <SuggestActivities draft={draft} onChange={onChange} activities={data} />}
      <ThemedText type="smallBold" themeColor={picked >= MinActivities ? undefined : 'error'}>
        {t('interest.minimum', { count: MinActivities })} {t('profile.activitiesPicked', { count: picked })}
      </ThemedText>
      <LoadState query={activities} />
      {byCategory.map(([category, items]) => (
        <Section key={category} title={categoryTitle(t, category)}>
          <Wrap>
            {items.map((a) => {
              const interest = draft.activities[a.id];
              const style = interest ? interestStyle(interest, theme) : undefined;
              return (
                <Chip
                  key={a.id}
                  label={a.name}
                  secondary={a.secondaryName}
                  selected={!!style}
                  selectedColors={style}
                  badge={style ? t(style.label) : undefined}
                  onPress={() => onChange(cycleInterest(draft, a.id))}
                />
              );
            })}
          </Wrap>
        </Section>
      ))}
    </>
  );
}

/** Explains the three taps before the grid, in the tiles' own colours. */
function HowToChoose() {
  const { t } = useI18n();
  const theme = useTheme();

  return (
    <View style={[styles.howTo, { borderColor: Brand.gold, backgroundColor: theme.surface }]}>
      <ThemedText type="smallBold">{t('interest.howTitle')}</ThemedText>
      {interestLevels.map((level) => {
        const st = interestStyle(level, theme)!;
        return (
          <View key={level} style={styles.legendRow}>
            <ThemedText style={styles.legendTap}>{t(st.tap)}</ThemedText>
            <View style={[styles.pill, { backgroundColor: st.background }]}>
              <ThemedText type="smallBold" style={{ color: st.text }}>
                {t(st.label)}
              </ThemedText>
            </View>
          </View>
        );
      })}
      <ThemedText type="small" themeColor="textSecondary">
        {t('interest.tapClear')}
      </ThemedText>
    </View>
  );
}

function categoryTitle(t: (k: StringKey) => string, category: string) {
  const key = `category.${category}`;
  // A category added on the server before this build knows it shows its id.
  return key in english ? t(key as StringKey) : category;
}

function StyleStep({ draft, onChange }: StepProps) {
  const { t } = useI18n();
  const sizes: [GroupSize, StringKey][] = [
    [GroupSize.SMALL, 'profile.groupSize.small'],
    [GroupSize.MEDIUM, 'profile.groupSize.medium'],
    [GroupSize.LARGE, 'profile.groupSize.large'],
  ];
  return (
    <>
      <Section title={t('profile.groupSize')}>
        <Wrap>
          {sizes.map(([s, key]) => (
            <Chip key={s} label={t(key)} selected={draft.groupSize === s} onPress={() => onChange({ ...draft, groupSize: s })} />
          ))}
        </Wrap>
      </Section>
      <Section title={t('profile.energy')}>
        <Wrap>
          {([1, 2, 3] as const).map((e) => (
            <Chip key={e} label={t(`profile.energy.${e}`)} selected={draft.energy === e} onPress={() => onChange({ ...draft, energy: e })} />
          ))}
        </Wrap>
      </Section>
      <Section title={t('profile.budget')}>
        <Wrap>
          {([0, 1, 2, 3] as const).map((b) => (
            <Chip key={b} label={t(`profile.budget.${b}`)} selected={draft.budget === b} onPress={() => onChange({ ...draft, budget: b })} />
          ))}
        </Wrap>
      </Section>
    </>
  );
}

const conductRules: StringKey[] = [
  'conduct.rule.kind',
  'conduct.rule.platonic',
  'conduct.rule.reliable',
  'conduct.rule.privacy',
  'conduct.rule.safe',
  'conduct.rule.report',
];

// The screen's heading is already "Code of conduct" (stepTitles), so this
// starts with the introduction rather than a second heading.
function ConductStep({ draft, onChange }: StepProps) {
  const { t } = useI18n();
  const theme = useTheme();
  return (
    <View style={styles.section}>
      <ThemedText themeColor="textSecondary">{t('conduct.intro')}</ThemedText>
      {conductRules.map((k) => (
        <View key={k} style={styles.rule}>
          <ThemedText style={styles.bullet}>•</ThemedText>
          <ThemedText style={styles.grow}>{t(k)}</ThemedText>
        </View>
      ))}
      <View style={styles.switchRow}>
        <ThemedText type="smallBold" style={styles.grow}>
          {t('conduct.agree')}
        </ThemedText>
        <Switch
          value={draft.conductAccepted}
          onValueChange={(conductAccepted) => onChange({ ...draft, conductAccepted })}
          trackColor={{ true: theme.accent }}
        />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  section: { gap: Spacing.two },
  wrap: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.two },
  input: { fontFamily: BodyFont.regular, borderRadius: 12, paddingHorizontal: Spacing.three, paddingVertical: Spacing.three, fontSize: 17 },
  switchRow: { flexDirection: 'row', alignItems: 'center', gap: Spacing.three, paddingVertical: Spacing.two },
  grow: { flex: 1 },
  spoken: { gap: Spacing.two, paddingVertical: Spacing.two },
  spokenHeader: { flexDirection: 'row', alignItems: 'center' },
  rule: { flexDirection: 'row', gap: Spacing.two },
  bullet: { width: 12 },
  howTo: { borderWidth: 2, borderRadius: 16, padding: Spacing.three, gap: Spacing.two },
  legendRow: { flexDirection: 'row', alignItems: 'center', gap: Spacing.three },
  legendTap: { width: 120 },
  pill: { borderRadius: 999, paddingVertical: Spacing.one, paddingHorizontal: Spacing.three },
});
