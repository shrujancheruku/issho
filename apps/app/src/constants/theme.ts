/**
 * The app's colours in light and dark, from the Issho design system, ported
 * from apps/mobile at v1-final. The ground and accent are "Ai" (indigo, the dye
 * of thread). Text pairs are at least 4.5:1 on background, surface and
 * backgroundElement. A screen takes its colours from useTheme(), never a
 * literal. Fonts, the spacing scale's users and the rest of v1's theme come
 * back with the screens that need them.
 */

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
    /**
     * White washes over the accent (the crew header and You's appearance
     * switch sit on it): a faint bar, the selected segment, the unfilled
     * stretch of a track. The same in both themes: the accent is dark in each.
     */
    onAccentWash: 'rgba(255,255,255,0.14)',
    onAccentWashStrong: 'rgba(255,255,255,0.22)',
    onAccentTrack: 'rgba(255,255,255,0.28)',
    /** The small menu that drops from a header. Dark gets the surface, so link text stays readable. */
    menuSurface: 'rgba(255,255,255,0.92)',
    /** Google's required attribution on a map: grey on its pale tiles, white on the dark ones. */
    mapAttribution: '#5E5E5E',
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
    onAccentWash: 'rgba(255,255,255,0.14)',
    onAccentWashStrong: 'rgba(255,255,255,0.22)',
    onAccentTrack: 'rgba(255,255,255,0.28)',
    menuSurface: 'rgba(22,30,64,0.96)',
    mapAttribution: '#FFFFFF',
  },
} as const;

export type Palette = (typeof Colors)['light' | 'dark'];

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
  /** Map pins' ring and the text on a grey pin. */
  white: '#FFFFFF',
  /** The soft glow under the walker on a drawn route. */
  goldHalo: 'rgba(222,169,44,0.3)',
} as const;

/**
 * The text sizes. Screens take a size from here rather than writing a number.
 */
export const FontSize = {
  caption: 12,
  small: 14,
  default: 16,
  heading: 22,
  subtitle: 26,
  title: 44,
} as const;

export type ThemeColor = keyof typeof Colors.light & keyof typeof Colors.dark;

export const Spacing = {
  half: 2,
  one: 4,
  two: 8,
  three: 16,
  four: 24,
  five: 32,
  six: 64,
} as const;

/** Widest a screen's content gets, so a phone layout does not stretch across a desktop browser. */
export const MaxContentWidth = 800;
