import { JoinLinkProblemReason, JoinLinkState } from '@issho/api-client';
import { fireEvent, render, screen, within } from '@testing-library/react-native';
import { Platform, Share } from 'react-native';

import { fakeApi } from '@/test-support/fake-api';
import { joinLinkRefusal } from '@/test-support/problems';
import { TestProviders } from '@/test-support/providers';

import YouScreen from '../(tabs)/you';

jest.mock('react-native/Libraries/Utilities/useColorScheme', () => ({
  __esModule: true,
  default: jest.fn(() => 'light'),
}));

// The scheme comes from app.json's manifest, which Jest has no copy of.
jest.mock('expo-linking', () => ({ createURL: (path: string) => `issho://${path}` }));

const at = (iso: string) => ({ seconds: BigInt(Math.floor(Date.parse(iso) / 1000)), nanos: 0 });
const link = (id: string, code: string, state: JoinLinkState, expires = '2026-12-06T03:00:00Z') => ({
  id,
  code,
  state,
  createdAt: at('2026-11-22T03:00:00Z'),
  expiresAt: at(expires),
});

async function you(api = fakeApi(), lang: 'en' | 'vi' = 'en') {
  await render(
    <TestProviders lang={lang} api={api}>
      <YouScreen />
    </TestProviders>,
  );
  return api;
}

describe("the You tab's join links", () => {
  it("lists the member's links by state, and how many they can still make", async () => {
    const api = fakeApi();
    api.joinLink.listMyJoinLinks.mockResolvedValue({
      links: [
        link('l1', 'code-one', JoinLinkState.OUTSTANDING),
        link('l2', 'code-two', JoinLinkState.USED),
        link('l3', 'code-three', JoinLinkState.EXPIRED, '2026-11-20T03:00:00Z'),
      ],
      remaining: 0,
    });
    await you(api);
    expect(await screen.findByText('Not used yet. Expires Sun, Dec 6.')).toBeTruthy();
    expect(screen.getByText('Used')).toBeTruthy();
    expect(screen.getByText('Expired')).toBeTruthy();
    // Only a link that can still admit someone can be shared or cancelled.
    expect(screen.getAllByRole('button', { name: 'Share' })).toHaveLength(1);
    expect(screen.getAllByRole('button', { name: 'Cancel link' })).toHaveLength(1);
    expect(screen.getByText('You have made all 3 of your join links.')).toBeTruthy();
    expect(screen.queryByRole('button', { name: 'Make a join link' })).toBeNull();
  });

  it('makes a link and lists it', async () => {
    const api = fakeApi();
    api.joinLink.listMyJoinLinks
      .mockResolvedValueOnce({ links: [], remaining: 3 })
      .mockResolvedValue({ links: [link('l1', 'fresh', JoinLinkState.OUTSTANDING)], remaining: 2 });
    api.joinLink.createJoinLink.mockResolvedValue({ link: link('l1', 'fresh', JoinLinkState.OUTSTANDING) });
    await you(api);
    expect(await screen.findByText('You can make 3 more.')).toBeTruthy();
    await fireEvent.press(screen.getByRole('button', { name: 'Make a join link' }));
    expect(await screen.findByText('Not used yet. Expires Sun, Dec 6.')).toBeTruthy();
    expect(screen.getByText('You can make 2 more.')).toBeTruthy();
    expect(api.joinLink.createJoinLink).toHaveBeenCalledTimes(1);
  });

  it('cancels a link, which then returns to the three', async () => {
    const api = fakeApi();
    api.joinLink.listMyJoinLinks
      .mockResolvedValueOnce({ links: [link('l1', 'c1', JoinLinkState.OUTSTANDING)], remaining: 2 })
      .mockResolvedValue({ links: [], remaining: 3 });
    await you(api);
    await fireEvent.press(await screen.findByRole('button', { name: 'Cancel link' }));
    expect(await screen.findByText('You can make 3 more.')).toBeTruthy();
    expect(api.joinLink.cancelJoinLink).toHaveBeenCalledWith({ id: 'l1' });
    expect(screen.queryByText('Not used yet. Expires Sun, Dec 6.')).toBeNull();
  });

  it('shares a link by the phone’s share sheet', async () => {
    const share = jest.spyOn(Share, 'share').mockResolvedValue({ action: 'sharedAction', activityType: undefined });
    const api = fakeApi();
    api.joinLink.listMyJoinLinks.mockResolvedValue({ links: [link('l1', 'code-one', JoinLinkState.OUTSTANDING)], remaining: 2 });
    await you(api);
    await fireEvent.press(await screen.findByRole('button', { name: 'Share' }));
    expect(share).toHaveBeenCalledTimes(1);
    expect(share.mock.calls[0][0].message).toMatch(/^Join me on Issho: \S+\/join\/code-one$/);
    share.mockRestore();
  });

  it('copies the link on the web, where there is no share sheet, and says so', async () => {
    const writeText = jest.fn().mockResolvedValue(undefined);
    Object.defineProperty(globalThis.navigator, 'clipboard', { value: { writeText }, configurable: true });
    jest.replaceProperty(Platform, 'OS', 'web');
    const api = fakeApi();
    api.joinLink.listMyJoinLinks.mockResolvedValue({ links: [link('l1', 'code-one', JoinLinkState.OUTSTANDING)], remaining: 2 });
    await you(api);
    await fireEvent.press(await screen.findByRole('button', { name: 'Share' }));
    expect(await screen.findByText('Link copied.')).toBeTruthy();
    expect(writeText).toHaveBeenCalledWith(expect.stringMatching(/\/join\/code-one$/));
    jest.restoreAllMocks();
  });

  it('says so when no links are left or the member has not been admitted', async () => {
    const api = fakeApi();
    api.joinLink.createJoinLink.mockRejectedValueOnce(joinLinkRefusal(JoinLinkProblemReason.NONE_LEFT));
    await you(api);
    await fireEvent.press(await screen.findByRole('button', { name: 'Make a join link' }));
    expect(await screen.findByText('You have made all 3 of your join links.')).toBeTruthy();

    api.joinLink.createJoinLink.mockRejectedValueOnce(joinLinkRefusal(JoinLinkProblemReason.NOT_ADMITTED));
    await fireEvent.press(screen.getByRole('button', { name: 'Make a join link' }));
    expect(await screen.findByText('You can make join links once you have joined with one.')).toBeTruthy();
  });

  it('says so when the links could not be loaded, and tries again', async () => {
    const api = fakeApi();
    api.joinLink.listMyJoinLinks.mockRejectedValueOnce(new Error('network'));
    await you(api);
    expect(await screen.findByText('Your join links could not be loaded.')).toBeTruthy();
    await fireEvent.press(screen.getByRole('button', { name: 'Try again' }));
    expect(await screen.findByText('You can make 3 more.')).toBeTruthy();
  });

  it('reads in Vietnamese', async () => {
    const api = fakeApi();
    api.joinLink.listMyJoinLinks.mockResolvedValue({ links: [link('l1', 'c1', JoinLinkState.USED)], remaining: 2 });
    await you(api, 'vi');
    expect(await screen.findByText('Đã dùng')).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Tạo liên kết tham gia' })).toBeTruthy();
    expect(within(screen.getByTestId('join-links')).getByText('Liên kết tham gia của bạn')).toBeTruthy();
  });
});
