import { createClient, type SupabaseClientOptions } from '@supabase/supabase-js';

import type { Session, SessionClient } from './session-client';

type Storage = NonNullable<NonNullable<SupabaseClientOptions<'public'>['auth']>['storage']>;

/**
 * The session client over Supabase Auth, used for sign-in only: the Go API
 * takes the token and does everything else (CLAUDE.md, "Supabase"). `url` and
 * `key` are the project's address and publishable key, both public by design.
 * `storage` is where the session is kept so it survives a restart: the phone's
 * storage on native, and none on the web, where Supabase uses the browser's.
 */
export function createSupabaseSessionClient(options: { url: string; key: string; storage?: Storage; fetch?: typeof globalThis.fetch }): SessionClient {
  const supabase = createClient(options.url, options.key, {
    auth: {
      storage: options.storage,
      persistSession: true,
      autoRefreshToken: true,
      // The email code is typed in; there is no link to read a session from.
      detectSessionInUrl: false,
    },
    global: { fetch: options.fetch },
  });

  const toSession = (s: { access_token: string; user: { email?: string } } | null): Session | null =>
    s ? { accessToken: s.access_token, email: s.user.email ?? '' } : null;

  return {
    async getSession() {
      const { data, error } = await supabase.auth.getSession();
      if (error) throw error;
      return toSession(data.session);
    },
    onChange(listener) {
      const { data } = supabase.auth.onAuthStateChange((_event, session) => listener(toSession(session)));
      return () => data.subscription.unsubscribe();
    },
    async sendCode(email) {
      // A first-time email makes an account when its code is entered (to join is to make one).
      const { error } = await supabase.auth.signInWithOtp({
        email,
        options: { shouldCreateUser: true },
      });
      if (error) throw error;
    },
    async verifyCode(email, code) {
      const { error } = await supabase.auth.verifyOtp({
        email,
        token: code,
        type: 'email',
      });
      if (!error) return true;
      // A wrong or expired code is the person's to retry; anything else (network, server) is not.
      if (error.status === 400 || error.status === 401 || error.status === 403 || error.status === 422) return false;
      throw error;
    },
    async signOut() {
      // Local scope: end this device's session even when Supabase cannot be reached.
      await supabase.auth.signOut({ scope: 'local' });
    },
  };
}
