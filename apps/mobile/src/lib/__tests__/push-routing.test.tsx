// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react-native';

import { notificationTarget, type NotificationTarget } from '../notification-target';
import { useNotificationTaps } from '../push';
import { goKinds } from '../test-support/go-kinds';
import { pushKinds } from '../queries';

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
  clearLastNotificationResponseAsync: jest.fn(() => Promise.resolve()),
  AndroidImportance: { HIGH: 4 },
}));

// eslint-disable-next-line @typescript-eslint/no-require-imports
const Notifications = require('expo-notifications') as {
  useLastNotificationResponse: jest.Mock;
  clearLastNotificationResponseAsync: jest.Mock;
};

// Which id each kind carries (backend `data()`), hence which screen a tap opens.
// Routing is by the id key present, not by kind: the kind only picks the row here.
const routes: Record<string, { data: Record<string, string>; target: NotificationTarget }> = {
  event_invited: { data: { eventId: 'e1' }, target: { eventId: 'e1' } },
  group_confirmed: { data: { eventId: 'e1' }, target: { eventId: 'e1' } },
  group_cancelled: { data: { eventId: 'e1' }, target: { eventId: 'e1' } },
  event_feedback: { data: { eventId: 'e1' }, target: { eventId: 'e1' } },
  plan_no_group: { data: { planId: 'p1' }, target: { planId: 'p1' } },
  crew_offered: { data: { crewId: 'c1' }, target: { crewId: 'c1' } },
  crew_formed: { data: { crewId: 'c1' }, target: { crewId: 'c1' } },
  crew_suggestion: { data: { crewId: 'c1' }, target: { crewId: 'c1' } },
};

test('every kind the backend sends has a route here and in the refresh table, so a new kind without one fails', () => {
  expect(goKinds.length).toBeGreaterThanOrEqual(8);
  expect([...goKinds].sort()).toEqual(Object.keys(routes).sort());
  expect(Object.keys(pushKinds).sort()).toEqual(Object.keys(routes).sort());
});

test.each(Object.entries(routes))('%s opens its screen', (kind, { data, target }) => {
  expect(notificationTarget({ kind, ...data })).toEqual(target);
});

test.each([
  ['no data', undefined],
  ['null data', null],
  ['an empty payload', {}],
  ['a kind without an id', { kind: 'group_confirmed' }],
  ['ids that are not strings', { eventId: 7, planId: null, crewId: {} }],
  ['a non-string kind', { kind: 42, other: 'x' }],
])('%s routes nowhere and does not throw', (_name, data) => {
  expect(notificationTarget(data as Record<string, unknown> | undefined)).toBeNull();
});

function Probe({ open }: { open: (t: NotificationTarget) => void }) {
  useNotificationTaps(open);
  return null;
}
const mount = (open: jest.Mock) =>
  render(
    <QueryClientProvider client={new QueryClient()}>
      <Probe open={open} />
    </QueryClientProvider>,
  );
const tapped = (data: Record<string, unknown> | undefined) => ({ notification: { request: { content: { data } } } });

beforeEach(() => jest.clearAllMocks());

test('a tap that launched the app opens its screen once the signed-in tree mounts, and is cleared', async () => {
  Notifications.useLastNotificationResponse.mockReturnValue(tapped({ kind: 'crew_formed', crewId: 'c1' }));
  const open = jest.fn();
  await mount(open);
  expect(open).toHaveBeenCalledTimes(1);
  expect(open).toHaveBeenCalledWith({ crewId: 'c1' });
  expect(Notifications.clearLastNotificationResponseAsync).toHaveBeenCalledTimes(1);
});

test('no tap yet opens nothing', async () => {
  Notifications.useLastNotificationResponse.mockReturnValue(undefined);
  const open = jest.fn();
  await mount(open);
  expect(open).not.toHaveBeenCalled();
});

test('a tap with an unknown kind and no ids, or a missing payload, opens nothing and does not throw', async () => {
  for (const data of [{ kind: 'something_new' }, undefined]) {
    Notifications.useLastNotificationResponse.mockReturnValue(tapped(data));
    const open = jest.fn();
    await mount(open);
    expect(open).not.toHaveBeenCalled();
  }
  expect(Notifications.clearLastNotificationResponseAsync).not.toHaveBeenCalled();
});
