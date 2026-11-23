import type { Tick } from '@issho/api-client';
import { useCallback, useState } from 'react';

import { useApi } from '@/api/api-provider';
import { ErrorText } from '@/components/form';
import { useI18n } from '@/i18n/i18n-provider';

import { isNotAdmitted } from './problems';

/** Why a signup was not made, as far as the screen needs to tell apart. */
export type Refusal = 'notAdmitted' | 'failed' | null;

/**
 * The entry point of a signup (the signup screen is #616). In the Pilot a join
 * link is the only way in, and a person who has signed in without one can read
 * plans and is refused here: `refusal` is `notAdmitted` for that, and the
 * screen shows it with `<SignUpRefusal>`. `signUp` resolves true when the
 * signup was made.
 */
export function useSignUp() {
  const api = useApi();
  const [refusal, setRefusal] = useState<Refusal>(null);
  const signUp = useCallback(
    async (planId: string, ticks: Pick<Tick, 'date' | 'part'>[]) => {
      setRefusal(null);
      try {
        await api.signup.signUp({ planId, ticks });
        return true;
      } catch (err) {
        setRefusal(isNotAdmitted(err) ? 'notAdmitted' : 'failed');
        return false;
      }
    },
    [api],
  );
  return { signUp, refusal };
}

/** Says why a signup was refused; nothing when it was not. */
export function SignUpRefusal({ refusal }: { refusal: Refusal }) {
  const { t } = useI18n();
  if (!refusal) return null;
  return <ErrorText>{t(refusal === 'notAdmitted' ? 'signUp.needJoinLink' : 'signUp.failed')}</ErrorText>;
}
