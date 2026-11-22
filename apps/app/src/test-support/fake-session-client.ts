import type { Session, SessionClient } from '@/auth/session-client';

/** The code the fake accepts; any other code is a wrong one. */
export const RIGHT_CODE = '123456';

/**
 * A stand-in for the session client (src/auth/session-client.ts): sending a code
 * does nothing, and `verifyCode` signs in when it is given RIGHT_CODE. No test
 * reaches Supabase Auth.
 */
export function fakeSessionClient(initial: Session | null = null) {
  let session = initial;
  const listeners = new Set<(s: Session | null) => void>();
  const set = (next: Session | null) => {
    session = next;
    listeners.forEach((listener) => listener(next));
  };
  return {
    getSession: jest.fn(async () => session),
    onChange: jest.fn((listener: (s: Session | null) => void) => {
      listeners.add(listener);
      return () => listeners.delete(listener);
    }),
    sendCode: jest.fn(async (_email: string) => {}),
    verifyCode: jest.fn(async (email: string, code: string) => {
      if (code !== RIGHT_CODE) return false;
      set({ accessToken: 'token', email });
      return true;
    }),
    signOut: jest.fn(async () => set(null)),
  } satisfies SessionClient;
}

export type FakeSessionClient = ReturnType<typeof fakeSessionClient>;
