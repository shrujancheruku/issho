import { act, fireEvent, screen, waitFor } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import type { ReactNode } from 'react';
import { StyleSheet } from 'react-native';

import { Spacing } from '@/constants/theme';
import { fakeApi } from '@/test-support/fake-api';
import { RateLimitedError } from '@/auth/session-client';
import { fakeSessionClient, RIGHT_CODE, type FakeSessionClient } from '@/test-support/fake-session-client';
import { TestProviders } from '@/test-support/providers';

import PlansScreen from '../(tabs)/plans';
import TabsLayout from '../(tabs)/_layout';
import TogetherScreen from '../(tabs)/together';
import YouScreen from '../(tabs)/you';
import Index from '../index';
import SignIn from '../sign-in';

jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context'),
  useSafeAreaInsets: jest.fn(() => ({ top: 0, bottom: 0, left: 0, right: 0 })),
}));

jest.mock('react-native/Libraries/Utilities/useColorScheme', () => ({
  __esModule: true,
  default: jest.fn(() => 'light'),
}));

async function open(path: string, lang: 'en' | 'vi', session: FakeSessionClient = fakeSessionClient()) {
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
      initialUrl: path,
      wrapper: ({ children }: { children: ReactNode }) => (
        <TestProviders lang={lang} api={fakeApi()} session={session}>
          {children}
        </TestProviders>
      ),
    } as never,
  );
  return session;
}

const enterEmail = async (email: string, button = 'Send me a code') => {
  await fireEvent.changeText(await screen.findByLabelText('Email'), email);
  await fireEvent.press(screen.getByRole('button', { name: button }));
};
const enterCode = async (code: string) => {
  await fireEvent.changeText(await screen.findByLabelText('6-digit code'), code);
  await fireEvent.press(screen.getByRole('button', { name: 'Sign in' }));
};

