import * as SecureStore from 'expo-secure-store';

/**
 * Supabase session storage backed by the iOS Keychain / Android Keystore.
 *
 * Why not AsyncStorage or expo-sqlite's localStorage (as some guides suggest)?
 * Those store the refresh token in plain text in the app sandbox. A refresh token
 * is a long-lived credential, so it belongs in the OS secure store.
 *
 * Why chunking? SecureStore values should stay under ~2 KB (larger values can fail
 * on some Android devices), and a serialised Supabase session (tokens plus user
 * object) is often bigger. Values are split across `<key>.0`, `<key>.1`, …, with
 * the chunk count stored under `<key>.n`.
 *
 * Writes aren't atomic: a crash part-way through can leave chunks from two
 * different writes mixed together, and the result can still look like valid
 * JSON. So every write gets a random generation id, stored with the count
 * (`<gen>:<n>`) and prefixed to each chunk (`<gen>:<data>`). A read only succeeds
 * if every chunk carries the same generation as the count. Anything else is
 * discarded and reads as "no session", so the worst case of a torn write is
 * being signed out, never a corrupted token.
 */
const CHUNK = 1800; // characters; leaves headroom under the ~2048-byte limit for multi-byte text

// SecureStore keys may only contain alphanumerics, ".", "-" and "_".
const safe = (key: string) => key.replace(/[^A-Za-z0-9._-]/g, '_');

const newGeneration = () => `${Date.now().toString(36)}${Math.random().toString(36).slice(2, 10)}`;

async function readHeader(key: string): Promise<{ gen: string; n: number } | null> {
  const raw = await SecureStore.getItemAsync(`${key}.n`);
  const [gen, n] = raw?.split(':') ?? [];
  return gen && Number(n) > 0 ? { gen, n: Number(n) } : null;
}

async function deleteChunks(key: string, from: number, to: number) {
  await Promise.all(Array.from({ length: Math.max(0, to - from) }, (_, i) => SecureStore.deleteItemAsync(`${key}.${from + i}`)));
}

export const secureStorage = {
  async getItem(rawKey: string): Promise<string | null> {
    const key = safe(rawKey);
    const header = await readHeader(key);
    if (!header) return null;
    const chunks = await Promise.all(Array.from({ length: header.n }, (_, i) => SecureStore.getItemAsync(`${key}.${i}`)));
    const prefix = `${header.gen}:`;
    if (chunks.some((c) => c === null || !c.startsWith(prefix))) {
      // Torn or partial write: discard so the user simply signs in again. (Not `this.`:
      // the storage methods may be called detached from the object.)
      await secureStorage.removeItem(rawKey);
      return null;
    }
    return chunks.map((c) => c!.slice(prefix.length)).join('');
  },

  async setItem(rawKey: string, value: string): Promise<void> {
    const key = safe(rawKey);
    const gen = newGeneration();
    const parts = value.match(new RegExp(`[\\s\\S]{1,${CHUNK}}`, 'g')) ?? [''];
    const previous = await readHeader(key);
    await Promise.all(parts.map((p, i) => SecureStore.setItemAsync(`${key}.${i}`, `${gen}:${p}`)));
    await SecureStore.setItemAsync(`${key}.n`, `${gen}:${parts.length}`);
    // Only after the new header is in place, drop chunks a longer previous value left behind.
    if (previous) await deleteChunks(key, parts.length, previous.n);
  },

  async removeItem(rawKey: string): Promise<void> {
    const key = safe(rawKey);
    const header = await readHeader(key);
    // Without a header we can't know how many chunks exist; there's nothing readable to remove anyway.
    if (header) await deleteChunks(key, 0, header.n);
    await SecureStore.deleteItemAsync(`${key}.n`);
  },
};
