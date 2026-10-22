import { File, Paths } from 'expo-file-system';

import type { Strings } from './strings';

/** A translated catalogue as stored on the device. */
export type CachedStrings = {
  strings: Strings;
  /** False when the server was still translating some strings. */
  complete: boolean;
  catalogVersion: string;
  /** catalogFingerprint of the app build that fetched it; missing in older caches. */
  appCatalog?: string;
};

// Documents, not the cache directory: the sign-in screen should be in the
// right language even offline, and the OS may empty the cache directory.
function file(lang: string) {
  return new File(Paths.document, `ui-strings-${lang}.json`);
}

/**
 * Synchronous so the first render is already in the right language. A
 * missing or corrupt file just means fetching again.
 */
export function readCached(lang: string): CachedStrings | undefined {
  try {
    const f = file(lang);
    if (!f.exists) return undefined;
    const parsed = JSON.parse(f.textSync()) as CachedStrings;
    return parsed && typeof parsed.strings === 'object' ? parsed : undefined;
  } catch {
    return undefined;
  }
}

export function writeCached(lang: string, value: CachedStrings) {
  try {
    const f = file(lang);
    if (!f.exists) f.create();
    f.write(JSON.stringify(value));
  } catch {
    // Only a cache: the next launch fetches again.
  }
}
