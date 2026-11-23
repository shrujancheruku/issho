import { Code, ConnectError } from '@connectrpc/connect';
import { OutingState, SeatState } from '@issho/api-client';
import { fireEvent, screen, waitFor, within } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import type { ReactNode } from 'react';

import * as format from '@/lib/format';
import { fakeApi } from '@/test-support/fake-api';
import { viWeekday } from '@/test-support/weekday';
import { fakeSessionClient } from '@/test-support/fake-session-client';
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

// 10:00 UTC on 22 November 2026 is 17:00 that day in Da Nang.
const NOW = new Date('2026-11-22T10:00:00Z');
// The device is on the city's clock unless a test says otherwise.
const deviceIn = (zone: string) => jest.spyOn(format, 'deviceTimeZone').mockReturnValue(zone);
beforeEach(() => {
  jest.useFakeTimers({ doNotFake: ['nextTick', 'setImmediate', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval', 'queueMicrotask'], now: NOW });
  deviceIn('Asia/Ho_Chi_Minh');
});
afterEach(() => {
  jest.useRealTimers();
  jest.restoreAllMocks();
});

async function open(url: string, { lang = 'en', api = fakeApi() }: { lang?: 'en' | 'vi'; api?: ReturnType<typeof fakeApi> } = {}) {
  api.plans.getPlan.mockImplementation(async ({ id }: { id: string }) => ({ plan: id === 'p2' ? aPlan({ id: 'p2', title: 'Night market food walk' }) : aPlan() }));
  api.plans.listPlans.mockResolvedValue({ plans: [aPlan(), aPlan({ id: 'p2', title: 'Night market food walk' })] });
  await renderRouter(
    {
      index: Index,
      '(tabs)/_layout': TabsLayout,
      '(tabs)/together': TogetherScreen,
      '(tabs)/plans': PlansScreen,
      '(tabs)/you': YouScreen,
      'plans/[id]': PlanPage,
    },
    { initialUrl: url, wrapper: ({ children }: { children: ReactNode }) => <TestProviders lang={lang} api={api}>{children}</TestProviders> } as never,
  );
  return api;
}

const refused = () => new ConnectError('refused', Code.FailedPrecondition);

describe('an invitation in "Needs you"', () => {
  it('is a card with the plan, the date and time, the count of yeses and the shared languages', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting()] });
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    expect(screen.getByText('Needs you')).toBeTruthy();
    expect(within(card).getByText('Lantern making in Hoi An')).toBeTruthy();
    expect(within(card).getByText('Wed, Nov 25, 9:00 AM')).toBeTruthy();
    expect(within(card).getByText('2 have said yes')).toBeTruthy();
    expect(within(card).getByText('You share: English, Vietnamese')).toBeTruthy();
    // An invited person sees how many said yes, never who.
    expect(within(card).queryByText(/Linh/)).toBeNull();
  });

  it("shows the device's time beside the city's when they differ", async () => {
    deviceIn('America/Los_Angeles');
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting()] });
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    expect(within(card).getByText('Wed, Nov 25, 9:00 AM')).toBeTruthy();
    expect(within(card).getByText('Your time: Tue, Nov 24, 6:00 PM')).toBeTruthy();
  });

  it('shows no second time when the device is on the city clock', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting()] });
    await open('/together', { api });
    await screen.findByTestId('needs-you-o1');
    expect(screen.queryByText(/Your time/)).toBeNull();
  });

  it('is said yes to in place, and the card goes with the answer', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValueOnce({ outings: [anOuting()] });
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting({ seatState: SeatState.ACCEPTED, yesCount: 3 })] });
    await fireEvent.press(within(card).getByRole('button', { name: 'Yes' }));
    expect(api.outing.answerInvitation).toHaveBeenCalledWith({ outingId: 'o1', answer: 1 });
    await waitFor(() => expect(screen.queryByTestId('needs-you-o1')).toBeNull());
    expect(screen.getByText('You said yes. It is set when 3 have.')).toBeTruthy();
  });

  it('is said no to in place', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValueOnce({ outings: [anOuting()] });
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting({ seatState: SeatState.DECLINED })] });
    await fireEvent.press(within(card).getByRole('button', { name: 'No' }));
    expect(api.outing.answerInvitation).toHaveBeenCalledWith({ outingId: 'o1', answer: 2 });
    await waitFor(() => expect(screen.queryByTestId('needs-you-o1')).toBeNull());
    expect(screen.getByText('You said no.')).toBeTruthy();
  });

  it('says the time is taken when the yes is refused for a time the person is already going', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting()] });
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    api.outing.answerInvitation.mockRejectedValue(refused());
    await fireEvent.press(within(card).getByRole('button', { name: 'Yes' }));
    expect(await screen.findByText("You are already going to another outing at that time, so you can't go to this one.")).toBeTruthy();
  });

  it('says the invitation has run out when its 48 hours ended', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValueOnce({ outings: [anOuting()] });
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    api.outing.answerInvitation.mockRejectedValue(refused());
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting({ seatState: SeatState.UNANSWERED })] });
    await fireEvent.press(within(card).getByRole('button', { name: 'Yes' }));
    expect(await screen.findByText('This invitation has run out.')).toBeTruthy();
  });

  it('says the group is full when the seat went to someone else', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValueOnce({ outings: [anOuting()] });
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    api.outing.answerInvitation.mockRejectedValue(refused());
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting({ seatState: SeatState.FULL })] });
    await fireEvent.press(within(card).getByRole('button', { name: 'Yes' }));
    expect(await screen.findByText('This group is full.')).toBeTruthy();
  });

  it('says it went wrong for any other failure, and lets the person try again', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting()] });
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    api.outing.answerInvitation.mockRejectedValue(new ConnectError('down', Code.Unavailable));
    await fireEvent.press(within(card).getByRole('button', { name: 'Yes' }));
    expect(await screen.findByText('We could not record your answer. Try again.')).toBeTruthy();
    expect(within(screen.getByTestId('needs-you-o1')).getByRole('button', { name: 'Yes' })).toBeTruthy();
  });

  it('says the outing is no longer theirs when it is not found, even once it has left the list', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValueOnce({ outings: [anOuting()] });
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    api.outing.answerInvitation.mockRejectedValue(new ConnectError('no such outing', Code.NotFound));
    api.outing.listMyOutings.mockResolvedValue({ outings: [] });
    await fireEvent.press(within(card).getByRole('button', { name: 'Yes' }));
    expect(await screen.findByText('This outing is no longer yours.')).toBeTruthy();
    expect(screen.queryByTestId('needs-you-o1')).toBeNull();
    expect(screen.queryByText('We could not record your answer. Try again.')).toBeNull();
  });

  it('says the outing is no longer theirs when the seat is not theirs to answer', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting()] });
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    api.outing.answerInvitation.mockRejectedValue(new ConnectError('not yours', Code.PermissionDenied));
    await fireEvent.press(within(card).getByRole('button', { name: 'Yes' }));
    expect(await screen.findByText('This outing is no longer yours.')).toBeTruthy();
  });

  it('reads in Vietnamese', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting()] });
    await open('/together', { lang: 'vi', api });
    const card = await screen.findByTestId('needs-you-o1');
    expect(screen.getByText('Cần bạn trả lời')).toBeTruthy();
    expect(within(card).getByText(`9:00 ${viWeekday('2026-11-25')}, 25 thg 11`)).toBeTruthy();
    expect(within(card).getByText('2 người đã đồng ý')).toBeTruthy();
    expect(within(card).getByText('Cùng nói được: Tiếng Anh, Tiếng Việt')).toBeTruthy();
  });
});

