/** What a tapped notification is about: an event, a plan (one that closed) or a crew. Shared by push.ts and push.web.ts. */
export type NotificationTarget = { eventId: string } | { planId: string } | { crewId: string };

/**
 * The screen a notification's data points at, or null when it names none
 * (no data, a malformed payload, ids that aren't strings). Event, then plan,
 * then crew, as the backend sets only one id per kind. Never throws.
 */
export function notificationTarget(data: Record<string, unknown> | null | undefined): NotificationTarget | null {
  if (typeof data?.eventId === 'string') return { eventId: data.eventId };
  if (typeof data?.planId === 'string') return { planId: data.planId };
  if (typeof data?.crewId === 'string') return { crewId: data.crewId };
  return null;
}
