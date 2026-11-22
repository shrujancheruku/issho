import { useRouter } from 'expo-router';
import { useCallback } from 'react';

import { useApi } from '@/api/api-provider';

/**
 * The entry point to the profile's questions, for a signup to call first
 * (docs/spec/people.md: the profile is asked at the first signup, not before).
 *
 * `gate(returnTo)` resolves true when the person has a profile, and the signup
 * goes on. With none it opens the questions and resolves false; finishing them
 * returns the person to `returnTo`, the signup they started, to press sign up
 * again. A failed lookup throws, so the signup shows its own error.
 */
export function useProfileGate() {
  const api = useApi();
  const router = useRouter();
  return useCallback(
    async (returnTo: string): Promise<boolean> => {
      const { profile } = await api.profile.getProfile({});
      if (profile) return true;
      router.push({ pathname: '/profile', params: { next: returnTo } });
      return false;
    },
    [api, router],
  );
}