describe('signing in by email code', () => {
  // The dev deploy's walk and `make e2e` open /you with no one signed in: the guard sends them here, so the version must show here too.
  it("shows the dev API's version to a visitor sent here from /you", async () => {
    await open('/you', 'en', fakeSessionClient(null));
    expect(await screen.findByText('API version test')).toBeTruthy();
    expect(screen).toHavePathname('/sign-in');
  });

  it('asks for the email, then the 6-digit code, and signs the visitor in', async () => {
    const session = await open('/sign-in', 'en');
    await enterEmail('ada@example.com');
    expect(await screen.findByText('We sent a 6-digit code to ada@example.com.')).toBeTruthy();
    expect(session.sendCode).toHaveBeenCalledWith('ada@example.com');

    await enterCode(RIGHT_CODE);
    await waitFor(() => expect(screen).toHavePathname('/plans'));
    expect(session.verifyCode).toHaveBeenCalledWith('ada@example.com', RIGHT_CODE);
  });

  it('says a wrong code is wrong and lets the visitor try again', async () => {
    const session = await open('/sign-in', 'en');
    await enterEmail('ada@example.com');
    await enterCode('000000');
    expect(await screen.findByText('That code is not right. Check it and try again.')).toBeTruthy();
    expect(screen).toHavePathname('/sign-in');

    await enterCode(RIGHT_CODE);
    await waitFor(() => expect(screen).toHavePathname('/plans'));
    expect(session.verifyCode).toHaveBeenCalledTimes(2);
  });

  it('says so when the code could not be sent, and stays on the email', async () => {
    const session = fakeSessionClient();
    session.sendCode.mockRejectedValueOnce(new Error('network'));
    await open('/sign-in', 'en', session);
    await enterEmail('ada@example.com');
    expect(await screen.findByText('We could not send the code. Try again.')).toBeTruthy();
    expect(screen.getByLabelText('Email')).toBeTruthy();
  });

  it('says so when the code could not be checked, and lets the visitor retry', async () => {
    const session = fakeSessionClient();
    session.verifyCode.mockRejectedValueOnce(new Error('network'));
    await open('/sign-in', 'en', session);
    await enterEmail('ada@example.com');
    await enterCode(RIGHT_CODE);
    expect(await screen.findByText('Something went wrong. Try again.')).toBeTruthy();
    await enterCode(RIGHT_CODE);
    await waitFor(() => expect(screen).toHavePathname('/plans'));
  });

  it('lets the visitor go back and use a different email', async () => {
    const session = await open('/sign-in', 'en');
    await enterEmail('typo@example.com');
    await fireEvent.press(await screen.findByRole('button', { name: 'Use a different email' }));
    await enterEmail('ada@example.com');
    expect(session.sendCode).toHaveBeenLastCalledWith('ada@example.com');
  });

  it('asks for an email before sending anything', async () => {
    const session = await open('/sign-in', 'en');
    await enterEmail('   ');
    expect(await screen.findByText('Enter your email address.')).toBeTruthy();
    expect(session.sendCode).not.toHaveBeenCalled();
  });

  it('returns the visitor to the screen they were sent from', async () => {
    await open('/sign-in?next=%2Fyou', 'en');
    await enterEmail('ada@example.com');
    await enterCode(RIGHT_CODE);
    await waitFor(() => expect(screen).toHavePathname('/you'));
  });

  it.each(['https://evil.example/', '//evil.example', 'javascript:alert(1)'])('ignores a return address that leaves the app: %s', async (next) => {
    await open(`/sign-in?next=${encodeURIComponent(next)}`, 'en');
    await enterEmail('ada@example.com');
    await enterCode(RIGHT_CODE);
    await waitFor(() => expect(screen).toHavePathname('/plans'));
  });

  it('sends a person who is already signed in on without asking again', async () => {
    await open('/sign-in?next=%2Fyou', 'en', fakeSessionClient({ accessToken: 't', email: 'ada@example.com' }));
    await waitFor(() => expect(screen).toHavePathname('/you'));
  });

  it('shows every state in Vietnamese', async () => {
    await open('/sign-in', 'vi');
    expect(screen.getByRole('header', { name: 'Đăng nhập' })).toBeTruthy();
    await enterEmail('ada@example.com', 'Gửi mã cho tôi');
    expect(await screen.findByText('Chúng tôi đã gửi mã 6 chữ số đến ada@example.com.')).toBeTruthy();
    await fireEvent.changeText(screen.getByLabelText('Mã 6 chữ số'), '000000');
    await fireEvent.press(screen.getByRole('button', { name: 'Đăng nhập' }));
    expect(await screen.findByText('Mã chưa đúng. Hãy kiểm tra và thử lại.')).toBeTruthy();
  });
});

describe('the sign-in screen on a device with a status bar', () => {
  it('starts below the top inset, so the title is not under the status bar', async () => {
    const { useSafeAreaInsets } = jest.requireMock('react-native-safe-area-context');
    useSafeAreaInsets.mockReturnValue({ top: 40, bottom: 0, left: 0, right: 0 });
    await open('/sign-in', 'en');
    expect(StyleSheet.flatten((await screen.findByTestId('screen')).props.style).paddingTop).toBe(40 + Spacing.four);
  });
});

describe('a build missing its Supabase settings', () => {
  const misconfigured = () => Object.assign(fakeSessionClient(), { missingConfig: ['EXPO_PUBLIC_SUPABASE_URL'] });

  it('says at start which value is missing, instead of waiting for a send to fail', async () => {
    await open('/sign-in', 'en', misconfigured());
    expect(await screen.findByText('This build is not set up for sign-in. Missing: EXPO_PUBLIC_SUPABASE_URL.')).toBeTruthy();
    expect(screen.queryByLabelText('Email')).toBeNull();
  });

  it('says it in Vietnamese too', async () => {
    await open('/sign-in', 'vi', misconfigured());
    expect(await screen.findByText('Bản dựng này chưa được cấu hình để đăng nhập: thiếu EXPO_PUBLIC_SUPABASE_URL.')).toBeTruthy();
  });

  it('shows the normal form when nothing is missing', async () => {
    await open('/sign-in', 'en');
    expect(await screen.findByLabelText('Email')).toBeTruthy();
    expect(screen.queryByText(/not set up for sign-in/)).toBeNull();
  });
});

