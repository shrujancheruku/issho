import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';

import type { Session, SessionClient } from './session-client';

/** `loading` until the stored session has been read: a screen must not send a signed-in person to sign-in before then. */
export type AuthState = { status: 'loading' } | { status: 'signedOut' } | { status: 'signedIn'; session: Session };

type Auth = AuthState & Pick<SessionClient, 'sendCode' | 'verifyCode' | 'signOut'> & { missingConfig: readonly string[] };

const AuthContext = createContext<Auth | null>(null);

/** Holds the person's session for everything below it. It starts `loading`, which is also what the server renders. */
export function AuthProvider({ client, children }: { client: SessionClient; children: ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: 'loading' });

  useEffect(() => {
    let live = true;
    const next = (session: Session | null): AuthState => (session ? { status: 'signedIn', session } : { status: 'signedOut' });
    const stop = client.onChange((session) => live && setState(next(session)));
    // The stored read only settles `loading`: a change that arrived meanwhile is newer and stays.
    const settle = (session: Session | null) => live && setState((prev) => (prev.status === 'loading' ? next(session) : prev));
    client.getSession().then(settle, () => settle(null));
    return () => {
      live = false;
      stop();
    };
  }, [client]);

  const value = useMemo<Auth>(
    () => ({
      ...state,
      missingConfig: client.missingConfig ?? [],
      sendCode: (email) => client.sendCode(email),
      verifyCode: (email, code) => client.verifyCode(email, code),
      signOut: () => client.signOut(),
    }),
    [client, state],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): Auth {
  const value = useContext(AuthContext);
  if (!value) throw new Error('useAuth needs an AuthProvider above it');
  return value;
}
