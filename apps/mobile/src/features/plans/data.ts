import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';

import type { FeedPlan, GetPlanCardsResponse, GetPlanResponse, ListFeedResponse } from '@issho/api-client';

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

/** GetPlanCards takes at most this many ids (the proto says so); more go in further calls. */
export const PlanCardsMax = 50;

/**
 * The feed cards of some plans (title, place, signup deadline, whether the
 * person signed up) from one GetPlanCards call per `PlanCardsMax` ids,
 * instead of a GetPlan (which waits on Google for the venues) per plan. The
 * feed's copy of a plan stands in while it loads. Plans the person can't see
 * are missing from the map. Callers pass the same ids to share one call.
 */
export function usePlanCards(ids: readonly string[]): ReadonlyMap<string, FeedPlan> {
  const queryClient = useQueryClient();
  const sorted = [...new Set(ids)].sort();
  const chunks: string[][] = [];
  for (let i = 0; i < sorted.length; i += PlanCardsMax) chunks.push(sorted.slice(i, i + PlanCardsMax));
  const results = useQueries({
    queries: chunks.map((chunk) => ({
      queryKey: keys.planCards(chunk),
      queryFn: () => api.plans.getPlanCards({ planIds: chunk }),
      placeholderData: () => {
        const feed = queryClient.getQueryData<ListFeedResponse>(keys.feed)?.plans ?? [];
        return { $typeName: 'issho.v1.GetPlanCardsResponse', plans: feed.filter((p) => chunk.includes(p.id)) } as GetPlanCardsResponse;
      },
    })),
  });
  const cards = new Map<string, FeedPlan>();
  for (const r of results) for (const p of r.data?.plans ?? []) cards.set(p.id, p);
  return cards;
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
