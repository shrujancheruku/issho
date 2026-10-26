import { Platform, StyleSheet, Text, type TextProps, type TextStyle } from 'react-native';

import {
  bodyFontFor,
  BodyFontLanguages,
  DisplayFont,
  DisplayFontLanguages,
  Fonts,
  TallLineScale,
  TallScriptLanguages,
  ThemeColor,
} from '@/constants/theme';
import { useTheme } from '@/hooks/use-theme';
import { isRightToLeft } from '@/i18n/direction';
import { useShownLanguage } from '@/i18n/i18n-provider';
import { baseLanguage } from '@/i18n/strings';

export type ThemedTextProps = TextProps & {
  type?: 'default' | 'title' | 'small' | 'smallBold' | 'subtitle' | 'link' | 'linkPrimary' | 'code';
  themeColor?: ThemeColor;
  /**
   * The language of the text, which picks the typeface (see
   * BodyFontLanguages). Defaults to the language being shown; pass it for
   * text in another one, such as the second language side by side.
   */
  lang?: string;
};

export function ThemedText({ style, type = 'default', themeColor, lang, ...rest }: ThemedTextProps) {
  const theme = useTheme();
  const shown = useShownLanguage();
  const tag = lang || shown || 'en';
  const language = baseLanguage(tag) || 'en';
  const color = theme[themeColor ?? (type === 'linkPrimary' ? 'link' : 'text')];
  let flat: TextStyle = StyleSheet.flatten([{ color }, styles[type], style]) ?? {};
  if (flat.lineHeight && TallScriptLanguages.has(language)) {
    flat = { ...flat, lineHeight: Math.round(flat.lineHeight * TallLineScale) };
  }
  // On the web, lang also lets the browser pick the right glyphs (Chinese
  // or Japanese forms of the same character) from the system font.
  // Text keeps its own direction inside a layout of the other one (English
  // beside Arabic): dir on the web, writingDirection on iOS (Android follows
  // the text's first letters by itself).
  const rtl = isRightToLeft(tag);
  const webLang = Platform.OS === 'web' ? ({ lang: tag, dir: rtl ? 'rtl' : 'ltr' } as TextProps) : {};
  if (Platform.OS === 'ios') flat = { ...flat, writingDirection: rtl ? 'rtl' : 'ltr' };

  // Standout text in a language the display face can't set: the system font,
  // heavy, so it still stands out.
  if (flat.fontFamily === DisplayFont && !DisplayFontLanguages.has(language)) {
    const { fontFamily: _display, ...system } = flat;
    return <Text style={{ ...system, fontWeight: 800 }} {...webLang} {...rest} />;
  }
  // Text without its own family gets the body face in the weight it asked for.
  // The weight is then dropped: each weight is a separate family, and keeping
  // it would make iOS fake a bold and Android fall back to the system font.
  // In a language the body face can't set, the system font keeps the weight.
  if (!flat.fontFamily) {
    if (!BodyFontLanguages.has(language)) return <Text style={flat} {...webLang} {...rest} />;
    const { fontWeight, ...unweighted } = flat;
    return <Text style={{ ...unweighted, fontFamily: bodyFontFor(fontWeight) }} {...webLang} {...rest} />;
  }
  return <Text style={flat} {...webLang} {...rest} />;
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
    // Display lines are at least 1.45 em: Dela Gothic's ascent and descent
    // need it, or Android cuts off descenders.
    lineHeight: 64,
  },
  subtitle: {
    fontFamily: DisplayFont,
    fontSize: 26,
    lineHeight: 38,
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
