import type { Signup } from '@issho/api-client';
import { useCallback, useEffect, useState } from 'react';

import { useApi } from '@/api/api-provider';
import { useAuth } from '@/auth/auth-provider';

/**
 * The signed-in person's own signups (ListMySignups). It loads on the client
 * only and only for a signed-in person: a plan page is server-rendered and
 * publicly cached, so nothing per visitor can be in it (docs/spec/architecture.md).
 * `signedOut` and `loading` (the stored session not yet read) call nothing.
 * `reload` asks again after a change.
 */
export type MySignups = { status: 'idle' } | { status: 'loading' } | { status: 'failed' } | { status: 'ready'; signups: Signup[] };

export function useMySignups() {
  const api = useApi();
  const auth = useAuth();
  const [read, setRead] = useState<MySignups | null>(null);
  const signedIn = auth.status === 'signedIn';

  const reload = useCallback(async () => {
    try {
      const { signups } = await api.signup.listMySignups({});
      setRead({ status: 'ready', signups });
    } catch {
      setRead({ status: 'failed' });
    }
  }, [api]);

  useEffect(() => {
    if (!signedIn) return;
    let live = true;
    api.signup.listMySignups({}).then(
      ({ signups }) => live && setRead({ status: 'ready', signups }),
      () => live && setRead({ status: 'failed' }),
    );
    return () => {
      live = false;
    };
  }, [signedIn, api]);

  // Nothing is kept for a person who is not signed in (or has signed out since).
  const state: MySignups = !signedIn ? { status: 'idle' } : (read ?? { status: 'loading' });
  return { state, reload };
}

/** Something is in play when the person has a signup running. Outings (#619) add to this. */
export const inPlay = (signups: Signup[]) => signups.length > 0;
