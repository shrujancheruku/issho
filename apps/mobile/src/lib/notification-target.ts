/** What a tapped notification is about: an event, a plan (one that closed) or a crew. Shared by push.ts and push.web.ts. */
export type NotificationTarget = { eventId: string } | { planId: string } | { crewId: string };
