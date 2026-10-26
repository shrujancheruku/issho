import { Tabs, TabList, TabTrigger, TabSlot, type TabTriggerSlotProps, type TabListProps } from 'expo-router/ui';
import { Pressable, StyleSheet, View } from 'react-native';

import { CountBadge } from '@/components/count-badge';
import { TabIcon } from '@/components/tab-icons';
import { ThemedText } from '@/components/themed-text';
import { BottomTabInset, MaxContentWidth, Spacing } from '@/constants/theme';
import { useTogether } from '@/features/together/data';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

/**
 * The web's tab bar, drawn like the phones' native one (react-native-web has
 * no native tabs): Together, Plans and You along the bottom, Together with
 * a badge for what needs an answer.
 */
export default function AppTabs() {
  const { t } = useI18n();
  const { needsYou } = useTogether();
  return (
    <Tabs>
      <TabSlot style={{ height: '100%' }} />
      <TabList asChild>
        <TabBar>
          <TabTrigger name="index" href="/" asChild>
            <TabButton icon="together" badge={needsYou.length} badgeLabel={t('tabs.needsYou', { count: needsYou.length })}>
              {t('tabs.together')}
            </TabButton>
          </TabTrigger>
          <TabTrigger name="plans" href="/plans" asChild>
            <TabButton icon="plans">{t('tabs.plans')}</TabButton>
          </TabTrigger>
          <TabTrigger name="profile" href="/profile" asChild>
            <TabButton icon="you">{t('tabs.you')}</TabButton>
          </TabTrigger>
        </TabBar>
      </TabList>
    </Tabs>
  );
}

// The slot ref is typed for a function component, not Pressable's host element, so it is not forwarded.
function TabButton({
  children,
  isFocused,
  icon,
  badge = 0,
  badgeLabel = '',
  ref: _ref,
  ...props
}: TabTriggerSlotProps & { icon: 'together' | 'plans' | 'you'; badge?: number; badgeLabel?: string }) {
  const theme = useTheme();
  const color = isFocused ? theme.link : theme.textSecondary;
  return (
    <Pressable {...props} accessibilityRole="tab" accessibilityState={{ selected: isFocused }} style={({ pressed }) => [styles.tab, pressed && styles.pressed]}>
      <View>
        <TabIcon name={icon} color={color} />
        <View style={styles.badge}>
          <CountBadge count={badge} label={badgeLabel} />
        </View>
      </View>
      <ThemedText type="smallBold" style={[styles.label, { color }]}>
        {children}
      </ThemedText>
    </Pressable>
  );
}

function TabBar(props: TabListProps) {
  const theme = useTheme();
  return (
    <View {...props} style={[styles.bar, { backgroundColor: theme.surface, borderTopColor: theme.backgroundSelected }]}>
      <View style={styles.inner}>{props.children}</View>
    </View>
  );
}

const styles = StyleSheet.create({
  bar: { position: 'absolute', start: 0, end: 0, bottom: 0, height: BottomTabInset, borderTopWidth: 1, alignItems: 'center' },
  inner: { flexDirection: 'row', justifyContent: 'space-around', width: '100%', maxWidth: MaxContentWidth, paddingTop: Spacing.two },
  tab: { alignItems: 'center', gap: Spacing.half, minWidth: 96, minHeight: 48 },
  label: { fontSize: 12, lineHeight: 16 },
  badge: { position: 'absolute', top: -4, start: 18 },
  pressed: { opacity: 0.7 },
});
