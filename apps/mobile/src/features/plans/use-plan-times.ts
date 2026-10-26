import { useMemo } from 'react';

import { useProfile } from '@/features/profile/use-profile';
import { slotKey } from '@/lib/slots';

import { usePlan } from './data';
import { initialTimes } from './signup';

/**
 * A plan with the person's times for it: their saved signup, or their
 * usual week (profile) for someone not signed up yet.
 */
export function usePlanTimes(id: string) {
  const plan = usePlan(id);
  const profile = useProfile();
  const usual = useMemo(
    () => (profile.data?.profile?.availability ?? []).map((s) => slotKey(s.weekday, s.block)),
    [profile.data],
  );
  const saved = useMemo(
    () => (plan.data && profile.data ? initialTimes(plan.data.signup, usual) : null),
    [plan.data, profile.data, usual],
  );
  return { plan, profile, usual, saved };
}
