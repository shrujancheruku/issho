import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import type { GetPlanResponse, ListFeedResponse } from '@issho/api-client';

import { api } from '@/lib/api';
import { keys, signupChanged } from '@/lib/queries';

import type { toSignup } from './signup';

/** Every open plan in the person's city, best for them first. */
export function useFeed() {
  return useQuery({ queryKey: keys.feed, queryFn: () => api.plans.listFeed({}), select: (r) => r.plans });
}

/**
 * One open plan with the person's signup, if any. Opened from the feed, the
 * screen shows the feed's copy of the plan (title, place, level) at once
 * while the stops, map and summary load (GetPlan waits on Google for the
 * venues). Not for a plan the person signed up for: the feed doesn't carry
 * their times, and showing their usual week instead would be wrong.
 */
export function usePlan(id: string) {
  const queryClient = useQueryClient();
  return useQuery({
    queryKey: keys.plan(id),
    queryFn: () => api.plans.getPlan({ planId: id }),
    enabled: !!id,
    placeholderData: () => {
      const plan = queryClient.getQueryData<ListFeedResponse>(keys.feed)?.plans.find((p) => p.id === id);
      if (!plan || plan.signedUp) return undefined;
      return { $typeName: 'issho.v1.GetPlanResponse', plan, steps: [], routePolyline: '', summary: '', secondarySummary: '' } as GetPlanResponse;
    },
  });
}

/** Signs up or withdraws, then refreshes the plan and the feed (counts change). */
export function useSignupActions(id: string) {
  const queryClient = useQueryClient();
  const refresh = () => signupChanged(queryClient, id);
  const signUp = useMutation({
    mutationFn: (signup: ReturnType<typeof toSignup>) => api.plans.signUp({ planId: id, signup }),
    onSuccess: refresh,
  });
  const withdraw = useMutation({ mutationFn: () => api.plans.withdraw({ planId: id }), onSuccess: refresh });
  return { signUp, withdraw };
}
