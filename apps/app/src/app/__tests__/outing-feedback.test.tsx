import { Code, ConnectError } from '@connectrpc/connect';
import { OutingState, SeatState } from '@issho/api-client';
import { fireEvent, screen, waitFor, within } from '@testing-library/react-native';
import { renderRouter } from 'expo-router/testing-library';
import type { ReactNode } from 'react';

import * as format from '@/lib/format';
import { fakeApi } from '@/test-support/fake-api';
import { viWeekday } from '@/test-support/weekday';
import { aFeedback, anOuting } from '@/test-support/outing-fixtures';
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

// The plan ends at 12:30 in Da Nang on Wed 25 Nov (05:30 UTC); feedback runs to 2 Dec 05:30 UTC.
const AFTER = '2026-11-25T06:00:00Z';
const clock = (iso: string) => jest.setSystemTime(new Date(iso));
beforeEach(() => {
  jest.useFakeTimers({ doNotFake: ['nextTick', 'setImmediate', 'setTimeout', 'clearTimeout', 'setInterval', 'clearInterval', 'queueMicrotask'], now: new Date(AFTER) });
  jest.spyOn(format, 'deviceTimeZone').mockReturnValue('Asia/Ho_Chi_Minh');
});
afterEach(() => {
  jest.useRealTimers();
  jest.restoreAllMocks();
});

const going = (over = {}) => anOuting({ seatState: SeatState.GOING, outingState: OutingState.SET, yesCount: 3, othersGoing: ['Linh', 'Minh'], checkedIn: true, ...over });
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

function afterTheOuting(feedback = aFeedback()) {
  const api = fakeApi();
  api.outing.listMyOutings.mockResolvedValue({ outings: [going()] });
  api.outing.getFeedback.mockResolvedValue({ feedback });
  return api;
}

