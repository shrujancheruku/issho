import type { Stored } from './catalog-cache';

export type { Stored };

// Web version of catalog-cache.ts: the same JSON, in localStorage. Only a
// cache, so failures are ignored and the catalogue is fetched again.
const storageKey = (key: string) => `catalog-${key}`;

export function readStored<T>(key: string): Stored<T> | undefined {
  try {
    const raw = localStorage.getItem(storageKey(key));
    const parsed = raw ? (JSON.parse(raw) as Stored<T>) : undefined;
    return typeof parsed?.at === 'number' ? parsed : undefined;
  } catch {
    return undefined;
  }
}

export function writeStored<T>(key: string, value: T) {
  try {
    localStorage.setItem(storageKey(key), JSON.stringify({ value, at: Date.now() } satisfies Stored<T>));
  } catch {
    // Full or blocked.
  }
}
