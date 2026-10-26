import { CrewStatus, GroupSize, Interest } from '@issho/api-client';
import { useMutation } from '@tanstack/react-query';
import { router } from 'expo-router';
import { ActivityIndicator, ScrollView, StyleSheet, View } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';

import { Chip } from '@/components/chip';
import { ListGroup, ListRow } from '@/components/list-group';
import { ScreenHeader, SectionTitle } from '@/components/screen-header';
import { Segmented } from '@/components/segmented';
import { TextLink } from '@/components/text-link';
import { ThemedText } from '@/components/themed-text';
import { ThemedView } from '@/components/themed-view';
import { BottomTabInset, FormWidth, Spacing } from '@/constants/theme';
import { setAppearance, useAppearance, type AppearancePreference } from '@/features/appearance/preference';
import { useAuth } from '@/features/auth/auth-provider';
import { useMyCrews } from '@/features/crews/data';
import { usePlanNames } from '@/features/plans/names';
import { pairText } from '@/features/plans/plan-card';
import type { Step } from '@/features/profile/draft';
import { interestLevels, interestStyle } from '@/features/profile/interest';
import { useProfile } from '@/features/profile/use-profile';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { api } from '@/lib/api';
import { confirm, notify } from '@/lib/dialogs';

const groupSizes: Partial<Record<GroupSize, StringKey>> = {
  [GroupSize.SMALL]: 'profile.groupSize.small',
  [GroupSize.MEDIUM]: 'profile.groupSize.medium',
  [GroupSize.LARGE]: 'profile.groupSize.large',
};

const edit = (step: Step) => router.push({ pathname: '/profile-edit/[step]', params: { step } });

/**
 * You: who you are to Issho (name, area, crews), what you like doing, how
 * you meet (each part opens its own editor), how the app looks, and the
 * account.
 */
export default function YouScreen() {
  const { t } = useI18n();
  const theme = useTheme();
  const profile = useProfile();
  const crews = useMyCrews();
  const names = usePlanNames();
  const p = profile.data?.profile;
  const activeCrews = crews.data?.filter((c) => c.status === CrewStatus.ACTIVE).length ?? 0;

  // Loved first: the chips read like the Plans sections.
  const liked = p
    ? [...p.activities]
        .filter((a) => a.interest !== Interest.UNSPECIFIED)
        .sort((a, b) => interestLevels.indexOf(b.interest as never) - interestLevels.indexOf(a.interest as never))
    : [];
  const languageLine = p
    ? [p.primaryLanguage, p.secondaryLanguage]
        .filter(Boolean)
        .map((code, i) => `${t(i === 0 ? 'language.primary' : 'you.secondLanguage')}: ${names.language(code)}`)
        .join('. ')
    : '';

  return (
    <ThemedView style={styles.fill}>
      <SafeAreaView style={styles.fill} edges={['top', 'left', 'right']}>
        <ScrollView contentContainerStyle={styles.content}>
          <ScreenHeader title={t('you.title')} />
          {!p ? (
            profile.isError ? (
              <TextLink label={t('common.retry')} onPress={() => void profile.refetch()} />
            ) : (
              <ActivityIndicator />
            )
          ) : (
            <>
              <View style={[styles.card, { backgroundColor: theme.surface }]}>
                <ThemedText type="subtitle">{p.displayName}</ThemedText>
                <ThemedText themeColor="textSecondary">{pairText(names.area(p.homeAreaId))}</ThemedText>
                {activeCrews > 0 && (
                  <ThemedText type="small" themeColor="textSecondary">
                    {t('you.crews', { count: activeCrews })}
                  </ThemedText>
                )}
              </View>

              <View style={styles.section}>
                <View style={styles.sectionHead}>
                  <SectionTitle>{t('profile.activities')}</SectionTitle>
                  <TextLink label={t('you.change')} onPress={() => edit('activities')} />
                </View>
                <View style={styles.chips}>
                  {liked.map((a) => {
                    const style = interestStyle(a.interest, theme)!;
                    const n = names.activity(a.activityId);
                    return (
                      <Chip
                        key={a.activityId}
                        label={n.name}
                        secondary={n.secondary}
                        selected
                        selectedColors={style}
                        onPress={() => edit('activities')}
                      />
                    );
                  })}
                </View>
              </View>

              <View style={styles.section}>
                <SectionTitle>{t('you.howYouMeet')}</SectionTitle>
                <ListGroup>
                  <SettingRow title={t('profile.about')} detail={p.displayName} onPress={() => edit('about')} />
                  <SettingRow
                    title={t('profile.area')}
                    detail={`${pairText(names.area(p.homeAreaId))}. ${t('profile.maxTravel')}: ${t('profile.minutes', { count: p.maxTravelMin })}`}
                    onPress={() => edit('area')}
                  />
                  <SettingRow
                    title={t('profile.availability')}
                    detail={t('you.slots', { count: p.availability.length })}
                    onPress={() => edit('availability')}
                  />
                  <SettingRow title={t('profile.languages')} detail={languageLine} onPress={() => edit('languages')} />
                  <SettingRow
                    title={t('profile.style')}
                    detail={groupSizes[p.groupSize] ? `${t('profile.groupSize')}: ${t(groupSizes[p.groupSize]!)}` : ''}
                    onPress={() => edit('style')}
                  />
                </ListGroup>
              </View>
            </>
          )}

          <AppearanceSetting />
          <Account />
        </ScrollView>
      </SafeAreaView>
    </ThemedView>
  );
}

