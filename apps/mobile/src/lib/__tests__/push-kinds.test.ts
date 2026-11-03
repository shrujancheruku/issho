// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { QueryClient } from '@tanstack/react-query';

import { goKinds } from '../test-support/go-kinds';
import { keys, pushKinds, pushReceived } from '../queries';

// The backend's notification kinds are read from the Go source, so adding a
// Notify* constant without a row in `pushKinds` fails here.

test('every kind the backend queues has a row in the push table, and the table has no stranger', () => {
  expect(goKinds.length).toBeGreaterThanOrEqual(8); // the parse found them
  expect([...goKinds].sort()).toEqual(Object.keys(pushKinds).sort());
});

const seedAll = (qc: QueryClient) => {
  for (const k of [keys.feed, keys.planCardsAll, keys.events, keys.event('e1'), keys.event('e2'), keys.crews, keys.crew('c1'), keys.crew('c2')])
    qc.setQueryData(k, 1);
};
const staleKeys = (qc: QueryClient) =>
  qc
    .getQueryCache()
    .getAll()
    .filter((q) => q.state.isInvalidated)
    .map((q) => JSON.stringify(q.queryKey))
    .sort();
const expected = (...ks: (readonly unknown[])[]) => ks.map((k) => JSON.stringify(k)).sort();

test.each([
  ['event_invited', { eventId: 'e1' }, [keys.events, keys.event('e1'), keys.feed, keys.planCardsAll]],
  ['group_confirmed', { eventId: 'e1' }, [keys.event('e1'), keys.events]],
  ['group_cancelled', { eventId: 'e1' }, [keys.event('e1'), keys.events]],
  ['event_feedback', { eventId: 'e1' }, [keys.event('e1'), keys.events]],
  ['plan_no_group', { planId: 'p1' }, [keys.feed, keys.planCardsAll, keys.events]],
  ['crew_offered', { crewId: 'c1' }, [keys.crews, keys.crew('c1')]],
  ['crew_formed', { crewId: 'c1' }, [keys.crews, keys.crew('c1')]],
  ['crew_suggestion', { crewId: 'c1' }, [keys.crew('c1')]],
])('%s refreshes only what the ticket lists', async (kind, ref, want) => {
  const qc = new QueryClient();
  seedAll(qc);
  await pushReceived(qc, kind, ref);
  expect(staleKeys(qc)).toEqual(expected(...want));
});

test('an unknown kind, a missing kind or a prototype name refreshes nothing', async () => {
  for (const kind of ['something_new', undefined, 42, 'constructor']) {
    const qc = new QueryClient();
    seedAll(qc);
    await pushReceived(qc, kind, { eventId: 'e1', crewId: 'c1' });
    expect(staleKeys(qc)).toEqual([]);
  }
});

test('a kind whose id is missing still refreshes the lists', async () => {
  const qc = new QueryClient();
  seedAll(qc);
  await pushReceived(qc, 'group_confirmed', {});
  expect(staleKeys(qc)).toEqual(expected(keys.events));
});
