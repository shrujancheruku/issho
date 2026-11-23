import { SignupProblemReason, type Plan } from '@issho/api-client';
import { fireEvent, screen, waitFor } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import type { ReactNode } from 'react';
import { StyleSheet } from 'react-native';

import { Spacing } from '@/constants/theme';

import { fakeApi } from '@/test-support/fake-api';
import { fakeSessionClient } from '@/test-support/fake-session-client';
import { aPlan } from '@/test-support/plan-fixtures';
import { signupRefusal } from '@/test-support/problems';
import { TestProviders } from '@/test-support/providers';

import PlansScreen from '../(tabs)/plans';
import TabsLayout from '../(tabs)/_layout';
import TogetherScreen from '../(tabs)/together';
import YouScreen from '../(tabs)/you';
import Index from '../index';
import PlanPage from '../plans/[id]';
import SignIn from '../sign-in';

jest.mock('react-native/Libraries/Utilities/useColorScheme', () => ({ __esModule: true, default: jest.fn(() => 'light') }));
jest.mock('react-native-safe-area-context', () => ({
  ...jest.requireActual('react-native-safe-area-context'),
  useSafeAreaInsets: jest.fn(() => ({ top: 0, bottom: 0, left: 0, right: 0 })),
}));

// 10:00 UTC on 22 November 2026 is 17:00 that day in Da Nang: the first offered date is 25 November.
const NOW = new Date('2026-11-22T10:00:00Z');
beforeEach(() => jest.useFakeTimers({ doNotFake: ['nextTick', 'setImmediate', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval', 'queueMicrotask'], now: NOW }));
afterEach(() => jest.useRealTimers());

const at = (seconds: number) => ({ $typeName: 'google.protobuf.Timestamp', seconds: BigInt(seconds), nanos: 0 });
const mySignup = (over = {}) => ({ $typeName: 'issho.v1.Signup', planId: 'p1', ticks: [{ date: '2026-11-26', part: 'morning' }], hearBy: '2026-11-28', signedAt: at(1000), ...over });

async function open(url: string, { lang = 'en', api = fakeApi(), session, plans = [aPlan({ partsOfDay: ['morning', 'evening'] })] }: { plans?: Plan[]; lang?: 'en' | 'vi'; api?: ReturnType<typeof fakeApi>; session?: ReturnType<typeof fakeSessionClient> } = {}) {
  api.plans.getPlan.mockResolvedValue({ plan: aPlan({ partsOfDay: ['morning', 'evening'], signedUp: 4 }) });
  api.plans.listPlans.mockResolvedValue({ plans });
  await renderRouter(
    {
      index: Index,
      'sign-in': SignIn,
      '(tabs)/_layout': TabsLayout,
      '(tabs)/together': TogetherScreen,
      '(tabs)/plans': PlansScreen,
      '(tabs)/you': YouScreen,
      'plans/[id]': PlanPage,
      profile: () => null,
    },
    { initialUrl: url, wrapper: ({ children }: { children: ReactNode }) => <TestProviders lang={lang} api={api} session={session}>{children}</TestProviders> } as never,
  );
  return api;
}

const tick = (label: string) => screen.getByRole('checkbox', { name: label });

describe('signing up from a plan page', () => {
  it('offers dates from 3 days ahead, with only the parts of the day the plan allows', async () => {
    const api = fakeApi();
    api.profile.getProfile.mockResolvedValue({ profile: { name: 'Ada' } });
    await open('/plans/p1', { api });
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign up' }));
    expect(await screen.findByRole('checkbox', { name: 'Wed, Nov 25 morning' })).toBeTruthy();
    expect(screen.queryByRole('checkbox', { name: /afternoon/ })).toBeNull();
    expect(screen.queryByRole('checkbox', { name: /Nov 24 / })).toBeNull();
    // The plan's round ends on the 28th: the last date is 7 days after it, and the next day is not offered.
    expect(screen.getByRole('checkbox', { name: 'Sat, Dec 5 evening' })).toBeTruthy();
    expect(screen.queryByRole('checkbox', { name: /Dec 6 / })).toBeNull();
  });

  it('opens the profile first when the person has none, returning to this plan', async () => {
    const api = fakeApi();
    api.profile.getProfile.mockResolvedValue({});
    await open('/plans/p1', { api });
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign up' }));
    await waitFor(() => expect(screen).toHavePathname('/profile'));
    expect(screen).toHaveSearchParams({ next: '/plans/p1' });
    expect(api.signup.signUp).not.toHaveBeenCalled();
  });

  it('signs up with the ticked times, then says when they will hear and how many are interested', async () => {
    const api = fakeApi();
    api.profile.getProfile.mockResolvedValue({ profile: { name: 'Ada' } });
    await open('/plans/p1', { api });
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign up' }));
    await fireEvent.press(await screen.findByRole('checkbox', { name: 'Thu, Nov 26 morning' }));
    await fireEvent.press(tick('Sat, Nov 28 evening'));
    api.signup.signUp.mockResolvedValue({ signup: mySignup() });
    api.signup.listMySignups.mockResolvedValue({ signups: [mySignup()] });
    api.plans.getPlan.mockResolvedValue({ plan: aPlan({ partsOfDay: ['morning', 'evening'], signedUp: 5 }) });
    await fireEvent.press(screen.getByRole('button', { name: 'Sign up' }));
    expect(await screen.findByText('You signed up. You will hear by Sat, Nov 28.')).toBeTruthy();
    expect(api.signup.signUp).toHaveBeenCalledWith({
      planId: 'p1',
      ticks: [
        { date: '2026-11-26', part: 'morning' },
        { date: '2026-11-28', part: 'evening' },
      ],
    });
    expect(await screen.findByText('Interested: 5')).toBeTruthy();
  });

  it('asks for at least one time', async () => {
    const api = fakeApi();
    api.profile.getProfile.mockResolvedValue({ profile: { name: 'Ada' } });
    await open('/plans/p1', { api });
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign up' }));
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign up' }));
    expect(await screen.findByText('Tick at least one time.')).toBeTruthy();
    expect(api.signup.signUp).not.toHaveBeenCalled();
  });

  it("prefills the ticks from the person's newest signup (by when it was made), where they fall in the new window", async () => {
    const api = fakeApi();
    api.profile.getProfile.mockResolvedValue({ profile: { name: 'Ada' } });
    // The API lists earliest hear-by first: p3 is last in that list but p2 was signed up most recently.
    api.signup.listMySignups.mockResolvedValue({
      signups: [
        mySignup({ planId: 'p2', hearBy: '2026-11-23', signedAt: at(3000), ticks: [{ date: '2026-11-20', part: 'evening' }, { date: '2026-11-27', part: 'evening' }, { date: '2026-11-27', part: 'afternoon' }] }),
        mySignup({ planId: 'p3', hearBy: '2026-11-24', signedAt: at(2000), ticks: [{ date: '2026-11-26', part: 'morning' }] }),
      ],
    });
    await open('/plans/p1', { api });
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign up' }));
    expect(await screen.findByRole('checkbox', { name: 'Fri, Nov 27 evening', checked: true })).toBeTruthy();
    expect(screen.getAllByRole('checkbox', { checked: true })).toHaveLength(1);
  });

  it('shows the same error state as the rest of the page when the signups could not be loaded, and tries again', async () => {
    const api = fakeApi();
    api.profile.getProfile.mockResolvedValue({ profile: { name: 'Ada' } });
    api.signup.listMySignups.mockRejectedValueOnce(new Error('down'));
    await open('/plans/p1', { api });
    expect(await screen.findByText('The plan could not be loaded.')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Sign up' })).toBeNull();
    await fireEvent.press(screen.getByText('Try again'));
    expect(await screen.findByRole('button', { name: 'Sign up' })).toBeTruthy();
  });

  it('tells a person who is not admitted that they need a join link', async () => {
    const api = fakeApi();
    api.profile.getProfile.mockResolvedValue({ profile: { name: 'Ada' } });
    api.signup.signUp.mockRejectedValue(signupRefusal(SignupProblemReason.NOT_ADMITTED));
    await open('/plans/p1', { api });
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign up' }));
    await fireEvent.press(await screen.findByRole('checkbox', { name: 'Thu, Nov 26 morning' }));
    await fireEvent.press(screen.getByRole('button', { name: 'Sign up' }));
    expect(await screen.findByText('You need a join link to sign up. Ask a member to send you one.')).toBeTruthy();
  });

  it('says so when the signup fails otherwise, keeping the ticks', async () => {
    const api = fakeApi();
    api.profile.getProfile.mockResolvedValue({ profile: { name: 'Ada' } });
    api.signup.signUp.mockRejectedValue(new Error('down'));
    await open('/plans/p1', { api });
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign up' }));
    await fireEvent.press(await screen.findByRole('checkbox', { name: 'Thu, Nov 26 morning' }));
    await fireEvent.press(screen.getByRole('button', { name: 'Sign up' }));
    expect(await screen.findByText('We could not sign you up. Try again.')).toBeTruthy();
    expect(tick('Thu, Nov 26 morning')).toBeTruthy();
  });

  it('sends a signed-out visitor to sign in and back to the plan', async () => {
    await open('/plans/p1', { session: fakeSessionClient(null) });
    await fireEvent.press(await screen.findByRole('button', { name: 'Sign in to sign up' }));
    await waitFor(() => expect(screen).toHavePathname('/sign-in'));
    expect(screen).toHaveSearchParams({ next: '/plans/p1' });
  });

  it('puts nothing about the visitor in the plan page until they are known', async () => {
    const api = fakeApi();
    api.signup.listMySignups.mockResolvedValue({ signups: [mySignup()] });
    await open('/plans/p1', { api, session: fakeSessionClient(null) });
    await screen.findByText('Lantern making in Hoi An');
    expect(screen.queryByText(/You signed up/)).toBeNull();
    expect(api.signup.listMySignups).not.toHaveBeenCalled();
  });
});

describe('a plan the person has signed up to', () => {
  const signedUp = async (extra: Parameters<typeof open>[1] = {}) => {
    const api = extra.api ?? fakeApi();
    api.profile.getProfile.mockResolvedValue({ profile: { name: 'Ada' } });
    api.signup.listMySignups.mockResolvedValue({ signups: [mySignup()] });
    await open('/plans/p1', { ...extra, api });
    return api;
  };

  it('shows its state on the same page: when they hear and how many are interested', async () => {
    await signedUp();
    expect(await screen.findByText('You signed up. You will hear by Sat, Nov 28.')).toBeTruthy();
    expect(screen.getByText('Interested: 4')).toBeTruthy();
    expect(screen.getByText('Lantern making in Hoi An')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Sign up' })).toBeNull();
  });

  it('writes the day on the city clock in Vietnamese', async () => {
    await signedUp({ lang: 'vi' });
    expect(await screen.findByText(/Bạn đã đăng ký\. Bạn sẽ nhận được tin trước Th 7, 28 thg 11\./)).toBeTruthy();
  });

  it('changes the times, starting from the ones already ticked', async () => {
    const api = await signedUp();
    await fireEvent.press(await screen.findByRole('button', { name: 'Change times' }));
    expect(await screen.findByRole('checkbox', { name: 'Thu, Nov 26 morning', checked: true })).toBeTruthy();
    await fireEvent.press(tick('Thu, Nov 26 morning'));
    await fireEvent.press(tick('Sun, Nov 29 evening'));
    await fireEvent.press(screen.getByRole('button', { name: 'Save times' }));
    await waitFor(() => expect(api.signup.signUp).toHaveBeenCalledWith({ planId: 'p1', ticks: [{ date: '2026-11-29', part: 'evening' }] }));
  });

  it('withdraws, and offers the sign-up again', async () => {
    const api = await signedUp();
    api.signup.listMySignups.mockResolvedValue({ signups: [] });
    await fireEvent.press(await screen.findByRole('button', { name: 'Withdraw' }));
    expect(await screen.findByRole('button', { name: 'Sign up' })).toBeTruthy();
    expect(api.signup.withdrawSignup).toHaveBeenCalledWith({ planId: 'p1' });
    expect(screen.queryByText(/You signed up/)).toBeNull();
  });

  it('says so when withdrawing fails, and stays signed up', async () => {
    const api = await signedUp();
    api.signup.withdrawSignup.mockRejectedValue(new Error('down'));
    await fireEvent.press(await screen.findByRole('button', { name: 'Withdraw' }));
    expect(await screen.findByText('We could not withdraw your signup. Try again.')).toBeTruthy();
    expect(screen.getByText('You signed up. You will hear by Sat, Nov 28.')).toBeTruthy();
  });
});

describe('Together', () => {
  it('lists each signup on the day it will hear, in time order', async () => {
    const api = fakeApi();
    api.signup.listMySignups.mockResolvedValue({
      signups: [mySignup({ planId: 'p2', hearBy: '2026-11-30' }), mySignup({ planId: 'p1', hearBy: '2026-11-28' })],
    });
    await open('/together', { api, plans: [aPlan(), aPlan({ id: 'p2', title: 'Sunrise on Son Tra' })] });
    await screen.findByText('Sat, Nov 28');
    expect(screen.getByText('Mon, Nov 30')).toBeTruthy();
    // The API listed the later day first; Together puts the earlier day on top.
    const titles = screen.getAllByText(/You will hear about/).map((n) => n.props.children);
    expect(titles).toEqual(['You will hear about Lantern making in Hoi An', 'You will hear about Sunrise on Son Tra']);
  });

  it('starts below the top inset, so the title is not under the status bar', async () => {
    jest.requireMock('react-native-safe-area-context').useSafeAreaInsets.mockReturnValue({ top: 40, bottom: 0, left: 0, right: 0 });
    await open('/together');
    expect(StyleSheet.flatten((await screen.findByTestId('screen')).props.style).paddingTop).toBe(40 + Spacing.four);
    jest.requireMock('react-native-safe-area-context').useSafeAreaInsets.mockReturnValue({ top: 0, bottom: 0, left: 0, right: 0 });
  });

  it('opens a signup on its plan', async () => {
    const api = fakeApi();
    api.signup.listMySignups.mockResolvedValue({ signups: [mySignup()] });
    await open('/together', { api });
    await fireEvent.press(await screen.findByText('You will hear about Lantern making in Hoi An'));
    await waitFor(() => expect(screen).toHavePathname('/plans/p1'));
  });

  it('says nothing is there when there are no signups, and when they could not be loaded', async () => {
    const api = fakeApi();
    await open('/together', { api });
    expect(await screen.findByText('Nothing here yet.')).toBeTruthy();
    const failing = fakeApi();
    failing.signup.listMySignups.mockRejectedValue(new Error('down'));
    await open('/together', { api: failing });
    expect(await screen.findByText('Your plans could not be loaded.')).toBeTruthy();
  });
});

describe('what the app opens on', () => {
  it('is Together when the person has something in play', async () => {
    const api = fakeApi();
    api.signup.listMySignups.mockResolvedValue({ signups: [mySignup()] });
    await open('/', { api });
    await waitFor(() => expect(screen).toHavePathname('/together'));
  });

  it('is Plans when they have nothing in play', async () => {
    await open('/');
    await waitFor(() => expect(screen).toHavePathname('/plans'));
  });

  it('is Plans for a visitor with no account, without asking for signups', async () => {
    const api = fakeApi();
    await open('/', { api, session: fakeSessionClient(null) });
    await waitFor(() => expect(screen).toHavePathname('/plans'));
    expect(api.signup.listMySignups).not.toHaveBeenCalled();
  });

  it('is Plans when the signups could not be read', async () => {
    const api = fakeApi();
    api.signup.listMySignups.mockRejectedValue(new Error('down'));
    await open('/', { api });
    await waitFor(() => expect(screen).toHavePathname('/plans'));
  });

  it('moves a signed-in person on from the server\'s redirect of / to Plans, only when they have something in play', async () => {
    const api = fakeApi();
    api.signup.listMySignups.mockResolvedValue({ signups: [mySignup()] });
    await open('/plans?open=1', { api });
    await waitFor(() => expect(screen).toHavePathname('/together'));
  });

  it('leaves someone who chose Plans on Plans', async () => {
    const api = fakeApi();
    api.signup.listMySignups.mockResolvedValue({ signups: [mySignup()] });
    await open('/plans', { api });
    await screen.findByText('Lantern making in Hoi An');
    expect(screen).toHavePathname('/plans');
  });
});
