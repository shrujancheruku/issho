import type { Session } from '@supabase/supabase-js';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createContext, useContext, useEffect, useState, type PropsWithChildren } from 'react';

import { Code, ConnectError } from '@issho/api-client';

import { api } from '@/lib/api';
import { unregisterPush } from '@/lib/push';
import { supabase } from '@/lib/supabase';

type AuthState = {
  /** False until the stored session has been read from SecureStore. */
  ready: boolean;
  session: Session | null;
  /** Null while the account is loading (or when signed out). */
  onboarded: boolean | null;
  /** A moderator suspended the account: the app shows only that. */
  suspended: boolean;
  /** Display languages from the profile; null until onboarded. */
  languages: { primary: string; secondary: string } | null;
  /** Set when the account couldn't be loaded (e.g. API unreachable). */
  accountError: Error | null;
  retryAccount: () => void;
  signOut: () => Promise<void>;
};

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: PropsWithChildren) {
  const queryClient = useQueryClient();
  const [ready, setReady] = useState(false);
  const [session, setSession] = useState<Session | null>(null);

  useEffect(() => {
    supabase.auth.getSession().then(({ data }) => {
      setSession(data.session);
      setReady(true);
    });
    // Fires on sign-in, sign-out and token refresh. Cached server data belongs to
    // the previous user, so clear it whenever the user changes.
    const { data } = supabase.auth.onAuthStateChange((_event, next) => {
      setSession((prev) => {
        if (prev?.user.id !== next?.user.id) queryClient.clear();
        return next;
      });
    });
    return () => data.subscription.unsubscribe();
  }, [queryClient]);

  // GetMe also creates the app account on first sign-in.
  const me = useQuery({
    queryKey: ['me', session?.user.id],
    queryFn: () => api.identity.getMe({}),
    enabled: !!session,
    retry: (count, err) => !isUnauthenticated(err) && count < 2,
  });

  // Sign-in emails are written in user_metadata.lang (supabase/templates).
  // Keep it equal to the profile's main language, so the next code arrives
  // in the language the person chose. Users may update their own metadata;
  // a failure only means the next email uses the previous language.
  const emailLang = me.data?.user?.onboarded ? me.data.user.primaryLanguage : '';
  const storedLang = session?.user.user_metadata?.lang;
  useEffect(() => {
    if (emailLang && storedLang !== emailLang) void supabase.auth.updateUser({ data: { lang: emailLang } });
  }, [emailLang, storedLang]);

  // A session the API rejects (e.g. the account was deleted on another device)
  // is useless; drop it so the user lands back on sign-in.
  useEffect(() => {
    if (isUnauthenticated(me.error)) void supabase.auth.signOut();
  }, [me.error]);

  const value: AuthState = {
    ready,
    session,
    onboarded: me.data?.user?.onboarded ?? null,
    suspended: me.data?.user?.suspended ?? false,
    languages: me.data?.user?.onboarded
      ? { primary: me.data.user.primaryLanguage, secondary: me.data.user.secondaryLanguage }
      : null,
    accountError: me.error,
    retryAccount: () => void me.refetch(),
    signOut: async () => {
      // While still signed in: the API needs the session to know whose token it is.
      await unregisterPush();
      await supabase.auth.signOut();
    },
  };
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used inside <AuthProvider>');
  return ctx;
}

/**
 * The API's answer to every call from a suspended account (except GetMe):
 * when one arrives mid-session, ask GetMe again so the app shows it.
 */
export function isSuspendedError(err: unknown) {
  return err instanceof ConnectError && err.code === Code.PermissionDenied && err.rawMessage === 'account suspended';
}

function isUnauthenticated(err: unknown) {
  return err instanceof ConnectError && err.code === Code.Unauthenticated;
}
