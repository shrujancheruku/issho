import { renderRouter, screen } from 'expo-router/testing-library';
import type { ReactNode } from 'react';

import { fakeApi } from '@/test-support/fake-api';
import { TestProviders } from '@/test-support/providers';

import PlansScreen from '../(tabs)/plans';
import TogetherScreen from '../(tabs)/together';
import YouScreen from '../(tabs)/you';
import TabsLayout from '../(tabs)/_layout';
import Index from '../index';

// React Native's hook binds the OS setting when it loads, so a test replaces the hook itself.
jest.mock('react-native/Libraries/Utilities/useColorScheme', () => ({ __esModule: true, default: jest.fn(() => 'light') }));

// The app's shape (docs/spec/app-shape.md): three tabs, Plans first for a
// visitor. The API is the generated client's fake, so no test reaches a network.
async function renderApp(lang: 'en' | 'vi', api = fakeApi()) {
  await renderRouter(
    {
      index: Index,
      '(tabs)/_layout': TabsLayout,
      '(tabs)/together': TogetherScreen,
      '(tabs)/plans': PlansScreen,
      '(tabs)/you': YouScreen,
    },
    { wrapper: ({ children }: { children: ReactNode }) => <TestProviders lang={lang} api={api}>{children}</TestProviders> } as never,
  );
  return api;
}

// react-navigation labels each tab "<title>, tab, <n> of 3"; the title is ours.
const tabNames = () => screen.getAllByLabelText(/, tab, \d of 3$/).map((tab) => tab.props.accessibilityLabel.split(',')[0]);

describe('the three tabs', () => {
  it('opens on Plans for a visitor', async () => {
    await renderApp('en');
    expect(screen).toHavePathname('/plans');
    expect(screen.getByText('No plans yet.')).toBeTruthy();
  });

  it('shows Together, Plans and You as empty screens', async () => {
    await renderApp('en');
    expect(tabNames()).toEqual(['Together', 'Plans', 'You']);
  });

  it('shows the tabs and the screen in Vietnamese', async () => {
    await renderApp('vi');
    expect(tabNames()).toEqual(['Cùng nhau', 'Kế hoạch', 'Bạn']);
    expect(screen.getByText('Chưa có kế hoạch nào.')).toBeTruthy();
  });

  it('makes no API call while a screen is empty', async () => {
    const api = await renderApp('en');
    expect(api.health.check).not.toHaveBeenCalled();
  });

  it.each([
    ['dark', '#0D1330'],
    ['light', '#F3F5F9'],
  ] as const)('follows the device setting: %s', async (scheme, background) => {
    jest.mocked(jest.requireMock('react-native/Libraries/Utilities/useColorScheme').default).mockReturnValue(scheme);
    await renderApp('en');
    expect(screen.getByTestId('screen')).toHaveStyle({ backgroundColor: background });
  });
});
