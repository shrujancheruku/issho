import { NativeTabs } from 'expo-router/native-tabs';

import { Brand } from '@/constants/theme';
import { useTogether } from '@/features/together/data';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

/**
 * Three tabs: Together (the people you're meeting: what needs you, your
 * group chats, plans waiting for a date), Plans (finding the next outing)
 * and You. Together's badge counts what needs an answer.
 */
export default function AppTabs() {
  const theme = useTheme();
  const { t } = useI18n();
  const { needsYou } = useTogether();

  // The selected tab in the accent; theme.link is the accent readable as text.
  return (
    <NativeTabs
      backgroundColor={theme.surface}
      indicatorColor={theme.backgroundElement}
      badgeBackgroundColor={Brand.gold}
      badgeTextColor={Brand.ink}
      iconColor={{ selected: theme.link }}
      labelStyle={{ selected: { color: theme.link } }}>
      <NativeTabs.Trigger name="index">
        <NativeTabs.Trigger.Label>{t('tabs.together')}</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon src={require('@/assets/images/tabIcons/together.png')} renderingMode="template" />
        {needsYou.length > 0 && <NativeTabs.Trigger.Badge>{String(needsYou.length)}</NativeTabs.Trigger.Badge>}
      </NativeTabs.Trigger>

      <NativeTabs.Trigger name="plans">
        <NativeTabs.Trigger.Label>{t('tabs.plans')}</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon src={require('@/assets/images/tabIcons/plans.png')} renderingMode="template" />
      </NativeTabs.Trigger>

      <NativeTabs.Trigger name="profile">
        <NativeTabs.Trigger.Label>{t('tabs.you')}</NativeTabs.Trigger.Label>
        <NativeTabs.Trigger.Icon src={require('@/assets/images/tabIcons/you.png')} renderingMode="template" />
      </NativeTabs.Trigger>
    </NativeTabs>
  );
}
