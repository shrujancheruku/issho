import { Pressable, StyleSheet } from 'react-native';
import Svg, { Path } from 'react-native-svg';

import { ThemedText } from '@/components/themed-text';
import { Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { isRightToLeft } from '@/i18n/direction';
import { useI18n } from '@/i18n/i18n-provider';

/** A secondary action as a link, 44 points tall to tap. */
export function TextLink({
  label,
  onPress,
  muted,
  center,
  disabled,
}: {
  label: string;
  onPress: () => void;
  muted?: boolean;
  center?: boolean;
  disabled?: boolean;
}) {
  return (
    <Pressable
      accessibilityRole="button"
      onPress={onPress}
      disabled={disabled}
      hitSlop={4}
      style={({ pressed }) => [styles.link, center && styles.center, (pressed || disabled) && styles.pressed]}>
      <ThemedText themeColor={muted ? 'textSecondary' : 'link'}>{label}</ThemedText>
    </Pressable>
  );
}

/** Back to where the person came from, named ("‹ Together"). */
export function BackLink({ label, onPress, color }: { label: string; onPress: () => void; color?: string }) {
  const theme = useTheme();
  const { languages } = useI18n();
  const c = color ?? theme.link;
  // The chevron points back in the reading direction.
  const flip = isRightToLeft(languages.primary);
  return (
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={label}
      onPress={onPress}
      hitSlop={8}
      style={({ pressed }) => [styles.back, pressed && styles.pressed]}>
      <Svg width={20} height={20} viewBox="0 0 20 20" fill="none" style={flip && styles.flip}>
        <Path d="M12.5 4.5 7 10l5.5 5.5" stroke={c} strokeWidth={2.2} strokeLinecap="round" strokeLinejoin="round" />
      </Svg>
      <ThemedText style={{ color: c }}>{label}</ThemedText>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  link: { minHeight: 44, justifyContent: 'center', alignSelf: 'flex-start' },
  center: { alignSelf: 'center' },
  pressed: { opacity: 0.6 },
  back: { minHeight: 44, flexDirection: 'row', alignItems: 'center', gap: Spacing.half, alignSelf: 'flex-start' },
  flip: { transform: [{ scaleX: -1 }] },
});