describe('after the plan\'s end, in Together', () => {
  it('puts a feedback card in "Needs you" with the day feedback closes, and counts it on the tab', async () => {
    const api = afterTheOuting();
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    expect(screen.getByText('Needs you')).toBeTruthy();
    expect(within(card).getByText('Lantern making in Hoi An')).toBeTruthy();
    expect(within(card).getByText('How did it go? Feedback closes Wed, Dec 2.')).toBeTruthy();
    expect(screen.getByLabelText('Together, 1 needs an answer')).toBeTruthy();
    expect(api.outing.getFeedback).toHaveBeenCalledWith({ outingId: 'o1' });
  });

  it("shows the device's time beside the closing day on the card and the row when the device is on another day", async () => {
    jest.spyOn(format, 'deviceTimeZone').mockReturnValue('America/Los_Angeles');
    const api = afterTheOuting();
    await open('/together', { api });
    const card = await screen.findByTestId('needs-you-o1');
    expect(within(card).getByText('How did it go? Feedback closes Wed, Dec 2.')).toBeTruthy();
    expect(within(card).getByText('Your time: Tue, Dec 1, 9:30 PM')).toBeTruthy();
  });

  it("shows the device's time beside the closing day on the form", async () => {
    jest.spyOn(format, 'deviceTimeZone').mockReturnValue('America/Los_Angeles');
    await open('/plans/p1', { api: afterTheOuting() });
    const strip = await screen.findByTestId('outing-strip');
    await within(strip).findByText('Did you go?');
    expect(within(strip).getByText('Feedback closes Wed, Dec 2.')).toBeTruthy();
    expect(within(strip).getByText('Your time: Tue, Dec 1, 9:30 PM')).toBeTruthy();
  });

  it('writes no second time when the device is on the same day', async () => {
    await open('/together', { api: afterTheOuting() });
    await screen.findByTestId('needs-you-o1');
    expect(screen.queryByText(/Your time/)).toBeNull();
  });

  it('opens the outing\'s page from "Give feedback"', async () => {
    await open('/together', { api: afterTheOuting() });
    const card = await screen.findByTestId('needs-you-o1');
    await fireEvent.press(within(card).getByRole('link', { name: 'Give feedback' }));
    await waitFor(() => expect(screen).toHavePathname('/plans/p1'));
  });

  it('asks for nothing before the plan ends, or of someone who is not going', async () => {
    clock('2026-11-25T05:29:00Z');
    const api = afterTheOuting();
    await open('/together', { api });
    await screen.findByTestId('together-row-o1');
    expect(screen.queryByTestId('needs-you-o1')).toBeNull();
    expect(api.outing.getFeedback).not.toHaveBeenCalled();
  });

  it('asks nothing of a person whose seat was cancelled', async () => {
    const api = afterTheOuting();
    api.outing.listMyOutings.mockResolvedValue({ outings: [going({ seatState: SeatState.CANCELLED })] });
    await open('/together', { api });
    await screen.findByTestId('together-row-o1');
    expect(screen.queryByTestId('needs-you-o1')).toBeNull();
    expect(api.outing.getFeedback).not.toHaveBeenCalled();
  });

  it('keeps the card off Together once feedback is given, and shows the day it closes in time order', async () => {
    const api = afterTheOuting(aFeedback({ answered: true, went: true }));
    api.signup.listMySignups.mockResolvedValue({
      signups: [
        { $typeName: 'issho.v1.Signup', planId: 'p1', ticks: [], hearBy: '2026-11-30' },
        { $typeName: 'issho.v1.Signup', planId: 'p1', ticks: [], hearBy: '2026-12-04' },
      ],
    });
    await open('/together', { api });
    const row = await screen.findByTestId('together-row-o1');
    expect(screen.queryByTestId('needs-you-o1')).toBeNull();
    expect(screen.queryByLabelText(/needs? an answer/)).toBeNull();
    expect(within(row).getByText('Feedback closes Wed, Dec 2.')).toBeTruthy();
    const links = screen.getAllByRole('link').map((l) => within(l).queryAllByText(/./).map((t) => t.props.children).flat().join(' '));
    expect(links).toEqual([expect.stringContaining('Nov 30'), expect.stringContaining('Dec 2'), expect.stringContaining('Dec 4')]);
  });

  it('shows the outing as muted once it is over', async () => {
    await open('/together', { api: afterTheOuting(aFeedback({ answered: true })) });
    expect(await screen.findByTestId('together-row-o1')).toHaveStyle({ opacity: 0.6 });
  });

  it('shows the same outing at full strength on the day', async () => {
    clock('2026-11-25T03:00:00Z');
    const api = afterTheOuting();
    await open('/together', { api });
    expect(await screen.findByTestId('together-row-o1')).not.toHaveStyle({ opacity: 0.6 });
  });

  it('still asks, and says the form could not be read, when GetFeedback fails', async () => {
    const api = afterTheOuting();
    api.outing.getFeedback.mockRejectedValue(new ConnectError('down', Code.Unavailable));
    await open('/together', { api });
    expect(await screen.findByTestId('needs-you-o1')).toBeTruthy();
  });

  it('reads in Vietnamese', async () => {
    await open('/together', { lang: 'vi', api: afterTheOuting() });
    const card = await screen.findByTestId('needs-you-o1');
    expect(within(card).getByText(`Buổi đi thế nào? Nhận ý kiến đến ${viWeekday('2026-12-02')}, 2 thg 12.`)).toBeTruthy();
    expect(within(card).getByRole('link', { name: 'Gửi ý kiến' })).toBeTruthy();
  });
});

