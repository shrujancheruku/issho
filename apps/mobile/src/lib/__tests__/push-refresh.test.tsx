// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react-native';

import { usePushRefresh } from '../push';
import { keys } from '../queries';

jest.mock('expo-constants', () => ({
  __esModule: true,
  default: { executionEnvironment: 'bare' },
  ExecutionEnvironment: { StoreClient: 'storeClient' },
}));
jest.mock('expo-device', () => ({ isDevice: true }));
jest.mock('@/lib/api', () => ({ api: {} }));
jest.mock('expo-notifications', () => ({
  setNotificationHandler: jest.fn(),
  addNotificationReceivedListener: jest.fn(),
  useLastNotificationResponse: jest.fn(),
  clearLastNotificationResponseAsync: jest.fn(),
  AndroidImportance: { HIGH: 4 },
}));

// eslint-disable-next-line @typescript-eslint/no-require-imports
const Notifications = require('expo-notifications') as { addNotificationReceivedListener: jest.Mock };

function Probe() {
  usePushRefresh();
  return null;
}

test('a received group_confirmed refreshes that event and the events list, and the listener is removed on unmount', async () => {
  let deliver: (n: unknown) => void = () => {};
  const remove = jest.fn();
  Notifications.addNotificationReceivedListener.mockImplementation((cb: (n: unknown) => void) => {
    deliver = cb;
    return { remove };
  });
  const qc = new QueryClient();
  qc.setQueryData(keys.event('e1'), 1);
  qc.setQueryData(keys.event('e2'), 1);
  qc.setQueryData(keys.events, 1);
  const view = await render(
    <QueryClientProvider client={qc}>
      <Probe />
    </QueryClientProvider>,
  );

  deliver({ request: { content: { data: { kind: 'group_confirmed', eventId: 'e1' } } } });
  expect(qc.getQueryState(keys.event('e1'))?.isInvalidated).toBe(true);
  expect(qc.getQueryState(keys.events)?.isInvalidated).toBe(true);
  expect(qc.getQueryState(keys.event('e2'))?.isInvalidated).toBe(false);

  await view.unmount();
  expect(remove).toHaveBeenCalled();
});
