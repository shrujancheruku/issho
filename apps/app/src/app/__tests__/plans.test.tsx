import { Code, ConnectError } from '@connectrpc/connect';
import { fireEvent, screen, waitFor } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import type { ReactNode } from 'react';
import { StyleSheet } from 'react-native';

import { Spacing } from '@/constants/theme';
import { fakeApi } from '@/test-support/fake-api';
import { aPlan } from '@/test-support/plan-fixtures';
import { TestProviders } from '@/test-support/providers';

import PlansScreen from '../(tabs)/plans';
import TabsLayout from '../(tabs)/_layout';
import TogetherScreen from '../(tabs)/together';
import YouScreen from '../(tabs)/you';
import Index from '../index';
import PlanPage from '../plans/[id]';

jest.mock('react-native/Libraries/Utilities/useColorScheme', () => ({ __esModule: true, default: jest.fn(() => 'light') }));

jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context'),
  useSafeAreaInsets: jest.fn(() => ({ top: 0, bottom: 0, left: 0, right: 0 })),
}));

async function renderApp(lang: 'en' | 'vi', api = fakeApi(), initialUrl = '/plans') {
  await renderRouter(
    {
      index: Index,
      '(tabs)/_layout': TabsLayout,
      '(tabs)/together': TogetherScreen,
      '(tabs)/plans': PlansScreen,
      '(tabs)/you': YouScreen,
      'plans/[id]': PlanPage,
    },
    { initialUrl, wrapper: ({ children }: { children: ReactNode }) => <TestProviders lang={lang} api={api}>{children}</TestProviders> } as never,
  );
  return api;
}

// A device whose status bar is 40 points tall (Android draws edge to edge).
function mockTopInset() {
  jest.requireMock('react-native-safe-area-context').useSafeAreaInsets.mockReturnValue({ top: 40, bottom: 0, left: 0, right: 0 });
}

const notFound = () => new ConnectError('not found', Code.NotFound);

describe('the Plans tab', () => {
  it("lists the city's plans, asking in the display language", async () => {
    const api = fakeApi();
    api.plans.listPlans.mockResolvedValue({ plans: [aPlan(), aPlan({ id: 'p2', title: 'Sunrise on Son Tra', price: 0 })] });
    await renderApp('en', api);
    expect(await screen.findByText('Lantern making in Hoi An')).toBeTruthy();
    expect(screen.getByText('Sunrise on Son Tra')).toBeTruthy();
    expect(screen.getByText('₫350,000')).toBeTruthy();
    expect(screen.getByText('Free')).toBeTruthy();
    expect(screen.getAllByText('09:00–12:30')).toHaveLength(2);
    expect(api.plans.listPlans).toHaveBeenCalledWith({ city: 'da-nang-hoi-an', displayLanguage: 'en' });
  });

  it('asks for Vietnamese when the display language is Vietnamese', async () => {
    const api = await renderApp('vi');
    await waitFor(() => expect(api.plans.listPlans).toHaveBeenCalledWith({ city: 'da-nang-hoi-an', displayLanguage: 'vi' }));
    expect(await screen.findByText('Chưa có kế hoạch nào.')).toBeTruthy();
  });

  it('says there are no plans when the city has none', async () => {
    await renderApp('en');
    expect(await screen.findByText('No plans yet.')).toBeTruthy();
  });

  it('says the plans could not be loaded, and tries again on request', async () => {
    const api = fakeApi();
    api.plans.listPlans.mockRejectedValueOnce(new Error('down')).mockResolvedValue({ plans: [aPlan()] });
    await renderApp('en', api);
    expect(await screen.findByText('The plans could not be loaded.')).toBeTruthy();
    await fireEvent.press(screen.getByText('Try again'));
    expect(await screen.findByText('Lantern making in Hoi An')).toBeTruthy();
  });

  it('starts below the top inset, so the title is not under the status bar', async () => {
    mockTopInset();
    await renderApp('en');
    expect(StyleSheet.flatten((await screen.findByTestId('screen')).props.style).paddingTop).toBe(40 + Spacing.four);
  });
});

describe('a plan page', () => {
  it('opens from the list and shows the title, stops with their legs, price, meeting point and what to bring', async () => {
    const api = fakeApi();
    api.plans.listPlans.mockResolvedValue({ plans: [aPlan()] });
    api.plans.getPlan.mockResolvedValue({ plan: aPlan() });
    await renderApp('en', api);
    await fireEvent.press(await screen.findByText('Lantern making in Hoi An'));
    expect(await screen.findByText('Fold and paint a silk lantern with a local maker.')).toBeTruthy();
    expect(screen).toHavePathname('/plans/p1');
    expect(api.plans.getPlan).toHaveBeenCalledWith({ id: 'p1', displayLanguage: 'en' });
    expect(screen.getByText('₫350,000')).toBeTruthy();
    expect(screen.getByText('Lantern studio')).toBeTruthy();
    expect(screen.getByText('Riverside café')).toBeTruthy();
    expect(screen.getByText('90 min')).toBeTruthy();
    expect(screen.getByText('Walk, 10 min to the next stop')).toBeTruthy();
    expect(screen.getByText('12 Nguyen Thai Hoc, Hoi An')).toBeTruthy();
    expect(screen.getByText('The blue door beside the tailor')).toBeTruthy();
    expect(screen.getByText('Sunscreen')).toBeTruthy();
    expect(screen.getByText('A small bag')).toBeTruthy();
  });

  it('reads in Vietnamese when asked to', async () => {
    const api = fakeApi();
    api.plans.getPlan.mockResolvedValue({ plan: aPlan({ language: 'vi', title: 'Làm đèn lồng ở Hội An' }) });
    await renderApp('vi', api, '/plans/p1');
    expect(await screen.findByText('Làm đèn lồng ở Hội An')).toBeTruthy();
    expect(api.plans.getPlan).toHaveBeenCalledWith({ id: 'p1', displayLanguage: 'vi' });
    expect(screen.getByText('Mang theo')).toBeTruthy();
  });

  it('says so when the plan has no text in the language asked for and reads in English', async () => {
    const api = fakeApi();
    api.plans.getPlan.mockResolvedValue({ plan: aPlan({ language: 'en' }) });
    await renderApp('vi', api, '/plans/p1');
    expect(await screen.findByText('Kế hoạch này chưa có bản tiếng Việt, nên hiện bằng tiếng Anh.')).toBeTruthy();
  });

  it("says a plan that doesn't exist is not found, and offers the list", async () => {
    const api = fakeApi();
    api.plans.getPlan.mockRejectedValue(notFound());
    await renderApp('en', api, '/plans/nope');
    expect(await screen.findByText('We could not find that plan.')).toBeTruthy();
    await fireEvent.press(screen.getByText('See all plans'));
    await waitFor(() => expect(screen).toHavePathname('/plans'));
  });

  it('says the plan could not be loaded when the API fails otherwise, not that it is missing', async () => {
    const api = fakeApi();
    api.plans.getPlan.mockRejectedValue(new Error('down'));
    await renderApp('en', api, '/plans/p1');
    expect(await screen.findByText('The plan could not be loaded.')).toBeTruthy();
    expect(screen.queryByText('We could not find that plan.')).toBeNull();
  });

  it('starts below the top inset', async () => {
    const api = fakeApi();
    api.plans.getPlan.mockResolvedValue({ plan: aPlan() });
    mockTopInset();
    await renderApp('en', api, '/plans/p1');
    await screen.findByText('Lantern studio');
    expect(StyleSheet.flatten((await screen.findByTestId('screen')).props.style).paddingTop).toBe(40 + Spacing.four);
  });
});
