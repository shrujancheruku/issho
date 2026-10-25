import { bundled } from './bundled';
import type { CachedStrings } from './cache';

/**
 * The copy of lang's UI text bundled into the app (`make ui-bundle`), for a
 * fresh install: the sign-in screen is in the right language straight away,
 * even offline, instead of English until the first fetch. It counts as
 * fetched long ago for no particular build (no appCatalog), so the app
 * fetches the current text at once and keeps that.
 */
export function fromBundle(lang: string): CachedStrings | undefined {
  const b = bundled[lang];
  if (!b) return undefined;
  return { strings: b.strings, complete: true, catalogVersion: b.catalogVersion, checkedAt: 0 };
}
