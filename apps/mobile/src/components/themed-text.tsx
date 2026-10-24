import { Platform, StyleSheet, Text, type TextProps, type TextStyle } from 'react-native';

import { bodyFontFor, DisplayFont, Fonts, ThemeColor } from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';

export type ThemedTextProps = TextProps & {
  type?: 'default' | 'title' | 'small' | 'smallBold' | 'subtitle' | 'link' | 'linkPrimary' | 'code';
  themeColor?: ThemeColor;
};

export function ThemedText({ style, type = 'default', themeColor, ...rest }: ThemedTextProps) {
  const theme = useTheme();
  const color = theme[themeColor ?? (type === 'linkPrimary' ? 'link' : 'text')];
  const flat: TextStyle = StyleSheet.flatten([{ color }, styles[type], style]) ?? {};
  // Text without its own family gets the body face in the weight it asked for.
  // The weight is then dropped: each weight is a separate family, and keeping
  // it would make iOS fake a bold and Android fall back to the system font.
  if (!flat.fontFamily) {
    const { fontWeight, ...unweighted } = flat;
    return <Text style={{ ...unweighted, fontFamily: bodyFontFor(fontWeight) }} {...rest} />;
  }
  return <Text style={flat} {...rest} />;
}

const styles = StyleSheet.create({
  small: {
    fontSize: 14,
    lineHeight: 20,
    fontWeight: 500,
  },
  smallBold: {
    fontSize: 14,
    lineHeight: 20,
    fontWeight: 700,
  },
  default: {
    fontSize: 16,
    lineHeight: 24,
    fontWeight: 500,
  },
  // Standout text (title, subtitle) uses the brand display face; see DisplayFont.
  title: {
    fontFamily: DisplayFont,
    fontSize: 44,
    lineHeight: 56,
  },
  subtitle: {
    fontFamily: DisplayFont,
    fontSize: 26,
    lineHeight: 36,
  },
  link: {
    lineHeight: 30,
    fontSize: 14,
  },
  // Coloured theme.link in ThemedText.
  linkPrimary: {
    lineHeight: 30,
    fontSize: 14,
  },
  code: {
    fontFamily: Fonts.mono,
    fontWeight: Platform.select({ android: 700 }) ?? 500,
    fontSize: 12,
  },
});
