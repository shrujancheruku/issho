/**
 * The app's colours in light and dark mode, from the Issho design system
 * (docs/design-system.md). The ground and accent are "Ai" (indigo, the dye of
 * thread); the alternatives in that doc only change background, surface,
 * backgroundElement, backgroundSelected, textSecondary, accent and link.
 * Text pairs are at least 4.5:1 on background, surface and backgroundElement.
 * The person chooses light, dark or their phone's setting in You
 * (features/appearance).
 */

import '@/global.css';

import { Platform } from 'react-native';

export const Colors = {
  light: {
    /** Ai-black ink: indigo so dark it reads as black, kin to the ground. */
    text: '#131B40',
    /** Screen background ("ground"). */
    background: '#F3F5F9',
    /** Cards and sheets. */
    surface: '#FFFFFF',
    /** Inputs, unselected chips, chat bubbles ("surface-sunk"). */
    backgroundElement: '#E7EBF3',
    /** Hairlines, pressed rows and empty pills ("line"). */
    backgroundSelected: '#D5DBE7',
    /** Input and outlined-control borders: 3:1 or more ("line-strong"). */
    lineStrong: '#7A849C',
    textSecondary: '#545D73',
    /** Selection, active tabs, crew and signed-up marks. Text on it: onAccent. */
    accent: '#24307A',
    onAccent: '#FFFFFF',
    link: '#24307A',
    /** Gold as text, and the thread on light grounds: Brand.gold is 2.1:1 there. */
    goldText: '#885716',
    /**
     * "Love it", and nothing else: vermilion, kin to the logo's sun. Chips,
     * map pins and the Plans section mark. Urgency and deadlines stay gold.
     */
    love: '#CC3719',
    onLove: '#FFFFFF',
    /** Error messages only. */
    error: '#BC002D',
    ok: '#2F6B3D',
    /** Keyboard focus rings. */
    focus: '#24307A',
    /** Pins for plans that aren't one of your activities, or not in your language. */
    pinLow: '#C8CBD0',
    pinLowText: '#5E5E5E',
    pinNone: '#8A8F98',
  },
  dark: {
    text: '#F1F3F8',
    background: '#0D1330',
    surface: '#161E40',
    backgroundElement: '#222B52',
    backgroundSelected: '#2C3765',
    lineStrong: '#6C78AA',
    textSecondary: '#A9B1CB',
    accent: '#3A4A9E',
    onAccent: '#FFFFFF',
    link: '#9FB0FF',
    // Gold itself reads as text on the night ground.
    goldText: '#DEA92C',
    love: '#EB603A',
    onLove: '#131B40',
    // Lighter than the light theme's red, which is 2.75:1 on the dark ground.
    error: '#FF7A90',
    ok: '#7FC28E',
    focus: '#DEA92C',
    pinLow: '#C8CBD0',
    pinLowText: '#5E5E5E',
    pinNone: '#8A8F98',
  },
} as const;

export type Palette = (typeof Colors)['light' | 'dark'];

/**
 * Display typeface for standout and hero text (the same face as the "ISSHO" wordmark).
 * Loaded at runtime in app/_layout.tsx with useFonts, so it also works in Expo Go;
 * the family name is the key passed to useFonts, identical on iOS and Android.
 * It has a single weight, so never combine it with fontWeight: Android would fall
 * back to the system font, and iOS would fake a bold.
 */
export const DisplayFont = 'DelaGothicOne_400Regular';

/**
 * Body typeface: Zen Kaku Gothic New, which sets Japanese and Latin with the same
 * rhythm. Each weight is its own family (custom fonts ignore fontWeight on
 * Android), so ThemedText picks the family from the style's fontWeight.
 * Loaded with DisplayFont in app/_layout.tsx.
 */
export const BodyFont = {
  regular: 'ZenKakuGothicNew_400Regular',
  medium: 'ZenKakuGothicNew_500Medium',
  bold: 'ZenKakuGothicNew_700Bold',
} as const;

