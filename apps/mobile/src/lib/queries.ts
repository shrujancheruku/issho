import type { QueryClient } from '@tanstack/react-query';

/**
 * Every React Query key in the app, and what each kind of change refreshes.
 * Screens and `data.ts` files never write a key array or list the keys a
 * mutation touches: they call `keys.*` and the named invalidators below, so
 * "what does this change affect" is answered in one place.
 */

/** Whose chat: an event's group chat (the caller's group) or a crew's. */
export type RoomRef = { eventId: string } | { crewId: string };

export const roomId = (room: RoomRef) => ('eventId' in room ? `event:${room.eventId}` : `crew:${room.crewId}`);

export const keys = {
  health: ['health'] as const,
  feed: ['feed'] as const,
  plan: (id: string) => ['plan', id] as const,
  /** The feed cards of a batch of plans (GetPlanCards; ids sorted, at most `PlanCardsMax`). */
  planCards: (ids: readonly string[]) => ['planCards', ...ids] as const,
  /** Prefix of every `planCards(...)`. */
  planCardsAll: ['planCards'] as const,
  events: ['events'] as const,
  event: (id: string) => ['event', id] as const,
  feedback: (id: string) => ['feedback', id] as const,
  crews: ['crews'] as const,
  crew: (id: string) => ['crew', id] as const,
  room: (room: RoomRef) => ['chat', roomId(room)] as const,
  profile: ['profile'] as const,
  /** The account (GetMe) of one user; the user id is in the key so a switch never shows the last user's. */
  me: (userId: string | undefined) => ['me', userId] as const,
  /** Prefix of every `me(userId)`. */
  meAll: ['me'] as const,
  uiStrings: (lang: string) => ['ui-strings', lang] as const,
  cities: (primary: string, secondary: string) => ['cities', primary, secondary] as const,
  /** Prefix of every `activities(...)`. */
  activitiesAll: ['activities'] as const,
  activities: (cityId: string, primary: string, secondary: string) => ['activities', cityId, primary, secondary] as const,
  languages: (primary: string, display: boolean) => ['languages', primary, display] as const,
};

const refresh = (qc: QueryClient, ...queryKeys: readonly (readonly unknown[])[]) =>
  Promise.all(queryKeys.map((queryKey) => qc.invalidateQueries({ queryKey })));

/**
 * The profile was saved. GetMe carries the languages and home city; the feed
 * and activities are per city and language, so a new city must show on Plans.
 */
export const profileSaved = (qc: QueryClient) =>
  refresh(qc, keys.meAll, keys.profile, keys.feed, keys.planCardsAll, keys.activitiesAll);

/** The API said the account is suspended: ask GetMe again so the app shows it. */
export const accountChanged = (qc: QueryClient) => refresh(qc, keys.meAll);

/** The person signed up for a plan or withdrew: its counts and the feed change. */
export const signupChanged = (qc: QueryClient, planId: string) => refresh(qc, keys.plan(planId), keys.feed, keys.planCardsAll);

/**
 * An event changed (answered, cancelled, checked in, feedback sent). With
 * `planId`, the plan too: a cancelled group puts people back on its waiting
 * list. `feed` is for changes that move the feed (not feedback).
 */
export const eventChanged = (qc: QueryClient, id: string, opts: { planId?: string; feed?: boolean } = {}) =>
  refresh(
    qc,
    keys.event(id),
    keys.events,
    ...(opts.feed ? [keys.feed, keys.planCardsAll] : []),
    ...(opts.planId ? [keys.plan(opts.planId)] : []),
  );

/** A crew offer was answered. */
export const crewResponded = (qc: QueryClient, id: string) => refresh(qc, keys.crew(id), keys.crews);

/**
 * The person left a crew. The crew and its chat room are gone for them, so
 * they are removed (a refetch would only fail), then the list is refreshed.
 */
export function crewLeft(qc: QueryClient, id: string) {
  qc.removeQueries({ queryKey: keys.crew(id) });
  qc.removeQueries({ queryKey: keys.room({ crewId: id }) });
  return refresh(qc, keys.crews);
}

/** The ids a push carries in its data (`notifications.go` `data()`); each is set only when the kind has one. */
export type PushRef = { eventId?: string; planId?: string; crewId?: string };

/**
 * What each push kind makes stale, keyed by the backend's notification kind.
 * One place for both received pushes and taps. A test
 * (`__tests__/push-kinds.test.ts`) reads the Go `Notify*` constants and fails
 * when a kind has no row here; the Go files point back at this table.
 */
export const pushKinds: Record<string, (ref: PushRef) => readonly (readonly unknown[])[]> = {
  event_invited: ({ eventId }) => [keys.events, ...(eventId ? [keys.event(eventId)] : []), keys.feed],
  group_confirmed: ({ eventId }) => [...(eventId ? [keys.event(eventId)] : []), keys.events],
  group_cancelled: ({ eventId }) => [...(eventId ? [keys.event(eventId)] : []), keys.events],
  event_feedback: ({ eventId }) => [...(eventId ? [keys.event(eventId)] : []), keys.events],
  plan_no_group: () => [keys.feed, keys.events],
  crew_offered: ({ crewId }) => [keys.crews, ...(crewId ? [keys.crew(crewId)] : [])],
  crew_formed: ({ crewId }) => [keys.crews, ...(crewId ? [keys.crew(crewId)] : [])],
  // An open room gets the message over its socket; only the crew's own data changes.
  crew_suggestion: ({ crewId }) => (crewId ? [keys.crew(crewId)] : []),
};

/**
 * A push arrived (or was tapped): refresh what its kind changed. An unknown
 * kind (a newer server than this app) refreshes nothing.
 */
export function pushReceived(qc: QueryClient, kind: unknown, ref: PushRef = {}) {
  const affected = typeof kind === 'string' && Object.hasOwn(pushKinds, kind) ? pushKinds[kind] : undefined;
  return refresh(qc, ...(affected?.(ref) ?? []));
}
