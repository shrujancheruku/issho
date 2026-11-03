import type { Session } from '@supabase/supabase-js';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createContext, useCallback, useContext, useEffect, useRef, useState, type PropsWithChildren } from 'react';

import { Code, ConnectError } from '@issho/api-client';

import { api } from '@/lib/api';
import { unregisterPush } from '@/lib/push';
import { keys } from '@/lib/queries';
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
  /** The home city, from GetMe; "" until onboarded (or from an API before it sent one). */
  cityId: string;
  /** Set when the account couldn't be loaded (e.g. API unreachable). */
  accountError: Error | null;
  retryAccount: () => void;
  signOut: () => Promise<void>;
};

/** How long signing out waits for the push token to be released. */
const UnregisterTimeoutMs = 3000;

const AuthContext = createContext<AuthState | null>(null);

export function AuthProvider({ children }: PropsWithChildren) {
  const queryClient = useQueryClient();
  const [ready, setReady] = useState(false);
  const [session, setSession] = useState<Session | null>(null);

  // The user the cache belongs to. A ref, so the listener can compare without
  // a side effect inside a setState updater (which React may run twice).
  const cacheUser = useRef<string | undefined>(undefined);
  const heard = useRef(false);

  useEffect(() => {
    supabase.auth.getSession().then(({ data }) => {
      // A listener event may have run first; it knows the newer user.
      if (!heard.current) cacheUser.current = data.session?.user.id;
      setSession(data.session);
      setReady(true);
    });
    // Fires on sign-in, sign-out and token refresh. Cached server data belongs to
    // the previous user, so clear it whenever the user changes.
    const { data } = supabase.auth.onAuthStateChange((_event, next) => {
      heard.current = true;
      if (cacheUser.current !== next?.user.id) queryClient.clear();
      cacheUser.current = next?.user.id;
      setSession(next);
    });
    return () => data.subscription.unsubscribe();
  }, [queryClient]);

  // GetMe also creates the app account on first sign-in.
  const me = useQuery({
    queryKey: keys.me(session?.user.id),
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

  // The only way out of a session: every sign-out (the button, a deleted
  // account, a session the API rejects) goes through here, so the push token
  // is always released first. Best effort: it can't keep anyone signed in.
  const signOut = useCallback(async () => {
    // While still signed in: the API needs the session to know whose token it is.
    let timer: ReturnType<typeof setTimeout> | undefined;
    await Promise.race([unregisterPush(), new Promise((resolve) => (timer = setTimeout(resolve, UnregisterTimeoutMs)))]);
    clearTimeout(timer);
    await supabase.auth.signOut();
  }, []);

  // A session the API rejects (e.g. the account was deleted on another device)
  // is useless; drop it so the user lands back on sign-in. The unregister
  // call fails then (the API rejects the session), which is fine: the next
  // account to register the token takes it over server-side.
  useEffect(() => {
    if (isUnauthenticated(me.error)) void signOut();
  }, [me.error, signOut]);

  const value: AuthState = {
    ready,
    session,
    onboarded: me.data?.user?.onboarded ?? null,
    suspended: me.data?.user?.suspended ?? false,
    cityId: me.data?.user?.cityId ?? '',
    languages: me.data?.user?.onboarded
      ? { primary: me.data.user.primaryLanguage, secondary: me.data.user.secondaryLanguage }
      : null,
    accountError: me.error,
    retryAccount: () => void me.refetch(),
    signOut,
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
