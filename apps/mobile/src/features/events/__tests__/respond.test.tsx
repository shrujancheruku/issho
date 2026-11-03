/// <reference types="jest" />
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, waitFor } from '@testing-library/react-native';
import { useEffect } from 'react';

import { useRespond } from '@/features/events/data';
import { keys } from '@/lib/queries';

jest.mock('@/lib/api', () => ({ api: { events: { respondToEvent: jest.fn(async () => ({})), checkIn: jest.fn() } } }));

let respond: ReturnType<typeof useRespond>;
function Probe() {
  const r = useRespond('e1');
  useEffect(() => {
    respond = r;
  });
  return null;
}

test('cancelling a place also refreshes the plan the event is for', async () => {
  const qc = new QueryClient();
  qc.setQueryData(keys.event('e1'), { event: { id: 'e1', planId: 'p1' } });
  qc.setQueryData(keys.plan('p1'), {});
  qc.setQueryData(keys.feed, {});
  await render(
    <QueryClientProvider client={qc}>
      <Probe />
    </QueryClientProvider>,
  );
  await act(async () => {
    await respond.mutateAsync('cancel');
  });
  await waitFor(() => expect(qc.getQueryState(keys.plan('p1'))?.isInvalidated).toBe(true));
  expect(qc.getQueryState(keys.feed)?.isInvalidated).toBe(true);
});
