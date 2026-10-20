import type { Session } from '@supabase/supabase-js';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { createContext, useContext, useEffect, useState, type PropsWithChildren } from 'react';

import { Code, ConnectError } from '@issho/api-client';

import { api } from '@/lib/api';
import { supabase } from '@/lib/supabase';

type AuthState = {
  /** False until the stored session has been read from SecureStore. */
  ready: boolean;
  session: Session | null;
  /** Null while the account is loading (or when signed out). */
  onboarded: boolean | null;
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

  // A session the API rejects (e.g. the account was deleted on another device)
  // is useless; drop it so the user lands back on sign-in.
  useEffect(() => {
    if (isUnauthenticated(me.error)) void supabase.auth.signOut();
  }, [me.error]);

  const value: AuthState = {
    ready,
    session,
    onboarded: me.data?.user?.onboarded ?? null,
    accountError: me.error,
    retryAccount: () => void me.refetch(),
    signOut: async () => {
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

function isUnauthenticated(err: unknown) {
  return err instanceof ConnectError && err.code === Code.Unauthenticated;
}
