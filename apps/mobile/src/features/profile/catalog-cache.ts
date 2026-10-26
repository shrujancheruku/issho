import { File, Paths } from 'expo-file-system';

/** A catalogue response as stored on the device, with when it was fetched (ms). */
export type Stored<T> = { value: T; at: number };

// The catalogue (cities and areas, activities) is names that change rarely
// and that the first screens wait for. Kept on the device, the app opens
// with last time's copy straight away and checks for a newer one in the
// background (useCities, useActivities). The cache directory is fine: if the
// OS empties it, the catalogue is fetched again.
function file(key: string) {
  return new File(Paths.cache, `catalog-${key.replace(/[^a-zA-Z0-9-]/g, '_')}.json`);
}

/** Synchronous, so the first render can use it. Missing or corrupt: undefined. */
export function readStored<T>(key: string): Stored<T> | undefined {
  try {
    const f = file(key);
    if (!f.exists) return undefined;
    const parsed = JSON.parse(f.textSync()) as Stored<T>;
    return typeof parsed?.at === 'number' ? parsed : undefined;
  } catch {
    return undefined;
  }
}

export function writeStored<T>(key: string, value: T) {
  try {
    const f = file(key);
    if (!f.exists) f.create();
    f.write(JSON.stringify({ value, at: Date.now() } satisfies Stored<T>));
  } catch {
    // Only a cache.
  }
}
