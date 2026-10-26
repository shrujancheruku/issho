import { StyleSheet, View } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Brand } from '@/constants/theme';

/** A small count: gold for things that need the person, the accent for unread. */
export function CountBadge({ count, label, tone = 'gold', accent }: { count: number; label: string; tone?: 'gold' | 'accent'; accent?: { background: string; text: string } }) {
  if (count <= 0) return null;
  const colors = tone === 'gold' ? { background: Brand.gold, text: Brand.ink } : (accent ?? { background: Brand.gold, text: Brand.ink });
  return (
    <View accessible accessibilityLabel={label} style={[styles.badge, { backgroundColor: colors.background }]}>
      <ThemedText style={[styles.text, { color: colors.text }]}>{count > 99 ? '99+' : String(count)}</ThemedText>
    </View>
  );
}

const styles = StyleSheet.create({
  badge: { minWidth: 20, height: 20, paddingHorizontal: 6, borderRadius: 10, alignItems: 'center', justifyContent: 'center' },
  text: { fontSize: 11, lineHeight: 14, fontWeight: 700 },
});