/**
 * Languages each brand face fully covers (every letter of the alphabet,
 * upper and lower case, plus common punctuation), measured with fontTools
 * on 2026-10-26. Text in any other language is set in the platform's own
 * font for the whole string: a face missing some letters falls back one
 * glyph at a time, mixing two typefaces inside a word (Polish "ł", Turkish
 * "ğ"), and our Japanese face draws the characters Chinese shares with
 * Japanese in their Japanese forms. Re-measure when a face changes.
 */
export const BodyFontLanguages: ReadonlySet<string> = new Set(
  'af bg ca cy da de en es eu fi fr ga gl id is it ja ms nb nl nn no pt ru sq sv sw tl'.split(' '),
);
export const DisplayFontLanguages: ReadonlySet<string> = new Set(
  ('af bg ca cs cy da de el en es et eu fi fr ga gl hr hu id is it ja lt lv mk ms mt nb nl nn no pl pt ro ru sk sl ' +
    'sq sr sv sw tl tr uk vi').split(' '),
);

/**
 * Scripts whose marks stack above or below the letters (Thai, Devanagari
 * and its relatives, Arabic, Vietnamese tones, Burmese, Khmer, Tibetan):
 * their lines need more height, or the marks are clipped.
 */
export const TallScriptLanguages: ReadonlySet<string> = new Set(
  'ar fa ur ps th lo km my hi mr ne bn pa gu or ta te kn ml si vi bo dz'.split(' '),
);
export const TallLineScale = 1.3;

/** The body family for a fontWeight: 600 and up is bold, 500 medium, else regular. */
export function bodyFontFor(weight: string | number | undefined): string {
  const w = Number(weight ?? 400);
  return w >= 600 ? BodyFont.bold : w >= 500 ? BodyFont.medium : BodyFont.regular;
}

/**
 * Brand colours (Kogane gold, shu sun, Ai-black ink; chosen 2026-10-26), the
 * same in both themes. The shu sun is the logo's and is never used on screens.
 * Gold is the thread: the main button, routes and the interest scale, which
 * runs goldTint (open to try), gold (like) and Colors.love (love). Ink text
 * on gold and goldTint (8:1 and 13:1).
 */
export const Brand = {
  red: '#DA4528',
  gold: '#DEA92C',
  goldTint: '#F2E2B2',
  goldDeep: '#885716',
  ink: '#131B40',
} as const;

export type ThemeColor = keyof typeof Colors.light & keyof typeof Colors.dark;

export const Fonts = Platform.select({
  ios: {
    /** iOS `UIFontDescriptorSystemDesignDefault` */
    sans: 'system-ui',
    /** iOS `UIFontDescriptorSystemDesignSerif` */
    serif: 'ui-serif',
    /** iOS `UIFontDescriptorSystemDesignRounded` */
    rounded: 'ui-rounded',
    /** iOS `UIFontDescriptorSystemDesignMonospaced` */
    mono: 'ui-monospace',
  },
  default: {
    sans: 'normal',
    serif: 'serif',
    rounded: 'normal',
    mono: 'monospace',
  },
  web: {
    sans: 'var(--font-display)',
    serif: 'var(--font-serif)',
    rounded: 'var(--font-rounded)',
    mono: 'var(--font-mono)',
  },
});

export const Spacing = {
  half: 2,
  one: 4,
  two: 8,
  three: 16,
  four: 24,
  five: 32,
  six: 64,
} as const;

/** Room under tab screens' content for the tab bar (on the web, components/app-tabs.web.tsx). */
export const BottomTabInset = Platform.select({ ios: 50, android: 80, web: 72 }) ?? 0;
export const MaxContentWidth = 800;
/**
 * Widest a form-like screen gets (sign-in, onboarding, profile editing):
 * header, content and footer share one centred column. Phones are narrower,
 * so it only matters on tablets and the web, where full-width inputs and
 * buttons looked oversized.
 */
export const FormWidth = 560;
