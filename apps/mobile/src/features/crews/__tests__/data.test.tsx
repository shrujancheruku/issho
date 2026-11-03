// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { CrewResponse, CrewStatus, type Crew } from '@issho/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook } from '@testing-library/react-native';
import type { ReactNode } from 'react';

import { api } from '@/lib/api';
import { keys } from '@/lib/queries';

import { crewView } from '../crew-view';
import { useCrew, useMyCrews, useRespondToCrew } from '../data';

jest.mock('@/lib/api', () => ({
  api: { crews: { getCrew: jest.fn(), listMyCrews: jest.fn(), respondToCrew: jest.fn() } },
}));
const mockApi = api as unknown as { crews: Record<'getCrew' | 'listMyCrews' | 'respondToCrew', jest.Mock> };

// Fake timers, as the chat room test does: TanStack Query notifies on a timer.
const flush = async () => {
  for (let i = 0; i < 6; i++) await act(async () => jest.advanceTimersByTime(0));
};

/** The server's side of one crew, which the tests move through its states. */
let server: { status: CrewStatus; response: CrewResponse; yes: number };
const crew = (): Crew =>
  ({
    id: 'c1',
    status: server.status,
    response: server.response,
    quorum: 3,
    members: Array.from({ length: 4 }, (_, i) => ({ id: `m${i}`, accepted: i < server.yes })),
  }) as unknown as Crew;
// A lapsed offer is gone from the person's list (the admin SPA is the only one that sees lapsed).
const listed = () => (server.status === CrewStatus.LAPSED ? [] : [crew()]);

beforeEach(() => {
  jest.useFakeTimers();
  jest.clearAllMocks();
  server = { status: CrewStatus.OFFERED, response: CrewResponse.OFFERED, yes: 1 };
  mockApi.crews.getCrew.mockImplementation(async () => ({ crew: crew() }));
  mockApi.crews.listMyCrews.mockImplementation(async () => ({ crews: listed() }));
});

afterEach(() => jest.useRealTimers());

function setup() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const wrapper = ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
  return { qc, wrapper };
}

const view = (c: Crew | undefined) => crewView(c as Crew);

test('an unanswered offer needs the person; accepting it waits for the others, then the crew is active', async () => {
  const { wrapper } = setup();
  const { result } = await renderHook(() => ({ crew: useCrew('c1'), respond: useRespondToCrew('c1') }), { wrapper });
  await flush();
  expect(result.current.crew.data).toBeDefined();
  expect(view(result.current.crew.data)).toMatchObject({ isOffer: true, needsAnswer: true, listedAsGroup: false });

  // Yes: the server records the answer; the crew screen refetches via the invalidation.
  mockApi.crews.respondToCrew.mockImplementation(async () => {
    server = { ...server, response: CrewResponse.ACCEPTED, yes: 2 };
    return {};
  });
  await act(async () => {
    await result.current.respond.mutateAsync(true);
  });
  expect(mockApi.crews.respondToCrew).toHaveBeenCalledWith({ crewId: 'c1', accept: true });
  await flush();
  expect(view(result.current.crew.data)).toMatchObject({ accepted: true, needsAnswer: false, yes: 2 });
  expect(view(result.current.crew.data).isOffer).toBe(true);

  // Enough accepted (the crew_formed push refreshes the same keys): active, listed as a group.
  server = { ...server, status: CrewStatus.ACTIVE, yes: 3 };
  await act(async () => {
    await result.current.crew.refetch();
  });
  await flush();
  expect(view(result.current.crew.data)).toMatchObject({ isOffer: false, isActive: true, needsAnswer: false, listedAsGroup: true });
});

// The app has no declined state: it sends accept=false and refreshes, and the
// server stops listing the crew for that person.
test('declining sends accept=false and invalidates the crew and the list', async () => {
  const { qc, wrapper } = setup();
  const { result } = await renderHook(() => ({ crews: useMyCrews(), respond: useRespondToCrew('c1') }), { wrapper });
  await flush();
  expect(result.current.crews.data).toHaveLength(1);
  qc.setQueryData(keys.crew('c1'), { crew: crew() });

  await act(async () => {
    await result.current.respond.mutateAsync(false);
  });
  expect(mockApi.crews.respondToCrew).toHaveBeenCalledWith({ crewId: 'c1', accept: false });
  expect(qc.getQueryState(keys.crew('c1'))?.isInvalidated).toBe(true);
  expect(mockApi.crews.listMyCrews.mock.calls.length).toBeGreaterThan(1); // the list was refetched
});

test('an offer that lapses while the screen is open stops needing an answer on the next refresh', async () => {
  const { qc, wrapper } = setup();
  const { result } = await renderHook(() => useCrew('c1'), { wrapper });
  await flush();
  expect(result.current.data).toBeDefined();
  expect(view(result.current.data)).toMatchObject({ isOffer: true, needsAnswer: true });

  // Too few accepted in time, behind the open screen's back.
  server = { ...server, status: CrewStatus.LAPSED };
  await act(async () => {
    await qc.invalidateQueries({ queryKey: keys.crew('c1') });
  });
  await flush();
  expect(view(result.current.data).isOffer).toBe(false);
  expect(view(result.current.data)).toMatchObject({ isActive: false, needsAnswer: false, listedAsGroup: false });
});

test('answering a lapsed offer fails without refreshing the crew as if it had worked', async () => {
  const { qc, wrapper } = setup();
  const { result } = await renderHook(() => useRespondToCrew('c1'), { wrapper });
  qc.setQueryData(keys.crew('c1'), { crew: crew() });
  mockApi.crews.respondToCrew.mockRejectedValue(new Error('offer closed'));
  await act(async () => {
    await result.current.mutateAsync(true).catch(() => {});
  });
  await flush();
  expect(result.current.isError).toBe(true);
  expect(qc.getQueryState(keys.crew('c1'))?.isInvalidated).toBe(false);
});
