import { QueryClient } from '@tanstack/react-query';

import { crewLeft, eventChanged, keys, profileSaved, signupChanged } from '../queries';

const seed = (qc: QueryClient, ...queryKeys: readonly (readonly unknown[])[]) =>
  queryKeys.forEach((queryKey) => qc.setQueryData(queryKey, { seeded: true }));
const stale = (qc: QueryClient, queryKey: readonly unknown[]) => qc.getQueryState(queryKey)?.isInvalidated;

test('saving a profile refreshes the account, profile, feed and activities of any city', async () => {
  const qc = new QueryClient();
  const activities = keys.activities('osaka', 'en', 'ja');
  seed(qc, keys.me('u1'), keys.profile, keys.feed, activities, keys.events);
  await profileSaved(qc);
  for (const k of [keys.me('u1'), keys.profile, keys.feed, activities]) expect(stale(qc, k)).toBe(true);
  expect(stale(qc, keys.events)).toBe(false);
});

test('leaving a crew removes the crew and its chat room, then refreshes the list', async () => {
  const qc = new QueryClient();
  seed(qc, keys.crew('c1'), keys.room({ crewId: 'c1' }), keys.room({ crewId: 'c2' }), keys.crews);
  await crewLeft(qc, 'c1');
  expect(qc.getQueryData(keys.crew('c1'))).toBeUndefined();
  expect(qc.getQueryData(keys.room({ crewId: 'c1' }))).toBeUndefined();
  expect(qc.getQueryData(keys.room({ crewId: 'c2' }))).toBeDefined();
  expect(stale(qc, keys.crews)).toBe(true);
});

test('cancelling a place refreshes the event, the events, the feed and the plan', async () => {
  const qc = new QueryClient();
  seed(qc, keys.event('e1'), keys.events, keys.feed, keys.plan('p1'), keys.plan('p2'));
  await eventChanged(qc, 'e1', { feed: true, planId: 'p1' });
  for (const k of [keys.event('e1'), keys.events, keys.feed, keys.plan('p1')]) expect(stale(qc, k)).toBe(true);
  expect(stale(qc, keys.plan('p2'))).toBe(false);
});

test('signing up refreshes the plan and the feed', async () => {
  const qc = new QueryClient();
  seed(qc, keys.plan('p1'), keys.feed, keys.events);
  await signupChanged(qc, 'p1');
  expect(stale(qc, keys.plan('p1'))).toBe(true);
  expect(stale(qc, keys.feed)).toBe(true);
  expect(stale(qc, keys.events)).toBe(false);
});
