import type { LeaveReason, ReportReason } from '@issho/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { api } from '@/lib/api';
import { crewLeft, crewResponded, keys } from '@/lib/queries';

/** The person's crews and open crew offers, newest first. */
export function useMyCrews() {
  return useQuery({ queryKey: keys.crews, queryFn: () => api.crews.listMyCrews({}), select: (r) => r.crews });
}

export function useCrew(id: string) {
  return useQuery({ queryKey: keys.crew(id), queryFn: () => api.crews.getCrew({ crewId: id }), enabled: !!id, select: (r) => r.crew });
}

/** Accepts or declines a crew offer, then refreshes the crews. */
export function useRespondToCrew(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (accept: boolean) => api.crews.respondToCrew({ crewId: id, accept }),
    onSuccess: () => crewResponded(queryClient, id),
  });
}

/** Leaves a crew with why, then drops it and its chat room and refreshes the crews. */
export function useLeaveCrew(id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (r: { reason: LeaveReason; details: string }) => api.crews.leaveCrew({ crewId: id, ...r }),
    onSuccess: () => crewLeft(queryClient, id),
  });
}

/** Reports someone in the crew to the moderators. */
export function useReportCrewMember(id: string) {
  return useMutation({
    mutationFn: (r: { mateId: string; reason: ReportReason; details: string }) =>
      api.crews.reportCrewMember({ crewId: id, memberId: r.mateId, reason: r.reason, details: r.details }),
  });
}
