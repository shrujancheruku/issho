import { fireEvent, render, screen, waitFor } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import type { ReactNode } from 'react';

import { ApiProvider } from '@/api/api-provider';
import { createAppApi } from '@/api/app-api';
import { AuthProvider } from '@/auth/auth-provider';
import { I18nProvider } from '@/i18n/i18n-provider';
import { fakeApi } from '@/test-support/fake-api';
import { fakeSessionClient } from '@/test-support/fake-session-client';
import { TestProviders } from '@/test-support/providers';

import PlansScreen from '../(tabs)/plans';
import TabsLayout from '../(tabs)/_layout';
import TogetherScreen from '../(tabs)/together';
import YouScreen from '../(tabs)/you';
import Index from '../index';
import SignIn from '../sign-in';

jest.mock('react-native/Libraries/Utilities/useColorScheme', () => ({
  __esModule: true,
  default: jest.fn(() => 'light'),
}));

describe('the You tab, signed in', () => {
  it("shows the account's email, as the API reports it", async () => {
    const api = fakeApi();
    api.account.getAccount.mockResolvedValue({
      account: {
        id: 'u1',
        email: 'grace@example.com',
        city: 'da-nang-hoi-an',
        displayLanguage: 'en',
      },
    });
    await render(
      <TestProviders lang="en" api={api}>
        <YouScreen />
      </TestProviders>,
    );
    expect(await screen.findByText('Signed in as grace@example.com')).toBeTruthy();
  });

  it('asks for the account in the language the person is reading in, and shows it in Vietnamese', async () => {
    const api = fakeApi();
    await render(
      <TestProviders lang="vi" api={api}>
        <YouScreen />
      </TestProviders>,
    );
    expect(await screen.findByText('Đã đăng nhập là ada@example.com')).toBeTruthy();
    expect(api.account.getAccount).toHaveBeenCalledWith({
      displayLanguage: 'vi',
    });
    expect(screen.getByRole('button', { name: 'Đăng xuất' })).toBeTruthy();
  });

  it('says so when the account could not be loaded', async () => {
    const api = fakeApi();
    api.account.getAccount.mockRejectedValue(new Error('network'));
    await render(
      <TestProviders lang="en" api={api}>
        <YouScreen />
      </TestProviders>,
    );
    expect(await screen.findByText('Your account could not be loaded.')).toBeTruthy();
    // Signing out still works when the account does not load.
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeTruthy();
  });

  it('signs the person out and leaves them on Plans, not at the sign-in screen', async () => {
    const session = fakeSessionClient({
      accessToken: 't',
      email: 'ada@example.com',
    });
    await renderRouter(
      {
        index: Index,
        'sign-in': SignIn,
        '(tabs)/_layout': TabsLayout,
        '(tabs)/together': TogetherScreen,
        '(tabs)/plans': PlansScreen,
        '(tabs)/you': YouScreen,
      },
      {
        initialUrl: '/you',
        wrapper: ({ children }: { children: ReactNode }) => (
          <TestProviders lang="en" api={fakeApi()} session={session}>
            {children}
          </TestProviders>
        ),
      } as never,
    );
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign out' }));
    await waitFor(() => expect(session.signOut).toHaveBeenCalled());
    await waitFor(() => expect(screen).toHavePathname('/plans'));
    // Opening You again now asks for sign-in.
    expect(await screen.findByText('No plans yet.')).toBeTruthy();
  });
});

describe('a token the API no longer accepts', () => {
  it('sends the person from You to sign-in, and back to You once they sign in again', async () => {
    const session = fakeSessionClient({ accessToken: 'stale', email: 'ada@example.com' });
    // The real client over a fetch that answers as the API does for a bad token.
    const fetch = jest.fn(
      async () =>
        new Response(JSON.stringify({ code: 'unauthenticated', message: 'invalid token' }), {
          status: 401,
          headers: { 'content-type': 'application/json' },
        }),
    );
    const api = createAppApi(session, { fetch: fetch as never });
    await renderRouter(
      {
        index: Index,
        'sign-in': SignIn,
        '(tabs)/_layout': TabsLayout,
        '(tabs)/together': TogetherScreen,
        '(tabs)/plans': PlansScreen,
        '(tabs)/you': YouScreen,
      },
      {
        initialUrl: '/you',
        wrapper: ({ children }: { children: ReactNode }) => (
          <ApiProvider api={api}>
            <AuthProvider client={session}>
              <I18nProvider lang="en">{children}</I18nProvider>
            </AuthProvider>
          </ApiProvider>
        ),
      } as never,
    );
    await waitFor(() => expect(screen).toHavePathname('/sign-in'));
    expect(screen).toHaveSearchParams({ next: '/you' });
    expect(session.signOut).toHaveBeenCalled();
    expect(screen.queryByText('Your account could not be loaded.')).toBeNull();
  });
});
