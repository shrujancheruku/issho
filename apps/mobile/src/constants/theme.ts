/**
 * Below are the colors that are used in the app. The colors are defined in the light and dark mode.
 * There are many other ways to style your app. For example, [Nativewind](https://www.nativewind.dev/), [Tamagui](https://tamagui.dev/), [unistyles](https://unistyl.es/), etc.
 */

import '@/global.css';

import { Platform } from 'react-native';

export const Colors = {
  light: {
    text: '#000000',
    background: '#ffffff',
    backgroundElement: '#F0F0F3',
    backgroundSelected: '#E0E1E6',
    textSecondary: '#60646C',
  },
  dark: {
    text: '#ffffff',
    background: '#000000',
    backgroundElement: '#212225',
    backgroundSelected: '#2E3135',
    textSecondary: '#B0B4BA',
  },
} as const;

/**
 * Display typeface for standout and hero text (the same face as the "ISSHO" wordmark).
 * Loaded at runtime in app/_layout.tsx with useFonts, so it also works in Expo Go;
 * the family name is the key passed to useFonts, identical on iOS and Android.
 * It has a single weight, so never combine it with fontWeight: Android would fall
 * back to the system font, and iOS would fake a bold.
 */
export const DisplayFont = 'DelaGothicOne_400Regular';

/** Brand colours from assets/brand (see its README). */
export const Brand = {
  red: '#BC002D',
  gold: '#C9A227',
  ink: '#111111',
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

export const BottomTabInset = Platform.select({ ios: 50, android: 80 }) ?? 0;
/**
 * On the web the tab bar floats over the top of the screen
 * (components/app-tabs.web.tsx), so tab screens start below it.
 */
export const TopTabInset = Platform.select({ web: 64 }) ?? 0;
export const MaxContentWidth = 800;
/**
 * Widest a form-like screen gets (sign-in, onboarding, profile editing):
 * header, content and footer share one centred column. Phones are narrower,
 * so it only matters on tablets and the web, where full-width inputs and
 * buttons looked oversized.
 */
export const FormWidth = 560;
