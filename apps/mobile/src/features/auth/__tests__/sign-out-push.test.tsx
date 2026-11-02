// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { Code, ConnectError } from '@issho/api-client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, waitFor } from '@testing-library/react-native';
import { useEffect } from 'react';

import { AuthProvider, useAuth } from '@/features/auth/auth-provider';
import { api } from '@/lib/api';
import { registerForPush, unregisterPush } from '@/lib/push';
import { supabase } from '@/lib/supabase';

// The real push.ts and auth provider run; only the edges are faked: the
// native push modules, the API client and Supabase.
jest.mock('expo-constants', () => ({
  __esModule: true,
  default: { expoConfig: { extra: { eas: { projectId: 'project' } } }, executionEnvironment: 'bare' },
  ExecutionEnvironment: { StoreClient: 'storeClient' },
}));
jest.mock('expo-device', () => ({ isDevice: true }));
jest.mock('expo-notifications', () => ({
  setNotificationHandler: jest.fn(),
  setNotificationChannelAsync: jest.fn(),
  getPermissionsAsync: jest.fn(async () => ({ granted: true, canAskAgain: true })),
  requestPermissionsAsync: jest.fn(async () => ({ granted: true })),
  getExpoPushTokenAsync: jest.fn(async () => ({ data: 'ExponentPushToken[device]' })),
  useLastNotificationResponse: jest.fn(),
  clearLastNotificationResponseAsync: jest.fn(),
  AndroidImportance: { HIGH: 4 },
}));

// The factories build their fakes inside, because jest.mock runs before
// this file's own constants exist; the tests reach them by importing.
jest.mock('@/lib/api', () => ({
  api: {
    identity: { getMe: jest.fn() },
    push: { registerPushToken: jest.fn(), unregisterPushToken: jest.fn() },
  },
}));
jest.mock('@/lib/supabase', () => {
  let listener: (event: string, session: unknown) => void = () => {};
  return {
    supabase: {
      auth: {
        getSession: jest.fn(),
        onAuthStateChange: jest.fn((cb: typeof listener) => {
          listener = cb;
          return { data: { subscription: { unsubscribe: jest.fn() } } };
        }),
        signOut: jest.fn(async () => listener('SIGNED_OUT', null)),
        updateUser: jest.fn(),
      },
    },
  };
});

const mockApi = api as unknown as {
  identity: { getMe: jest.Mock };
  push: { registerPushToken: jest.Mock; unregisterPushToken: jest.Mock };
};
const mockSupabase = supabase as unknown as { auth: { getSession: jest.Mock; signOut: jest.Mock } };

const sessionOf = (id: string) => ({ user: { id, user_metadata: {} } });

let signOut: () => Promise<void>;
function Probe() {
  const auth = useAuth();
  useEffect(() => {
    signOut = auth.signOut;
  });
  return null;
}

function renderProvider() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  return render(
    <QueryClientProvider client={client}>
      <AuthProvider>
        <Probe />
      </AuthProvider>
    </QueryClientProvider>,
  );
}

beforeEach(async () => {
  await unregisterPush(); // module state from the previous test
  jest.clearAllMocks();
  mockApi.identity.getMe.mockResolvedValue({ user: { onboarded: false } });
});

test('after the API rejects the session, the next account on the phone is registered', async () => {
  // A is signed in and registered; then the API says A's session is dead.
  mockSupabase.auth.getSession.mockResolvedValue({ data: { session: sessionOf('user-a') } });
  mockApi.identity.getMe.mockRejectedValue(new ConnectError('gone', Code.Unauthenticated));
  expect(await registerForPush({ ask: false, userId: 'user-a' })).toBe(true);
  expect(mockApi.push.registerPushToken).toHaveBeenCalledTimes(1);

  renderProvider();
  // The forced sign-out released the token before signing out of Supabase.
  await waitFor(() => expect(mockSupabase.auth.signOut).toHaveBeenCalledTimes(1));
  expect(mockApi.push.unregisterPushToken).toHaveBeenCalledWith({ token: 'ExponentPushToken[device]' });

  // B signs in on the same phone: the same token is registered again.
  expect(await registerForPush({ ask: false, userId: 'user-b' })).toBe(true);
  expect(mockApi.push.registerPushToken).toHaveBeenCalledTimes(2);
});

test('registering again for the same account does nothing, for another account registers', async () => {
  expect(await registerForPush({ ask: false, userId: 'user-a' })).toBe(true);
  expect(await registerForPush({ ask: false, userId: 'user-a' })).toBe(true);
  expect(mockApi.push.registerPushToken).toHaveBeenCalledTimes(1);
  // Even if sign-out never ran (e.g. the app was killed), another account re-registers.
  expect(await registerForPush({ ask: false, userId: 'user-b' })).toBe(true);
  expect(mockApi.push.registerPushToken).toHaveBeenCalledTimes(2);
});

test('signOut releases the push token before signing out of Supabase', async () => {
  mockSupabase.auth.getSession.mockResolvedValue({ data: { session: sessionOf('user-a') } });
  await registerForPush({ ask: false, userId: 'user-a' });
  renderProvider();
  await waitFor(() => expect(signOut).toBeDefined());
  await signOut();
  const unregister = mockApi.push.unregisterPushToken.mock.invocationCallOrder[0];
  const supabaseSignOut = mockSupabase.auth.signOut.mock.invocationCallOrder[0];
  expect(unregister).toBeLessThan(supabaseSignOut);
});

test('signOut still signs out when releasing the token fails', async () => {
  mockSupabase.auth.getSession.mockResolvedValue({ data: { session: sessionOf('user-a') } });
  await registerForPush({ ask: false, userId: 'user-a' });
  mockApi.push.unregisterPushToken.mockRejectedValue(new Error('offline'));
  renderProvider();
  await waitFor(() => expect(signOut).toBeDefined());
  await signOut();
  expect(mockSupabase.auth.signOut).toHaveBeenCalledTimes(1);
});
