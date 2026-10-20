// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { secureStorage } from '@/lib/secure-storage';

// In-memory stand-in for the OS keychain. ("mock" prefix lets jest.mock reference it.)
const mockStore = new Map<string, string>();
jest.mock('expo-secure-store', () => ({
  getItemAsync: jest.fn(async (k: string) => mockStore.get(k) ?? null),
  setItemAsync: jest.fn(async (k: string, v: string) => void mockStore.set(k, v)),
  deleteItemAsync: jest.fn(async (k: string) => void mockStore.delete(k)),
}));

const KEY = 'sb-127-auth-token'; // Supabase-style key; ':' etc. would be sanitised
const session = (size: number) => JSON.stringify({ access_token: 'x'.repeat(size), user: { email: 'a@example.com' } });

beforeEach(() => mockStore.clear());

test('round-trips a session larger than one chunk', async () => {
  const value = session(5000);
  await secureStorage.setItem(KEY, value);
  expect(Number(mockStore.get(`${KEY}.n`)!.split(':')[1])).toBeGreaterThan(1);
  for (const [k, v] of mockStore) if (k !== `${KEY}.n`) expect(v.split(':')[1].length).toBeLessThanOrEqual(1800);
  expect(await secureStorage.getItem(KEY)).toBe(value);
});

test('shrinking a value removes leftover chunks', async () => {
  await secureStorage.setItem(KEY, session(5000));
  await secureStorage.setItem(KEY, session(10));
  expect(mockStore.get(`${KEY}.n`)).toMatch(/:1$/);
  expect([...mockStore.keys()].sort()).toEqual([`${KEY}.0`, `${KEY}.n`]);
});

test('removeItem deletes every chunk', async () => {
  await secureStorage.setItem(KEY, session(5000));
  await secureStorage.removeItem(KEY);
  expect(mockStore.size).toBe(0);
  expect(await secureStorage.getItem(KEY)).toBeNull();
});

test('a missing chunk reads as no session', async () => {
  await secureStorage.setItem(KEY, session(5000));
  mockStore.delete(`${KEY}.1`);
  expect(await secureStorage.getItem(KEY)).toBeNull();
});

test('a torn write (chunk from another write) is discarded, even if it looks valid', async () => {
  await secureStorage.setItem(KEY, session(5000));
  // A chunk from a different (interrupted) write: same length, still valid JSON once joined.
  const stale = mockStore.get(`${KEY}.1`)!.replace(/^[^:]+:/, 'othergen:');
  mockStore.set(`${KEY}.1`, stale);
  expect(await secureStorage.getItem(KEY)).toBeNull();
  expect(mockStore.size).toBe(0);
});

test('keys are sanitised to SecureStore-safe characters', async () => {
  await secureStorage.setItem('sb:auth/token', '{}');
  expect(mockStore.has('sb_auth_token.n')).toBe(true);
});
