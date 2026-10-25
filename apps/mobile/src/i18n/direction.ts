import { DevSettings, I18nManager, Platform } from 'react-native';

import { baseLanguage } from './strings';

/** Languages written right to left (ISO 639-1 codes the app accepts). */
export const RightToLeftLanguages: ReadonlySet<string> = new Set(['ar', 'dv', 'fa', 'he', 'ps', 'sd', 'ug', 'ur', 'yi']);

export function isRightToLeft(lang: string): boolean {
  return RightToLeftLanguages.has(baseLanguage(lang));
}

/**
 * Lays the app out in the primary display language's direction. On the web
 * it switches at once (the page's dir; styles use start/end). On phones
 * React Native reads the direction at launch: a dev build reloads now, and a
 * release build switches at its next start (reloading it needs
 * expo-updates, which the app doesn't use yet).
 */
export function applyDirection(lang: string) {
  const rtl = isRightToLeft(lang);
  if (Platform.OS === 'web') {
    if (typeof document !== 'undefined') {
      document.documentElement.dir = rtl ? 'rtl' : 'ltr';
      document.documentElement.lang = lang;
    }
    return;
  }
  if (I18nManager.isRTL === rtl) return;
  I18nManager.allowRTL(rtl);
  I18nManager.forceRTL(rtl);
  if (__DEV__) DevSettings.reload();
}
