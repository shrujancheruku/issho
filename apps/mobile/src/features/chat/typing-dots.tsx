import { StyleSheet, View } from 'react-native';
import Animated, { FadeIn, FadeOut } from 'react-native-reanimated';

import { ThemedText } from '@/components/themed-text';
import { Motion } from '@/constants/motion';
import { Spacing } from '@/constants/theme';
import { useMotion } from '@/hooks/use-motion';
import { useScreenActive } from '@/hooks/use-screen-active';
import { useTheme } from '@/hooks/use-theme';
import { useI18n } from '@/i18n/i18n-provider';

// Each dot swells and brightens, one after another: twice the pace of the
// live loop's breath, which would read as sleepy for someone typing.
const breathe = {
  '0%': { opacity: 0.35, transform: [{ scale: 0.8 }] },
  '30%': { opacity: 1, transform: [{ scale: 1 }] },
  '60%': { opacity: 0.35, transform: [{ scale: 0.8 }] },
  '100%': { opacity: 0.35, transform: [{ scale: 0.8 }] },
};
const beat = Motion.breathe / 2;

/**
 * Who else is typing in a chat: three breathing dots and their name (or
 * how many). Nothing while nobody is. With reduced motion the dots hold
 * still; the words say it anyway.
 */
export function TypingDots({ names }: { names: string[] }) {
  const { t } = useI18n();
  const theme = useTheme();
  const { loops } = useMotion();
  const active = useScreenActive();
  if (!names.length) return null;
  return (
    <Animated.View
      entering={FadeIn.duration(Motion.change)}
      exiting={FadeOut.duration(Motion.change)}
      style={styles.row}
      accessibilityLiveRegion="polite">
      <View style={styles.dots} aria-hidden>
        {[0, 1, 2].map((i) => (
          <Animated.View
            key={i}
            style={[
              styles.dot,
              { backgroundColor: theme.textSecondary },
              loops && {
                animationName: breathe,
                animationDuration: beat,
                animationDelay: (i * beat) / 6,
                animationIterationCount: 'infinite',
                animationTimingFunction: 'ease-in-out',
                animationPlayState: active ? 'running' : 'paused',
              },
            ]}
          />
        ))}
      </View>
      <ThemedText type="small" themeColor="textSecondary">
        {names.length === 1 ? t('chat.typing', { name: names[0] }) : t('chat.typingMany', { count: names.length })}
      </ThemedText>
    </Animated.View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: 'row', alignItems: 'center', gap: Spacing.two, paddingHorizontal: Spacing.three, paddingVertical: Spacing.one },
  dots: { flexDirection: 'row', gap: 3 },
  dot: { width: 6, height: 6, borderRadius: 3 },
});
