import { Link } from 'expo-router';
import { useEffect, useState } from 'react';
import { Pressable, StyleSheet, Text } from 'react-native';

import { useApi } from '@/api/api-provider';
import { RequireSignIn } from '@/components/require-sign-in';
import { ScreenFrame } from '@/components/screen-frame';
import { FontSize, Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import { formatCalendarDate } from '@/lib/format';
import { loadPlans } from '@/plans/load';
import { byDay, signupItems, type TogetherItem } from '@/together/items';
import { useMySignups } from '@/signup/use-my-signups';

/** Everything of the person's, ordered by time (docs/spec/app-shape.md). The signups are here; outings and "Needs you" come with their tickets. */
export default function Together() {
  return (
    <RequireSignIn path="/together">
      <TogetherList />
    </RequireSignIn>
  );
}

function TogetherList() {
  const { t, lang } = useI18n();
  const theme = useTheme();
  const api = useApi();
  const { state } = useMySignups();
  const [items, setItems] = useState<TogetherItem[] | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  const failed = loadFailed || state.status === 'failed';

  useEffect(() => {
    if (state.status !== 'ready') return;
    let live = true;
    // A signup names its plan by id only; the titles come from the city's list, in the display language.
    loadPlans(api, lang).then(
      ({ plans }) => live && setItems(byDay(signupItems(state.signups, plans))),
      () => live && setLoadFailed(true),
    );
    return () => {
      live = false;
    };
  }, [state, api, lang]);

  return (
    <ScreenFrame>
      <Text accessibilityRole="header" style={[styles.title, { color: theme.text }]}>
        {t('tabs.together')}
      </Text>
      {items?.length === 0 ? <Text style={[styles.body, { color: theme.textSecondary }]}>{t('together.empty')}</Text> : null}
      {failed ? <Text style={[styles.body, { color: theme.error }]}>{t('together.loadFailed')}</Text> : null}
      {items?.map((item) => (
        <Link key={`${item.kind}-${item.planId}`} href={{ pathname: '/plans/[id]', params: { id: item.planId } }} asChild>
          <Pressable accessibilityRole="link" style={StyleSheet.flatten([styles.row, { backgroundColor: theme.surface, borderColor: theme.backgroundSelected }])}>
            <Text style={[styles.day, { color: theme.textSecondary }]}>{formatCalendarDate(item.day, lang)}</Text>
            <Text style={[styles.body, { color: theme.text }]}>{t('together.hearAbout', { plan: item.title })}</Text>
          </Pressable>
        </Link>
      ))}
    </ScreenFrame>
  );
}

const styles = StyleSheet.create({
  title: { fontSize: FontSize.subtitle, fontWeight: '700' },
  row: { borderWidth: 1, borderRadius: 12, padding: Spacing.three, gap: Spacing.one },
  day: { fontSize: FontSize.small },
  body: { fontSize: FontSize.default },
});
