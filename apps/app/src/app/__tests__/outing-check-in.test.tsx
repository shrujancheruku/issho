import { Code, ConnectError } from '@connectrpc/connect';
import { OutingState, SeatState } from '@issho/api-client';
import { fireEvent, screen, waitFor, within } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import type { ReactNode } from 'react';

import * as format from '@/lib/format';
import { fakeApi } from '@/test-support/fake-api';
import { anOuting } from '@/test-support/outing-fixtures';
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

// The outing is Wed 25 Nov, 09:00 to 12:30 in Da Nang (02:00 to 05:30 UTC); check-in opens at 08:30 (01:30 UTC).
const THIRTY_MINUTES_BEFORE = '2026-11-25T01:30:00Z';
const ON_THE_DAY = '2026-11-25T01:45:00Z';
const JUST_BEFORE_THE_END = '2026-11-25T05:29:00Z';
const THE_END = '2026-11-25T05:30:00Z';

const clock = (iso: string) => jest.setSystemTime(new Date(iso));
beforeEach(() => {
  jest.useFakeTimers({ doNotFake: ['nextTick', 'setImmediate', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval', 'queueMicrotask'], now: new Date(ON_THE_DAY) });
  jest.spyOn(format, 'deviceTimeZone').mockReturnValue('Asia/Ho_Chi_Minh');
});
afterEach(() => {
  jest.useRealTimers();
  jest.restoreAllMocks();
});

const going = (over = {}) => anOuting({ seatState: SeatState.GOING, outingState: OutingState.SET, yesCount: 3, othersGoing: ['Linh', 'Minh'], ...over });
const refused = () => new ConnectError('refused', Code.FailedPrecondition);

async function open(url: string, { lang = 'en', api = fakeApi() }: { lang?: 'en' | 'vi'; api?: ReturnType<typeof fakeApi> } = {}) {
  api.plans.getPlan.mockResolvedValue({ plan: aPlan() });
  api.plans.listPlans.mockResolvedValue({ plans: [aPlan()] });
  await renderRouter(
    { index: Index, '(tabs)/_layout': TabsLayout, '(tabs)/together': TogetherScreen, '(tabs)/plans': PlansScreen, '(tabs)/you': YouScreen, 'plans/[id]': PlanPage },
    { initialUrl: url, wrapper: ({ children }: { children: ReactNode }) => <TestProviders lang={lang} api={api}>{children}</TestProviders> } as never,
  );
  return api;
}

describe('check-in on the day', () => {
  it('offers "I\'m here" from 30 minutes before the start, and not a minute sooner', async () => {
    clock(THIRTY_MINUTES_BEFORE);
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    expect(within(strip).getByRole('button', { name: "I'm here" })).toBeTruthy();
  });

  it('says when check-in opens, with no button, before it does', async () => {
    clock('2026-11-25T01:29:00Z');
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    expect(within(strip).getByText('Check-in opens Wed, Nov 25, 8:30 AM.')).toBeTruthy();
    expect(within(strip).queryByRole('button', { name: "I'm here" })).toBeNull();
  });

  it('stays open to the plan\'s end, which is not part of it', async () => {
    clock(JUST_BEFORE_THE_END);
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
    await open('/plans/p1', { api });
    expect(within(await screen.findByTestId('outing-strip')).getByRole('button', { name: "I'm here" })).toBeTruthy();
  });

  it('offers no check-in to someone who is not going', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting({ seatState: SeatState.ACCEPTED })] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    expect(within(strip).queryByRole('button', { name: "I'm here" })).toBeNull();
    expect(within(strip).queryByText(/Check-in opens/)).toBeNull();
  });

  it('checks in with one tap, then shows who else has, and no cancel', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValueOnce({ outings: [going()] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    expect(within(strip).getByRole('button', { name: 'Cancel my place' })).toBeTruthy();
    api.outing.checkIn.mockResolvedValue({});
    api.outing.listMyOutings.mockResolvedValue({ outings: [going({ checkedIn: true, checkedInNames: ['Linh'] })] });
    await fireEvent.press(within(strip).getByRole('button', { name: "I'm here" }));
    expect(api.outing.checkIn).toHaveBeenCalledWith({ outingId: 'o1' });
    expect(await within(strip).findByText('You have checked in.')).toBeTruthy();
    expect(within(strip).getByText('Also here: Linh')).toBeTruthy();
    expect(within(strip).queryByRole('button', { name: "I'm here" })).toBeNull();
    expect(within(strip).queryByRole('button', { name: 'Cancel my place' })).toBeNull();
  });

  it('says so when no one else has checked in yet', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going({ checkedIn: true })] });
    await open('/plans/p1', { api });
    expect(await screen.findByText('No one else has checked in yet.')).toBeTruthy();
  });

  it('says the outing is over when the end passed with the screen open, and lets go of the button', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    clock(THE_END);
    api.outing.checkIn.mockRejectedValue(refused());
    await fireEvent.press(within(strip).getByRole('button', { name: "I'm here" }));
    expect(await screen.findByText('This outing is over, so check-in has closed.')).toBeTruthy();
  });

  it('says it went wrong for any other failure, and the button works again', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    api.outing.checkIn.mockRejectedValue(new ConnectError('down', Code.Unavailable));
    await fireEvent.press(within(strip).getByRole('button', { name: "I'm here" }));
    expect(await screen.findByText('We could not check you in. Try again.')).toBeTruthy();
    await waitFor(() => expect(within(strip).getByRole('button', { name: "I'm here" })).not.toBeDisabled());
  });

  it('reads in Vietnamese', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
    await open('/plans/p1', { lang: 'vi', api });
    expect(within(await screen.findByTestId('outing-strip')).getByRole('button', { name: 'Tôi đến rồi' })).toBeTruthy();
  });

  it('is shown on the client only: a visitor\'s cached page calls nothing', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
    await open('/plans/p1', { api });
    await screen.findByTestId('outing-strip');
    expect(api.outing.checkIn).not.toHaveBeenCalled();
  });
});
