import { EventAction as Action, type ReportReason } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { feedKey } from '@/features/plans/data';
import { api } from '@/lib/api';

import type { EventAction } from './state';

export const eventsKey = ['events'] as const;
const eventKey = (id: string) => ['event', id] as const;

/** The person's events from yesterday on, soonest first. */
export function useMyEvents() {
  return useQuery({ queryKey: eventsKey, queryFn: () => api.events.listMyEvents({}), select: (r) => r.events });
}

/** One event with its stops and, once confirmed, who is going. */
export function useEvent(id: string) {
  return useQuery({ queryKey: eventKey(id), queryFn: () => api.events.getEvent({ eventId: id }), enabled: !!id });
}

const actions: Record<Exclude<EventAction, 'checkIn' | 'feedback'>, Action> = {
  confirm: Action.CONFIRM,
  decline: Action.DECLINE,
  cancel: Action.CANCEL,
};

/**
 * Answers an invitation, then refreshes the event, the list of events and
 * the feed (a cancelled group puts people back on a plan's waiting list).
 */
export function useRespond(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    // Checking in is its own call; the rest are answers.
    mutationFn: async (action: Exclude<EventAction, 'feedback'>) => {
      if (action === 'checkIn') await api.events.checkIn({ eventId: id });
      else await api.events.respondToEvent({ eventId: id, action: actions[action] });
    },
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: eventKey(id) }),
        queryClient.invalidateQueries({ queryKey: eventsKey }),
        queryClient.invalidateQueries({ queryKey: feedKey }),
      ]),
  });
}

const feedbackKey = (id: string) => ['feedback', id] as const;

/** The person's feedback form for an event: their group and earlier answer. */
export function useFeedback(id: string) {
  return useQuery({ queryKey: feedbackKey(id), queryFn: () => api.events.getFeedback({ eventId: id }), enabled: !!id });
}

export type FeedbackAnswer = { attended: boolean; planRating: number; groupRating: number; picks: string[]; avoid: string[] };

/** Sends (or replaces) the person's feedback, then refreshes their events. */
export function useSubmitFeedback(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (answer: FeedbackAnswer) => api.events.submitFeedback({ eventId: id, ...answer }),
    onSuccess: (res) => {
      queryClient.setQueryData(feedbackKey(id), res.feedback);
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: eventKey(id) }),
        queryClient.invalidateQueries({ queryKey: eventsKey }),
      ]);
    },
  });
}

export type ReportAnswer = { mateId: string; reason: ReportReason; details: string };

/** Reports someone from the person's group to the moderators. */
export function useReport(id: string) {
  return useMutation({ mutationFn: (r: ReportAnswer) => api.events.reportPerson({ eventId: id, ...r }) });
}