describe('when Auth limits codes', () => {
  it('says to wait a minute, not "something went wrong", when sending is rate-limited', async () => {
    const session = fakeSessionClient();
    session.sendCode.mockRejectedValueOnce(new RateLimitedError());
    await open('/sign-in', 'en', session);
    await enterEmail('ada@example.com');
    expect(await screen.findByText('Too many tries. Wait a minute and try again.')).toBeTruthy();
    expect(screen.queryByText('We could not send the code. Try again.')).toBeNull();
  });

  it('says to wait a minute when checking the code is rate-limited, in Vietnamese too', async () => {
    const session = fakeSessionClient();
    session.verifyCode.mockRejectedValueOnce(new RateLimitedError());
    await open('/sign-in', 'vi', session);
    await enterEmail('ada@example.com', 'Gửi mã cho tôi');
    await fireEvent.changeText(await screen.findByLabelText('Mã 6 chữ số'), RIGHT_CODE);
    await fireEvent.press(screen.getByRole('button', { name: 'Đăng nhập' }));
    expect(await screen.findByText('Bạn đã thử quá nhiều lần. Hãy đợi một phút rồi thử lại.')).toBeTruthy();
  });
});

describe('asking for a new code', () => {
  beforeEach(() => jest.useFakeTimers());
  afterEach(() => jest.useRealTimers());

  it('is off for a minute after a send, says so, and is on again after', async () => {
    const session = await open('/sign-in', 'en');
    await enterEmail('ada@example.com');
    expect(await screen.findByText('You can ask for a new code in 60 seconds.')).toBeTruthy();
    const resend = screen.getByRole('button', { name: 'Send a new code' });
    expect(resend).toBeDisabled();
    await fireEvent.press(resend);
    expect(session.sendCode).toHaveBeenCalledTimes(1);

    await act(async () => {
      jest.advanceTimersByTime(10_000);
    });
    expect(screen.getByText('You can ask for a new code in 50 seconds.')).toBeTruthy();

    await act(async () => {
      jest.advanceTimersByTime(50_000);
    });
    expect(screen.queryByText(/You can ask for a new code/)).toBeNull();
    await fireEvent.press(screen.getByRole('button', { name: 'Send a new code' }));
    expect(session.sendCode).toHaveBeenCalledTimes(2);
    // A new send starts the wait again.
    expect(await screen.findByText('You can ask for a new code in 60 seconds.')).toBeTruthy();
  });

  it('does not let the same address be sent to again by going back to the email step', async () => {
    const session = await open('/sign-in', 'en');
    await enterEmail('ada@example.com');
    await fireEvent.press(await screen.findByRole('button', { name: 'Use a different email' }));
    expect(screen.getByRole('button', { name: 'Send me a code' })).toBeDisabled();
    await fireEvent.changeText(screen.getByLabelText('Email'), 'grace@example.com');
    expect(screen.getByRole('button', { name: 'Send me a code' })).not.toBeDisabled();
    expect(session.sendCode).toHaveBeenCalledTimes(1);
  });

  it('starts the wait over after a rate-limited send, so it is not hammered', async () => {
    const session = fakeSessionClient();
    await open('/sign-in', 'en', session);
    await enterEmail('ada@example.com');
    session.sendCode.mockRejectedValueOnce(new RateLimitedError());
    await act(async () => {
      jest.advanceTimersByTime(60_000);
    });
    await fireEvent.press(await screen.findByRole('button', { name: 'Send a new code' }));
    expect(await screen.findByText('Too many tries. Wait a minute and try again.')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Send a new code' })).toBeDisabled();
  });
});
