import { createAppSessionClient } from '../app-session-client';

// The phone's storage is native code; the client is never given a Supabase address here, so it is never read.
jest.mock('../session-storage', () => ({ sessionStorage: undefined }));

// A build given no Supabase address (EXPO_PUBLIC_SUPABASE_URL) must still start:
// the person is signed out, and asking for a code fails the way a network error does.
describe('the app session client without a Supabase address', () => {
  const saved = { ...process.env };
  beforeEach(() => {
    delete process.env.EXPO_PUBLIC_SUPABASE_URL;
    delete process.env.EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY;
  });
  afterEach(() => {
    process.env = { ...saved };
  });

  it('reads as signed out instead of throwing', async () => {
    const client = createAppSessionClient();
    expect(() => client.onChange(() => {})()).not.toThrow();
    await expect(client.getSession()).resolves.toBeNull();
  });

  it('rejects sending and checking a code, as a failed call, so the screen can say so', async () => {
    const client = createAppSessionClient();
    await expect(client.sendCode('ada@example.com')).rejects.toThrow();
    await expect(client.verifyCode('ada@example.com', '123456')).rejects.toThrow();
  });

  it('signs out without throwing', async () => {
    await expect(createAppSessionClient().signOut()).resolves.toBeUndefined();
  });
});
