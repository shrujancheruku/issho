import { Tabs } from 'expo-router';
import { Image, Platform, type ColorValue } from 'react-native';

import { useI18n } from '@/i18n/i18n-provider';
import { useTheme } from '@/hooks/use-theme';

// The tab bar icons are v1's, drawn to be tinted.
const icons = {
  together: require('@/assets/images/tabIcons/together.png'),
  plans: require('@/assets/images/tabIcons/plans.png'),
  you: require('@/assets/images/tabIcons/you.png'),
};

/** Together, Plans, You (docs/spec/app-shape.md). Making, the owner's and insiders' fourth tab, comes with its ticket. */
export default function TabsLayout() {
  const { t } = useI18n();
  const theme = useTheme();
  const tab = (icon: keyof typeof icons) => ({
    tabBarIcon: ({ color }: { color: ColorValue }) => <Image source={icons[icon]} style={{ width: 24, height: 24, tintColor: color }} />,
  });
  return (
    <Tabs
      screenOptions={{
        headerShown: false,
        tabBarActiveTintColor: theme.link,
        tabBarInactiveTintColor: theme.textSecondary,
        tabBarStyle: {
          backgroundColor: theme.surface,
          borderTopColor: theme.backgroundSelected,
          // On the web the default 49-point bar clips the label's descenders (the dot under Vietnamese "ạ"); phones add their own safe-area height.
          ...Platform.select({ web: { height: 64, paddingBottom: 8 } }),
        },
      }}>
      <Tabs.Screen name="together" options={{ title: t('tabs.together'), ...tab('together') }} />
      <Tabs.Screen name="plans" options={{ title: t('tabs.plans'), ...tab('plans') }} />
      <Tabs.Screen name="you" options={{ title: t('tabs.you'), ...tab('you') }} />
    </Tabs>
  );
}