function SettingRow({ title, detail, onPress }: { title: string; detail: string; onPress: () => void }) {
  return (
    <ListRow onPress={onPress}>
      <View style={styles.rowText}>
        <ThemedText>{title}</ThemedText>
        {!!detail && (
          <ThemedText type="small" themeColor="textSecondary">
            {detail}
          </ThemedText>
        )}
      </View>
    </ListRow>
  );
}

/** Light, dark, or the phone's own setting (the default). */
function AppearanceSetting() {
  const { t } = useI18n();
  const pref = useAppearance();
  const options: { value: AppearancePreference; label: string }[] = [
    { value: 'system', label: t('you.appearanceSystem') },
    { value: 'light', label: t('you.appearanceLight') },
    { value: 'dark', label: t('you.appearanceDark') },
  ];
  return (
    <View style={styles.section}>
      <SectionTitle>{t('you.appearance')}</SectionTitle>
      <Segmented label={t('you.appearance')} options={options} value={pref} onChange={setAppearance} />
    </View>
  );
}

/**
 * Sign out and delete the account, as plain rows: neither is red (red is
 * for errors) and deleting asks first. No motion beyond the press.
 */
function Account() {
  const { t } = useI18n();
  const { session, signOut } = useAuth();
  const deleteAccount = useMutation({
    mutationFn: () => api.identity.deleteAccount({}),
    // The server has deleted the auth user; clear the local session too.
    onSuccess: () => signOut(),
    onError: () => notify(t('account.deleteFailed'), t('account.deleteFailedBody')),
  });
  const confirmDelete = () =>
    confirm({
      title: t('account.deleteTitle'),
      body: t('account.deleteBody'),
      confirm: t('account.deleteConfirm'),
      cancel: t('common.cancel'),
      onConfirm: () => deleteAccount.mutate(),
    });
  return (
    <View style={styles.section}>
      <SectionTitle>{t('you.account')}</SectionTitle>
      <ListGroup>
        <ListRow onPress={signOut} chevron={false}>
          <ThemedText style={styles.rowText}>{t('account.signOut')}</ThemedText>
        </ListRow>
        <ListRow onPress={deleteAccount.isPending ? undefined : confirmDelete} chevron={false}>
          <ThemedText style={styles.rowText}>{deleteAccount.isPending ? t('account.deleting') : t('account.delete')}</ThemedText>
        </ListRow>
      </ListGroup>
      <ThemedText type="small" themeColor="textSecondary">
        {t('account.signedInAs', { email: session?.user.email ?? '' })}
      </ThemedText>
    </View>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1 },
  content: {
    padding: Spacing.three,
    paddingTop: Spacing.four,
    paddingBottom: BottomTabInset + Spacing.four,
    gap: Spacing.four + 4,
    width: '100%',
    maxWidth: FormWidth,
    alignSelf: 'center',
  },
  card: { borderRadius: 16, padding: Spacing.three, gap: Spacing.half },
  section: { gap: Spacing.two },
  sectionHead: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between' },
  chips: { flexDirection: 'row', flexWrap: 'wrap', gap: Spacing.two },
  rowText: { flex: 1, minWidth: 0 },
});