describe('the feedback on the outing\'s page', () => {
  it('asks the three questions in order, with everyone ticked by default in "who else was there?"', async () => {
    await open('/plans/p1', { api: afterTheOuting() });
    const strip = await screen.findByTestId('outing-strip');
    await within(strip).findByText('Did you go?');
    await fireEvent.press(within(strip).getByRole('radio', { name: 'Yes' }));
    const headings = ['Did you go?', 'Who else was there?', 'Who would you meet again?'].map((q) => within(strip).getByText(q));
    const order = within(strip).getAllByText(/^(Did you go\?|Who else was there\?|Who would you meet again\?)$/).map((n) => n.props.children);
    expect(order).toEqual(['Did you go?', 'Who else was there?', 'Who would you meet again?']);
    expect(headings).toHaveLength(3);
    const there = within(within(strip).getByTestId('feedback-there'));
    expect(there.getByRole('checkbox', { name: 'Linh' })).toBeChecked();
    expect(there.getByRole('checkbox', { name: 'Minh' })).toBeChecked();
    expect(within(strip).getByText('Only you see this. Nobody is told who you pick.')).toBeTruthy();
  });

  it('does not ask the rest after a "no" to "did you go?", and saves only that', async () => {
    const api = afterTheOuting();
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    await within(strip).findByText('Did you go?');
    api.outing.saveFeedback.mockResolvedValue({ feedback: aFeedback({ answered: true, went: false }) });
    await fireEvent.press(within(strip).getByRole('radio', { name: 'No' }));
    expect(within(strip).queryByText('Who else was there?')).toBeNull();
    await fireEvent.press(within(strip).getByRole('button', { name: 'Save my answers' }));
    expect(api.outing.saveFeedback).toHaveBeenCalledWith({ outingId: 'o1', went: false, notThere: [], meetAgain: [] });
    expect(await within(strip).findByText('You said you did not go. You can change your answer until feedback closes.')).toBeTruthy();
  });

  it('cannot be saved until "did you go?" is answered', async () => {
    await open('/plans/p1', { api: afterTheOuting() });
    const strip = await screen.findByTestId('outing-strip');
    await within(strip).findByText('Did you go?');
    expect(within(strip).getByRole('button', { name: 'Save my answers' })).toBeDisabled();
  });

  it('saves who was not there and who the person would meet again, by handle', async () => {
    const api = afterTheOuting();
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    await within(strip).findByText('Did you go?');
    api.outing.saveFeedback.mockResolvedValue({ feedback: aFeedback({ answered: true, went: true, notThere: ['h-minh'], meetAgain: ['h-linh'] }) });
    await fireEvent.press(within(strip).getByRole('radio', { name: 'Yes' }));
    await fireEvent.press(within(within(strip).getByTestId('feedback-there')).getByRole('checkbox', { name: 'Minh' }));
    await fireEvent.press(within(within(strip).getByTestId('feedback-again')).getByRole('checkbox', { name: 'Linh' }));
    await fireEvent.press(within(strip).getByRole('button', { name: 'Save my answers' }));
    expect(api.outing.saveFeedback).toHaveBeenCalledWith({ outingId: 'o1', went: true, notThere: ['h-minh'], meetAgain: ['h-linh'] });
    expect(await within(strip).findByText('You said you went. You can change your answers until feedback closes.')).toBeTruthy();
  });

  it('offers only the people ticked as there to meet again, and drops a pick when its person is unticked', async () => {
    await open('/plans/p1', { api: afterTheOuting() });
    const strip = await screen.findByTestId('outing-strip');
    await within(strip).findByText('Did you go?');
    await fireEvent.press(within(strip).getByRole('radio', { name: 'Yes' }));
    const again = within(strip).getByTestId('feedback-again');
    await fireEvent.press(within(again).getByRole('checkbox', { name: 'Minh' }));
    expect(within(again).getByRole('checkbox', { name: 'Minh' })).toBeChecked();
    await fireEvent.press(within(within(strip).getByTestId('feedback-there')).getByRole('checkbox', { name: 'Minh' }));
    expect(within(within(strip).getByTestId('feedback-again')).queryByRole('checkbox', { name: 'Minh' })).toBeNull();
    await fireEvent.press(within(within(strip).getByTestId('feedback-there')).getByRole('checkbox', { name: 'Minh' }));
    // Ticked again, the pick is not back: the person chooses again.
    expect(within(within(strip).getByTestId('feedback-again')).getByRole('checkbox', { name: 'Minh' })).not.toBeChecked();
  });

  it('takes the card out of "Needs you" and the count once it is saved, and the answers can be reopened and changed', async () => {
    const api = afterTheOuting();
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    await within(strip).findByText('Did you go?');
    const saved = aFeedback({ answered: true, went: true, notThere: ['h-minh'], meetAgain: ['h-linh'] });
    api.outing.saveFeedback.mockResolvedValue({ feedback: saved });
    await fireEvent.press(within(strip).getByRole('radio', { name: 'Yes' }));
    await fireEvent.press(within(strip).getByRole('button', { name: 'Save my answers' }));
    await within(strip).findByText('You said you went. You can change your answers until feedback closes.');
    // Reopened with the answers given, until feedback closes.
    await fireEvent.press(within(strip).getByRole('button', { name: 'Change my answers' }));
    expect(within(within(strip).getByTestId('feedback-there')).getByRole('checkbox', { name: 'Minh' })).not.toBeChecked();
    expect(within(within(strip).getByTestId('feedback-there')).getByRole('checkbox', { name: 'Linh' })).toBeChecked();
    expect(within(within(strip).getByTestId('feedback-again')).getByRole('checkbox', { name: 'Linh' })).toBeChecked();
    api.outing.saveFeedback.mockResolvedValue({ feedback: aFeedback({ answered: true, went: true, notThere: [], meetAgain: [] }) });
    await fireEvent.press(within(within(strip).getByTestId('feedback-there')).getByRole('checkbox', { name: 'Minh' }));
    await fireEvent.press(within(within(strip).getByTestId('feedback-again')).getByRole('checkbox', { name: 'Linh' }));
    await fireEvent.press(within(strip).getByRole('button', { name: 'Save my answers' }));
    expect(api.outing.saveFeedback).toHaveBeenLastCalledWith({ outingId: 'o1', went: true, notThere: [], meetAgain: [] });
  });

  it('keeps the picks out of the address', async () => {
    const api = afterTheOuting();
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    await within(strip).findByText('Did you go?');
    api.outing.saveFeedback.mockResolvedValue({ feedback: aFeedback({ answered: true, went: true, meetAgain: ['h-linh'] }) });
    await fireEvent.press(within(strip).getByRole('radio', { name: 'Yes' }));
    await fireEvent.press(within(within(strip).getByTestId('feedback-again')).getByRole('checkbox', { name: 'Linh' }));
    await fireEvent.press(within(strip).getByRole('button', { name: 'Save my answers' }));
    await within(strip).findByText(/You said you went/);
    expect(screen).toHavePathname('/plans/p1');
    // Nothing but the plan's own id is in the address.
    expect(screen).toHaveSearchParams({ id: 'p1' });
    // The saved answers are not written back onto the page for anyone to read over a shoulder.
    expect(within(strip).queryByText(/h-linh/)).toBeNull();
  });

  it('says feedback has closed when it closed with the screen open, and the button works again', async () => {
    const api = afterTheOuting();
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    await within(strip).findByText('Did you go?');
    await fireEvent.press(within(strip).getByRole('radio', { name: 'No' }));
    clock('2026-12-02T05:30:00Z');
    api.outing.saveFeedback.mockRejectedValue(refused());
    await fireEvent.press(within(strip).getByRole('button', { name: 'Save my answers' }));
    expect(await within(strip).findByText('Feedback has closed, so your answers can no longer be saved.')).toBeTruthy();
    expect(within(strip).getByRole('button', { name: 'Save my answers' })).not.toBeDisabled();
  });

  it('says it went wrong for any other failure and lets the person try again', async () => {
    const api = afterTheOuting();
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    await within(strip).findByText('Did you go?');
    await fireEvent.press(within(strip).getByRole('radio', { name: 'No' }));
    api.outing.saveFeedback.mockRejectedValue(new ConnectError('down', Code.Unavailable));
    await fireEvent.press(within(strip).getByRole('button', { name: 'Save my answers' }));
    expect(await within(strip).findByText('We could not save your answers. Try again.')).toBeTruthy();
    await waitFor(() => expect(within(strip).getByRole('button', { name: 'Save my answers' })).not.toBeDisabled());
  });

  it('says the form could not be loaded, and tries again', async () => {
    const api = afterTheOuting();
    api.outing.getFeedback.mockRejectedValueOnce(new ConnectError('down', Code.Unavailable));
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    expect(await within(strip).findByText('Your feedback form could not be loaded.')).toBeTruthy();
    api.outing.getFeedback.mockResolvedValue({ feedback: aFeedback() });
    await fireEvent.press(within(strip).getByRole('button', { name: 'Try again' }));
    expect(await within(strip).findByText('Did you go?')).toBeTruthy();
  });

  it('says feedback has closed once it has, and asks for nothing', async () => {
    clock('2026-12-02T05:30:00Z');
    const api = afterTheOuting();
    await open('/plans/p1', { api });
    const strip = await screen.findByTestId('outing-strip');
    expect(within(strip).getByText('Feedback has closed.')).toBeTruthy();
    expect(within(strip).queryByText('Did you go?')).toBeNull();
    expect(api.outing.getFeedback).not.toHaveBeenCalled();
    expect(strip).toHaveStyle({ opacity: 0.6 });
  });

  it('is muted around the form once the outing is over, with the form at full strength', async () => {
    await open('/plans/p1', { api: afterTheOuting() });
    const strip = await screen.findByTestId('outing-strip');
    await within(strip).findByText('Did you go?');
    expect(strip).not.toHaveStyle({ opacity: 0.6 });
    expect(within(strip).getByTestId('outing-summary')).toHaveStyle({ opacity: 0.6 });
  });

  it('reads in Vietnamese', async () => {
    await open('/plans/p1', { lang: 'vi', api: afterTheOuting() });
    const strip = await screen.findByTestId('outing-strip');
    expect(await within(strip).findByText('Bạn có đi không?')).toBeTruthy();
  });
});
