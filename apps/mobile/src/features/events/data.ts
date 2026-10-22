import { EventAction as Action } from '@issho/api-client';
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

const actions: Record<EventAction, Action> = {
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
    mutationFn: (action: EventAction) => api.events.respondToEvent({ eventId: id, action: actions[action] }),
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: eventKey(id) }),
        queryClient.invalidateQueries({ queryKey: eventsKey }),
        queryClient.invalidateQueries({ queryKey: feedKey }),
      ]),
  });
}
