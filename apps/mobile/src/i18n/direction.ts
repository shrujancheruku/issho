import * as Updates from 'expo-updates';
import { DevSettings, I18nManager, Platform } from 'react-native';

import { baseLanguage } from './strings';

/** Languages written right to left (ISO 639-1 codes the app accepts). */
const RightToLeftLanguages: ReadonlySet<string> = new Set(['ar', 'dv', 'fa', 'he', 'ps', 'sd', 'ug', 'ur', 'yi']);

export function isRightToLeft(lang: string): boolean {
  return RightToLeftLanguages.has(baseLanguage(lang));
}

/**
 * Lays the app out in the primary display language's direction. On the web
 * it switches at once (the page's dir; styles use start/end). On phones
 * React Native reads the direction at launch, so the app reloads: a dev
 * build through DevSettings, a release build through expo-updates (where
 * updates aren't enabled, as in Expo Go, it switches at the next start).
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
  else if (Updates.isEnabled) void Updates.reloadAsync();
}
