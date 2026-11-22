import { JoinLinkProblemReason } from '@issho/api-client';
import { fireEvent, screen, waitFor } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import type { ReactNode } from 'react';

import { clearPendingJoin, peekPendingJoin } from '@/join/pending-join';
import { fakeApi } from '@/test-support/fake-api';
import { joinLinkRefusal } from '@/test-support/problems';
import { fakeSessionClient, RIGHT_CODE, type FakeSessionClient } from '@/test-support/fake-session-client';
import { TestProviders } from '@/test-support/providers';

import PlansScreen from '../(tabs)/plans';
import TabsLayout from '../(tabs)/_layout';
import TogetherScreen from '../(tabs)/together';
import YouScreen from '../(tabs)/you';
import Index from '../index';
import JoinArrival from '../join/index';
import JoinWithCode from '../join/[code]';
import SignIn from '../sign-in';

jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context'),
  useSafeAreaInsets: jest.fn(() => ({ top: 0, bottom: 0, left: 0, right: 0 })),
}));

jest.mock('react-native/Libraries/Utilities/useColorScheme', () => ({
  __esModule: true,
  default: jest.fn(() => 'light'),
}));

// A join link's code is a secret in a URL: it is never written to a log.
const logs = ['log', 'info', 'warn', 'error', 'debug'] as const;
let logged: jest.SpyInstance[];
beforeEach(() => {
  clearPendingJoin();
  logged = logs.map((level) => jest.spyOn(console, level).mockImplementation(() => {}));
});
afterEach(() => logged.forEach((spy) => spy.mockRestore()));

const SECRET = 'k3y-0f-a-j01n-l1nk';

async function open(path: string, opts: { lang?: 'en' | 'vi'; session?: FakeSessionClient; api?: ReturnType<typeof fakeApi> } = {}) {
  const api = opts.api ?? fakeApi();
  const session = opts.session ?? fakeSessionClient({ accessToken: 't', email: 'ada@example.com' });
  await renderRouter(
    {
      index: Index,
      'sign-in': SignIn,
      join: JoinArrival,
      'join/[code]': JoinWithCode,
      '(tabs)/_layout': TabsLayout,
      '(tabs)/together': TogetherScreen,
      '(tabs)/plans': PlansScreen,
      '(tabs)/you': YouScreen,
    },
    {
      initialUrl: path,
      wrapper: ({ children }: { children: ReactNode }) => (
        <TestProviders lang={opts.lang ?? 'en'} api={api} session={session}>
          {children}
        </TestProviders>
      ),
    } as never,
  );
  return { api, session };
}

function loggedAnything() {
  return logged.flatMap((spy) => spy.mock.calls).some((call) => JSON.stringify(call).includes(SECRET));
}

describe('arriving by a join link', () => {
  it('admits a signed-in person, making their account first, and takes the code out of the address', async () => {
    const { api } = await open(`/join/${SECRET}`);
    expect(await screen.findByText("You're in")).toBeTruthy();
    expect(api.joinLink.redeemJoinLink).toHaveBeenCalledTimes(1);
    expect(api.joinLink.redeemJoinLink).toHaveBeenCalledWith({ code: SECRET });
    // Redeeming needs an account to exist, and asking for the account is what makes it.
    expect(api.account.getAccount.mock.invocationCallOrder[0]).toBeLessThan(api.joinLink.redeemJoinLink.mock.invocationCallOrder[0]);
    expect(screen).toHavePathname('/join');
    expect(screen).toHaveSearchParams({});
    expect(peekPendingJoin()).toBeNull();
    expect(loggedAnything()).toBe(false);
  });

  it('goes on to the plans', async () => {
    await open(`/join/${SECRET}`);
    await fireEvent.press(await screen.findByRole('button', { name: 'See plans' }));
    await waitFor(() => expect(screen).toHavePathname('/plans'));
  });

  it('signs a signed-out person in first, keeps the code out of the sign-in address, and admits them after', async () => {
    const { api } = await open(`/join/${SECRET}`, { session: fakeSessionClient() });
    await waitFor(() => expect(screen).toHavePathname('/sign-in'));
    expect(screen).toHaveSearchParams({ next: '/join' });
    expect(api.joinLink.redeemJoinLink).not.toHaveBeenCalled();

    await fireEvent.changeText(await screen.findByLabelText('Email'), 'ada@example.com');
    await fireEvent.press(screen.getByRole('button', { name: 'Send me a code' }));
    await fireEvent.changeText(await screen.findByLabelText('6-digit code'), RIGHT_CODE);
    await fireEvent.press(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByText("You're in")).toBeTruthy();
    expect(api.joinLink.redeemJoinLink).toHaveBeenCalledTimes(1);
    expect(api.joinLink.redeemJoinLink).toHaveBeenCalledWith({ code: SECRET });
    expect(screen).toHavePathname('/join');
    expect(loggedAnything()).toBe(false);
  });

  it('says plainly that a used, expired or cancelled link cannot be used, and forgets its code', async () => {
    const api = fakeApi();
    api.joinLink.redeemJoinLink.mockRejectedValue(joinLinkRefusal(JoinLinkProblemReason.LINK_UNUSABLE));
    await open(`/join/${SECRET}`, { api });
    expect(await screen.findByText('This join link has been used, has expired or was cancelled. Ask the person who sent it for a new one.')).toBeTruthy();
    expect(peekPendingJoin()).toBeNull();
    expect(screen.getByRole('button', { name: 'See plans' })).toBeTruthy();
    expect(loggedAnything()).toBe(false);
  });

  it('tells someone already in that the link was not used', async () => {
    const api = fakeApi();
    api.joinLink.redeemJoinLink.mockRejectedValue(joinLinkRefusal(JoinLinkProblemReason.ALREADY_ADMITTED));
    await open(`/join/${SECRET}`, { api });
    expect(await screen.findByText("You're already in, so this link was not used.")).toBeTruthy();
  });

  it('lets the person try again when the link could not be checked, keeping the code for it', async () => {
    const api = fakeApi();
    api.joinLink.redeemJoinLink.mockRejectedValueOnce(new Error('network'));
    await open(`/join/${SECRET}`, { api });
    expect(await screen.findByText('We could not use the join link. Check your connection and try again.')).toBeTruthy();
    await fireEvent.press(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByText("You're in")).toBeTruthy();
    expect(api.joinLink.redeemJoinLink).toHaveBeenCalledTimes(2);
    expect(api.joinLink.redeemJoinLink).toHaveBeenLastCalledWith({ code: SECRET });
  });

  it('sends a visit with no link to the plans', async () => {
    const { api } = await open('/join');
    await waitFor(() => expect(screen).toHavePathname('/plans'));
    expect(api.joinLink.redeemJoinLink).not.toHaveBeenCalled();
  });

  it('reads in Vietnamese', async () => {
    await open(`/join/${SECRET}`, { lang: 'vi' });
    expect(await screen.findByText('Bạn đã vào')).toBeTruthy();
  });
});
