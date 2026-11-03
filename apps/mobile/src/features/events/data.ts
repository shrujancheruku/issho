import { EventAction as Action, type GetEventResponse, type ReportReason } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient, type QueryClient } from '@tanstack/react-query';

import { api } from '@/lib/api';
import { eventChanged, keys } from '@/lib/queries';

import type { EventAction } from './outing';

/** The person's events from yesterday on, soonest first. */
export function useMyEvents() {
  return useQuery({ queryKey: keys.events, queryFn: () => api.events.listMyEvents({}), select: (r) => r.events });
}

/** One event with its stops and, once confirmed, who is going. */
export function useEvent(id: string) {
  return useQuery({ queryKey: keys.event(id), queryFn: () => api.events.getEvent({ eventId: id }), enabled: !!id });
}

const actions: Record<Exclude<EventAction, 'checkIn' | 'feedback'>, Action> = {
  confirm: Action.CONFIRM,
  decline: Action.DECLINE,
  cancel: Action.CANCEL,
};

/** The plan an event is for, from the cached event (undefined when it was never opened). */
const planIdOf = (queryClient: QueryClient, id: string) => queryClient.getQueryData<GetEventResponse>(keys.event(id))?.event?.planId;

/**
 * Answers an invitation, then refreshes the event, the list of events and
 * the feed and the plan (a cancelled group puts people back on a plan's
 * waiting list).
 * `refreshAfter` (ms) succeeds at once and refreshes later, so a button can
 * show its result before the refreshed list moves the item away.
 */
export function useRespond(id: string, { refreshAfter = 0 }: { refreshAfter?: number } = {}) {
  const queryClient = useQueryClient();
  const refresh = () => eventChanged(queryClient, id, { feed: true, planId: planIdOf(queryClient, id) });
  return useMutation({
    // Checking in is its own call; the rest are answers.
    mutationFn: async (action: Exclude<EventAction, 'feedback'>) => {
      if (action === 'checkIn') await api.events.checkIn({ eventId: id });
      else await api.events.respondToEvent({ eventId: id, action: actions[action] });
    },
    onSuccess: () => {
      if (!refreshAfter) return refresh();
      setTimeout(() => void refresh(), refreshAfter);
    },
  });
}

/** The person's feedback form for an event: their group and earlier answer. */
export function useFeedback(id: string) {
  return useQuery({ queryKey: keys.feedback(id), queryFn: () => api.events.getFeedback({ eventId: id }), enabled: !!id });
}

export type FeedbackAnswer = { attended: boolean; planRating: number; groupRating: number; picks: string[]; avoid: string[] };

/** Sends (or replaces) the person's feedback, then refreshes their events. */
export function useSubmitFeedback(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (answer: FeedbackAnswer) => api.events.submitFeedback({ eventId: id, ...answer }),
    onSuccess: (res) => {
      queryClient.setQueryData(keys.feedback(id), res.feedback);
      return eventChanged(queryClient, id);
    },
  });
}

type ReportAnswer = { mateId: string; reason: ReportReason; details: string };

/** Reports someone from the person's group to the moderators. */
export function useReport(id: string) {
  return useMutation({ mutationFn: (r: ReportAnswer) => api.events.reportPerson({ eventId: id, ...r }) });
}
