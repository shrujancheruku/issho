import aesjs from 'aes-js';

type Store = {
  getItem(key: string): Promise<string | null>;
  setItem(key: string, value: string): Promise<void>;
  removeItem(key: string): Promise<void>;
};

/**
 * Storage for the session on a phone: the session is AES-encrypted into the
 * bulk store (AsyncStorage, which Android backup can copy), and the key lives
 * in the key store (expo-secure-store, hardware-backed and kept out of backups).
 * SecureStore alone cannot hold a session: its values are limited to about 2 KB.
 * This is the pattern Supabase documents for React Native. A fresh key is made
 * on every write, so a counter is never reused under one key. When the key is
 * gone (a restore onto another phone) the session reads as absent: the person
 * signs in again.
 */
export function createEncryptedStorage(deps: { keys: Store; bulk: Store; randomBytes: (size: number) => Uint8Array }): Store {
  return {
    async getItem(name) {
      const [encrypted, keyHex] = await Promise.all([deps.bulk.getItem(name), deps.keys.getItem(name)]);
      if (!encrypted || !keyHex) return null;
      try {
        const cipher = new aesjs.ModeOfOperation.ctr(aesjs.utils.hex.toBytes(keyHex), new aesjs.Counter(1));
        return aesjs.utils.utf8.fromBytes(cipher.decrypt(aesjs.utils.hex.toBytes(encrypted)));
      } catch {
        return null;
      }
    },
    async setItem(name, value) {
      const key = deps.randomBytes(32);
      const cipher = new aesjs.ModeOfOperation.ctr(key, new aesjs.Counter(1));
      const encrypted = aesjs.utils.hex.fromBytes(cipher.encrypt(aesjs.utils.utf8.toBytes(value)));
      // Key first: a crash between the two leaves an unreadable value, which reads as signed out.
      await deps.keys.setItem(name, aesjs.utils.hex.fromBytes(key));
      await deps.bulk.setItem(name, encrypted);
    },
    async removeItem(name) {
      await Promise.all([deps.bulk.removeItem(name), deps.keys.removeItem(name)]);
    },
  };
}
