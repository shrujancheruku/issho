import { getLocales } from 'expo-localization';
import { useSyncExternalStore } from 'react';

import { pickLanguage, type Language } from './strings';

const noopSubscribe = () => () => {};
const device = (): Language => pickLanguage(getLocales().map((locale) => locale.languageTag));
const server = (): Language => 'en';

/**
 * The display language chosen from the device's or browser's languages. The
 * Expo server renders public pages without knowing the visitor (it holds
 * nothing personal, so pages can be cached), so it says English, and so does
 * the browser's first pass over that HTML; the device's language follows right
 * after. Reading it straight away makes the first client render differ from the
 * server's, and React throws the server's HTML away. A person's own choice in
 * You comes with that screen.
 */
export function useDeviceLanguage(): Language {
  return useSyncExternalStore(noopSubscribe, device, server);
}
