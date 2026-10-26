import { StyleSheet, View, type StyleProp, type ViewStyle } from 'react-native';

import { ThemedText } from '@/components/themed-text';
import { Brand } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';

/**
 * A person or group as their initial in a circle (Issho shows no photos
 * outside confirmed outings). `ring`: gold for a crew or someone who said
 * yes, dashed for someone still deciding.
 */
export function Avatar({
  name,
  size = 36,
  ring,
  fill,
  style,
}: {
  name: string;
  size?: number;
  ring?: 'gold' | 'dashed';
  /** Background and text colours, when not the default sunk surface. */
  fill?: { background: string; text: string };
  style?: StyleProp<ViewStyle>;
}) {
  const theme = useTheme();
  const initial = Array.from(name.trim())[0]?.toUpperCase() ?? '?';
  return (
    <View
      aria-hidden
      style={[
        styles.circle,
        {
          width: size,
          height: size,
          borderRadius: size / 2,
          backgroundColor: fill?.background ?? theme.backgroundElement,
          borderWidth: ring ? Math.max(2, Math.round(size / 18)) : 0,
          borderColor: ring === 'gold' ? Brand.gold : theme.lineStrong,
          borderStyle: ring === 'dashed' ? 'dashed' : 'solid',
        },
        style,
      ]}>
      <ThemedText style={{ fontSize: Math.round(size * 0.36), lineHeight: Math.round(size * 0.5), fontWeight: 700, color: fill?.text ?? theme.text }}>
        {initial}
      </ThemedText>
    </View>
  );
}

const styles = StyleSheet.create({
  circle: { alignItems: 'center', justifyContent: 'center' },
});