describe('the Together tab', () => {
  it('carries a count of what needs an answer', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting(), anOuting({ id: 'o2', planId: 'p2' }), anOuting({ id: 'o3', seatState: SeatState.GOING, outingState: OutingState.SET })] });
    await open('/plans', { api });
    expect(await screen.findByLabelText('Together, 2 need an answer')).toBeTruthy();
  });

  it('carries no count when nothing needs an answer', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting({ seatState: SeatState.GOING, outingState: OutingState.SET })] });
    await open('/together', { api });
    await screen.findByText('Lantern making in Hoi An');
    expect(screen.queryByLabelText(/need an answer/)).toBeNull();
  });

  it('opens on Together for someone whose only play is an outing', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting()] });
    await open('/', { api });
    await waitFor(() => expect(screen).toHavePathname('/together'));
  });

  it('lists a set outing on its date, between the signups, opening the plan page', async () => {
    const api = fakeApi();
    api.signup.listMySignups.mockResolvedValue({
      signups: [
        { $typeName: 'issho.v1.Signup', planId: 'p2', ticks: [], hearBy: '2026-11-24' },
        { $typeName: 'issho.v1.Signup', planId: 'p2', ticks: [], hearBy: '2026-11-27' },
      ],
    });
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting({ seatState: SeatState.GOING, outingState: OutingState.SET })] });
    await open('/together', { api });
    const rows = await screen.findAllByRole('link');
    expect(rows.map((r) => within(r).queryAllByText(/./).map((t) => t.props.children).flat().join(' '))).toEqual([
      expect.stringContaining('Nov 24'),
      expect.stringContaining('Nov 25'),
      expect.stringContaining('Nov 27'),
    ]);
    expect(within(rows[1]).getByText("You're going")).toBeTruthy();
    await fireEvent.press(rows[1]);
    await waitFor(() => expect(screen).toHavePathname('/plans/p1'));
  });

  it.each([
    ['an outing that was not set', { outingState: OutingState.NOT_SET, seatState: SeatState.ACCEPTED }, 'Not set: fewer than 3 said yes'],
    ['an outing that was cancelled', { outingState: OutingState.CANCELLED, seatState: SeatState.GOING }, 'Cancelled'],
    ['an outing the person said no to', { seatState: SeatState.DECLINED }, 'You said no.'],
    ['an outing the person cancelled their place in', { outingState: OutingState.SET, seatState: SeatState.CANCELLED }, 'You cancelled your place.'],
  ])('shows %s as muted', async (_name, over, line) => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting(over)] });
    await open('/together', { api });
    const row = (await screen.findByText(line)).parent!.parent!;
    expect(screen.getByTestId('together-row-o1')).toHaveStyle({ opacity: 0.6 });
    expect(row).toBeTruthy();
  });

  it('shows a going outing at full strength', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting({ seatState: SeatState.GOING, outingState: OutingState.SET })] });
    await open('/together', { api });
    await screen.findByText("You're going");
    expect(screen.getByTestId('together-row-o1')).not.toHaveStyle({ opacity: 0.6 });
  });

  it('says the outings could not be loaded, and tries again', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockRejectedValueOnce(new Error('down'));
    await open('/together', { api });
    expect(await screen.findByText('Your plans could not be loaded.')).toBeTruthy();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting()] });
    await fireEvent.press(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByTestId('needs-you-o1')).toBeTruthy();
  });
});

