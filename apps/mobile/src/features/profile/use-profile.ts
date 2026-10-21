import { useQuery } from '@tanstack/react-query';

import { api } from '@/lib/api';

export const profileKey = ['profile'] as const;

/** The caller's saved profile (settings). Cleared with the cache on sign-out. */
export function useProfile() {
  return useQuery({ queryKey: profileKey, queryFn: () => api.profile.getProfile({}) });
}
