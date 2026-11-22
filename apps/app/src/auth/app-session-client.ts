import type { SessionClient } from './session-client';
import { sessionStorage } from './session-storage';
import { createSupabaseSessionClient } from './supabase-session-client';

/**
 * The session client this build uses. EXPO_PUBLIC_SUPABASE_URL and
 * EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY are the project's address and its
 * publishable key: both public, bundled into the app (docs/secrets.md).
 *
 * The Supabase client is made on first use, which is in the browser or on the
 * phone and never during the web's server render: the server holds no session.
 */
export function createAppSessionClient(): SessionClient {
  let client: SessionClient | undefined;
  const real = () =>
    (client ??= createSupabaseSessionClient({
      url: process.env.EXPO_PUBLIC_SUPABASE_URL ?? '',
      key: process.env.EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY ?? '',
      storage: sessionStorage,
    }));
  // Without a Supabase address the client cannot be made. That must not crash the
  // app on start: the person is signed out, and sending or checking a code fails
  // as any failed call does, which the sign-in screen already says.
  const made = (): SessionClient | null => {
    try {
      return real();
    } catch {
      return null;
    }
  };
  return {
    getSession: async () => (await made()?.getSession()) ?? null,
    onChange: (listener) => made()?.onChange(listener) ?? (() => {}),
    sendCode: async (email) => real().sendCode(email),
    verifyCode: async (email, code) => real().verifyCode(email, code),
    signOut: async () => made()?.signOut(),
  };
}
