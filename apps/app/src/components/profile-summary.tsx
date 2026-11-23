import { useRouter } from 'expo-router';
import { useEffect, useState } from 'react';
import { StyleSheet, Text, View } from 'react-native';
import type { Profile } from '@issho/api-client';

import { useApi } from '@/api/api-provider';
import { Button } from '@/components/form';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { GENDERS, LANGUAGES, offersWomenOnly } from '@/profile/answers';

/** The You tab's profile: what the person gave, and the way to change it (or to give it, when they have not). */
export function ProfileSummary() {
  const { t } = useI18n();
  const theme = useTheme();
  const api = useApi();
  const router = useRouter();
  // undefined until the API answers; null when there is none (or it could not be read).
  const [profile, setProfile] = useState<Profile | null | undefined>(undefined);

  useEffect(() => {
    let live = true;
    api.profile
      .getProfile({})
      .then((res) => live && setProfile(res.profile ?? null))
      .catch(() => live && setProfile(null));
    return () => {
      live = false;
    };
  }, [api]);

  if (profile === undefined) return null;
  const open = () => router.push({ pathname: '/profile', params: { next: '/you' } });
  if (profile === null) {
    return (
      <View testID="profile" style={styles.box}>
        <Text style={[styles.text, { color: theme.textSecondary }]}>{t('you.profile.none')}</Text>
        <Button label={t('you.profile.give')} onPress={open} />
      </View>
    );
  }
  const languages = profile.languages.map((code) => LANGUAGES.find((l) => l.code === code)).map((l, i) => (l ? t(l.labelKey) : profile.languages[i]));
  const gender = GENDERS.find((g) => g.value === profile.gender);
  const rows: [string, string][] = [
    [t('you.profile.gender'), gender ? t(gender.labelKey) : profile.gender],
    [t('you.profile.languages'), languages.join(', ')],
    [t('you.profile.city'), profile.livesInCity ? t('you.profile.yes') : t('you.profile.no')],
  ];
  if (offersWomenOnly(profile.gender as never) && profile.onlyWomenAndNonBinary) rows.push([t('you.profile.onlyWomen'), t('you.profile.yes')]);
  return (
    <View testID="profile" style={styles.box}>
      <Text accessibilityRole="header" style={[styles.heading, { color: theme.text }]}>
        {t('you.profile.title')}
      </Text>
      <Text style={[styles.name, { color: theme.text }]}>{profile.name}</Text>
      {rows.map(([label, value]) => (
        <View key={label} style={styles.row}>
          <Text style={[styles.text, { color: theme.textSecondary }]}>{label}</Text>
          <Text style={[styles.text, { color: theme.text }]}>{value}</Text>
        </View>
      ))}
      <Button quiet label={t('you.profile.change')} onPress={open} />
    </View>
  );
}

const styles = StyleSheet.create({
  box: { gap: Spacing.two },
  heading: { fontSize: FontSize.heading, fontWeight: '700' },
  name: { fontSize: FontSize.default, fontWeight: '600' },
  row: { gap: Spacing.half },
  text: { fontSize: FontSize.default },
});
