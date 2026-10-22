import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api } from '@/lib/api';

import type { toSignup } from './signup';

export const feedKey = ['feed'] as const;
const planKey = (id: string) => ['plan', id] as const;

/** Every open plan in the person's city, best for them first. */
export function useFeed() {
  return useQuery({ queryKey: feedKey, queryFn: () => api.plans.listFeed({}), select: (r) => r.plans });
}

/** One open plan with the person's signup, if any. */
export function usePlan(id: string) {
  return useQuery({ queryKey: planKey(id), queryFn: () => api.plans.getPlan({ planId: id }), enabled: !!id });
}

/** Signs up or withdraws, then refreshes the plan and the feed (counts change). */
export function useSignupActions(id: string) {
  const queryClient = useQueryClient();
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: planKey(id) }),
      queryClient.invalidateQueries({ queryKey: feedKey }),
    ]);
  const signUp = useMutation({
    mutationFn: (signup: ReturnType<typeof toSignup>) => api.plans.signUp({ planId: id, signup }),
    onSuccess: refresh,
  });
  const withdraw = useMutation({ mutationFn: () => api.plans.withdraw({ planId: id }), onSuccess: refresh });
  return { signUp, withdraw };
}