describe('the outing on the plan page', () => {
  const going = () => anOuting({ seatState: SeatState.GOING, outingState: OutingState.SET, yesCount: 3, othersGoing: ['Linh', 'Minh'] });

  it('puts a strip on top of the plan with the date and time, the languages, the others going and the stage', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    expect(within(strip).getByText('Wed, Nov 25, 9:00 AM')).toBeTruthy();
    expect(within(strip).getByText('You share: English, Vietnamese')).toBeTruthy();
    expect(within(strip).getByText('Going with: Linh, Minh')).toBeTruthy();
    expect(within(strip).getByText('Stage: Set')).toBeTruthy();
    // Below it, the plan exactly as anyone sees it.
    expect(screen.getByText('Fold and paint a silk lantern with a local maker.')).toBeTruthy();
    expect(screen.getByText('The blue door beside the tailor')).toBeTruthy();
  });

  it('shows no strip on a plan the person has no outing for', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
    await open('/plans/p2', { api });
    await screen.findByText('Night market food walk');
    expect(screen.queryByTestId('outing-strip')).toBeNull();
  });

  it('is cancelled after a confirmation, and not before it', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValueOnce({ outings: [going()] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    await fireEvent.press(within(strip).getByRole('button', { name: 'Cancel my place' }));
    expect(api.outing.cancelSeat).not.toHaveBeenCalled();
    expect(within(strip).getByText('Give up your place in this outing?')).toBeTruthy();
    // Keeping the place closes the question and calls nothing.
    await fireEvent.press(within(strip).getByRole('button', { name: 'Keep my place' }));
    expect(api.outing.cancelSeat).not.toHaveBeenCalled();
    await fireEvent.press(within(strip).getByRole('button', { name: 'Cancel my place' }));
    api.outing.listMyOutings.mockResolvedValue({ outings: [{ ...going(), seatState: SeatState.CANCELLED, othersGoing: [] }] });
    await fireEvent.press(within(strip).getByRole('button', { name: 'Yes, cancel my place' }));
    expect(api.outing.cancelSeat).toHaveBeenCalledWith({ outingId: 'o1' });
    expect(await screen.findByText('You cancelled your place.')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Cancel my place' })).toBeNull();
  });

  it('says why a cancel was refused once checked in, with no cancel to offer', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValueOnce({ outings: [going()] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    await fireEvent.press(within(strip).getByRole('button', { name: 'Cancel my place' }));
    api.outing.cancelSeat.mockRejectedValue(refused());
    api.outing.listMyOutings.mockResolvedValue({ outings: [{ ...going(), checkedIn: true }] });
    await fireEvent.press(within(strip).getByRole('button', { name: 'Yes, cancel my place' }));
    expect(await screen.findByText('You have checked in, so you can no longer cancel.')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Cancel my place' })).toBeNull();
  });

  it('answers an invitation from the strip, showing a count of yeses and never who', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting()] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    expect(within(strip).getByText('2 have said yes')).toBeTruthy();
    expect(within(strip).getByText('Stage: Offered')).toBeTruthy();
    await fireEvent.press(within(strip).getByRole('button', { name: 'Yes' }));
    expect(api.outing.answerInvitation).toHaveBeenCalledWith({ outingId: 'o1', answer: 1 });
  });

  it('offers a yes in place of a no while the 48 hours last', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting({ seatState: SeatState.DECLINED })] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    expect(within(strip).getByText('You said no.')).toBeTruthy();
    expect(within(strip).getByRole('button', { name: 'Yes' })).toBeTruthy();
    expect(within(strip).queryByRole('button', { name: 'No' })).toBeNull();
  });

  it.each([
    ['not set', { outingState: OutingState.NOT_SET, seatState: SeatState.ACCEPTED }, 'Stage: Not set', 'Not set: fewer than 3 said yes'],
    ['cancelled', { outingState: OutingState.CANCELLED, seatState: SeatState.GOING }, 'Stage: Cancelled', 'Cancelled'],
  ])('shows an outing that is %s as muted, with nothing to answer or cancel', async (_name, over, stage, line) => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [anOuting(over)] });
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    expect(strip).toHaveStyle({ opacity: 0.6 });
    expect(within(strip).getByText(stage)).toBeTruthy();
    expect(within(strip).getByText(line)).toBeTruthy();
    expect(within(strip).queryAllByRole('button')).toHaveLength(0);
  });

  it("reads in Vietnamese, with the stage in the spec's words", async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
    await open('/plans/p1', { lang: 'vi', api });
    const strip = await screen.findByTestId('outing-strip');
    expect(within(strip).getByText('Đi cùng: Linh, Minh')).toBeTruthy();
    expect(within(strip).getByText('Giai đoạn: Đã chốt')).toBeTruthy();
  });

  it('calls nothing for a visitor, and draws nothing per person into the page', async () => {
    const api = fakeApi();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
    api.plans.getPlan.mockResolvedValue({ plan: aPlan() });
    await renderRouter(
      { index: Index, '(tabs)/_layout': TabsLayout, '(tabs)/together': TogetherScreen, '(tabs)/plans': PlansScreen, '(tabs)/you': YouScreen, 'plans/[id]': PlanPage },
      { initialUrl: '/plans/p1', wrapper: ({ children }: { children: ReactNode }) => <TestProviders lang="en" api={api} session={fakeSessionClient(null)}>{children}</TestProviders> } as never,
    );
    await screen.findByText('Lantern making in Hoi An');
    expect(api.outing.listMyOutings).not.toHaveBeenCalled();
    expect(screen.queryByTestId('outing-strip')).toBeNull();
  });
});
