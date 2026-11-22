import { createEncryptedStorage } from '../encrypted-storage';

// The key store and the bulk store are stand-ins for expo-secure-store and AsyncStorage.
function memory() {
  const data = new Map<string, string>();
  return {
    data,
    getItem: async (k: string) => data.get(k) ?? null,
    setItem: async (k: string, v: string) => void data.set(k, v),
    removeItem: async (k: string) => void data.delete(k),
  };
}

function setup() {
  const keys = memory();
  const bulk = memory();
  let n = 0;
  const randomBytes = (size: number) => Uint8Array.from({ length: size }, () => (n += 7) % 256);
  return { keys, bulk, storage: createEncryptedStorage({ keys, bulk, randomBytes }) };
}

describe('the encrypted session storage', () => {
  it('gives back what was stored', async () => {
    const { storage } = setup();
    await storage.setItem('session', '{"refresh_token":"secret-refresh"}');
    expect(await storage.getItem('session')).toBe('{"refresh_token":"secret-refresh"}');
  });

  it('keeps no readable session in the bulk store, and only the key in the key store', async () => {
    const { storage, keys, bulk } = setup();
    await storage.setItem('session', '{"refresh_token":"secret-refresh"}');
    expect([...bulk.data.values()].join()).not.toContain('secret-refresh');
    expect([...keys.data.values()].join()).not.toContain('secret-refresh');
    // The key is small enough for SecureStore's 2 KB limit however long the session is.
    await storage.setItem('session', 'x'.repeat(10_000));
    expect([...keys.data.values()].every((v) => v.length < 2048)).toBe(true);
    expect(await storage.getItem('session')).toBe('x'.repeat(10_000));
  });

  it('forgets the session and its key together', async () => {
    const { storage, keys, bulk } = setup();
    await storage.setItem('session', 'value');
    await storage.removeItem('session');
    expect(await storage.getItem('session')).toBeNull();
    expect(keys.data.size).toBe(0);
    expect(bulk.data.size).toBe(0);
  });

  it('reads as signed out when the key is gone (a restore onto a new phone)', async () => {
    const { storage, keys } = setup();
    await storage.setItem('session', 'value');
    keys.data.clear();
    expect(await storage.getItem('session')).toBeNull();
  });

  it('reads nothing when it has stored nothing', async () => {
    expect(await setup().storage.getItem('session')).toBeNull();
  });
});
