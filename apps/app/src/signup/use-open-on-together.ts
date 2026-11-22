import { useRouter } from 'expo-router';
import { useEffect } from 'react';

import { useAuth } from '@/auth/auth-provider';

import { inPlay, useMySignups } from './use-my-signups';

/**
 * "What opens first" (docs/spec/app-shape.md): Together when the person has
 * something in play, else Plans. A visitor has no account, so asks nothing. Waits
 * for the stored session and for the signups: a failed read leaves the person on
 * Plans rather than guessing. `onPlans` runs once Plans is the answer.
 */
export function useOpenOnTogether(onPlans: () => void) {
  const auth = useAuth();
  const router = useRouter();
  const { state } = useMySignups();
  useEffect(() => {
    if (auth.status === 'signedOut' || state.status === 'failed') return onPlans();
    if (state.status === 'ready') {
      if (inPlay(state.signups)) router.replace('/together');
      else onPlans();
    }
    // `onPlans` is built fresh each render and only navigates.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [auth.status, state]);
}
