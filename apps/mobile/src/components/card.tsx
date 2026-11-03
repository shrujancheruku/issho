import { StyleSheet, View, type ViewProps } from 'react-native';

import { Spacing } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';

/** A rounded surface that groups one thing's text: the plan, the event, the invitation. */
export function Card({ gap = Spacing.one, style, ...rest }: ViewProps & { gap?: number }) {
  const theme = useTheme();
  return <View style={[styles.card, { backgroundColor: theme.surface, gap }, style]} {...rest} />;
}

const styles = StyleSheet.create({
  card: { borderRadius: 16, padding: Spacing.three },
});
