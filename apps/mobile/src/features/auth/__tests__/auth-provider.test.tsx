// TypeScript 6 no longer auto-includes @types packages; scope Jest globals to this file.
/// <reference types="jest" />
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, waitFor } from '@testing-library/react-native';

import { AuthProvider } from '@/features/auth/auth-provider';
import { api } from '@/lib/api';
import { supabase } from '@/lib/supabase';

// The real auth provider runs; only the API client, push and Supabase are faked.
jest.mock('@/lib/api', () => ({ api: { identity: { getMe: jest.fn() } } }));
jest.mock('@/lib/push', () => ({ unregisterPush: jest.fn() }));
jest.mock('@/lib/supabase', () => ({
  supabase: { auth: { getSession: jest.fn(), onAuthStateChange: jest.fn(() => ({ data: { subscription: { unsubscribe: jest.fn() } } })), signOut: jest.fn(), updateUser: jest.fn() } },
}));

const mockApi = api as unknown as { identity: { getMe: jest.Mock } };
const mockSupabase = supabase as unknown as { auth: { getSession: jest.Mock; onAuthStateChange: jest.Mock } };

const sessionOf = (id: string) => ({ user: { id, user_metadata: {} } });
const emit = (event: string, session: unknown) => {
  const listener = mockSupabase.auth.onAuthStateChange.mock.calls.at(-1)[0];
  act(() => listener(event, session));
};

test('switching user clears the cache; a token refresh for the same user does not', async () => {
  mockApi.identity.getMe.mockResolvedValue({ user: { onboarded: false } });
  mockSupabase.auth.getSession.mockResolvedValue({ data: { session: sessionOf('user-a') } });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const clear = jest.spyOn(client, 'clear');
  render(
    <QueryClientProvider client={client}>
      <AuthProvider>{null}</AuthProvider>
    </QueryClientProvider>,
  );
  await waitFor(() => expect(mockApi.identity.getMe).toHaveBeenCalled());
  clear.mockClear();

  emit('TOKEN_REFRESHED', sessionOf('user-a'));
  expect(clear).not.toHaveBeenCalled();

  emit('SIGNED_IN', sessionOf('user-b'));
  expect(clear).toHaveBeenCalledTimes(1);

  emit('TOKEN_REFRESHED', sessionOf('user-b'));
  expect(clear).toHaveBeenCalledTimes(1);

  emit('SIGNED_OUT', null);
  expect(clear).toHaveBeenCalledTimes(2);
});
