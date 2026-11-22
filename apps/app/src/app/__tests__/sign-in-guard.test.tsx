import { fireEvent, screen, waitFor } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import type { ReactNode } from 'react';

import { fakeApi } from '@/test-support/fake-api';
import { fakeSessionClient, RIGHT_CODE } from '@/test-support/fake-session-client';
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

async function open(path: string, session = fakeSessionClient()) {
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
        <TestProviders lang="en" api={fakeApi()} session={session}>
          {children}
        </TestProviders>
      ),
    } as never,
  );
  return session;
}

describe('signed-in screens', () => {
  it.each(['/you', '/together'])('send a signed-out visitor from %s to sign-in', async (path) => {
    await open(path);
    await waitFor(() => expect(screen).toHavePathname('/sign-in'));
    expect(screen).toHaveSearchParams({ next: path });
  });

  it('return the visitor to the screen they asked for, once signed in', async () => {
    await open('/you');
    await waitFor(() => expect(screen).toHavePathname('/sign-in'));
    await fireEvent.changeText(await screen.findByLabelText('Email'), 'ada@example.com');
    await fireEvent.press(screen.getByRole('button', { name: 'Send me a code' }));
    await fireEvent.changeText(await screen.findByLabelText('6-digit code'), RIGHT_CODE);
    await fireEvent.press(screen.getByRole('button', { name: 'Sign in' }));
    await waitFor(() => expect(screen).toHavePathname('/you'));
    expect(await screen.findByText('Signed in as ada@example.com')).toBeTruthy();
  });

  it('leave the public Plans screen open to a visitor', async () => {
    await open('/plans');
    expect(await screen.findByText('No plans yet.')).toBeTruthy();
    expect(screen).toHavePathname('/plans');
  });

  it('wait for the stored session before deciding, so a restart does not bounce a signed-in person', async () => {
    const session = fakeSessionClient({
      accessToken: 't',
      email: 'ada@example.com',
    });
    let release!: () => void;
    const stored = new Promise<void>((resolve) => (release = resolve));
    const read = session.getSession.getMockImplementation()!;
    session.getSession.mockImplementation(async () => {
      await stored;
      return read();
    });
    await open('/you', session);
    expect(screen).toHavePathname('/you');
    release();
    expect(await screen.findByText('Signed in as ada@example.com')).toBeTruthy();
    expect(screen).toHavePathname('/you');
  });
});
