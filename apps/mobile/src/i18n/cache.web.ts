import type { CachedStrings } from './cache';

export type { CachedStrings };

// Web version of cache.ts (expo-file-system's File API isn't available in
// browsers): the same JSON, in localStorage. Only a cache, so failures are
// ignored and the strings are fetched again.
const key = (lang: string) => `ui-strings-${lang}`;

export function readCached(lang: string): CachedStrings | undefined {
  try {
    const raw = localStorage.getItem(key(lang));
    const parsed = raw ? (JSON.parse(raw) as CachedStrings) : undefined;
    return parsed && typeof parsed.strings === 'object' ? parsed : undefined;
  } catch {
    return undefined;
  }
}

export function writeCached(lang: string, value: CachedStrings) {
  try {
    localStorage.setItem(key(lang), JSON.stringify(value));
  } catch {
    // Full or blocked.
  }
}
