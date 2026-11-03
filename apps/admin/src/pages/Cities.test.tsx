import { create, type MessageInitShape } from '@bufbuild/protobuf';
import {
  AdminJobFailureSchema, AdminJobState, ApplyCityPackResponseSchema, CityPackSummarySchema, Code, ConnectError, DiscoverAreaVenuesResponseSchema,
  GetAdminJobResponseSchema, ListCityPacksResponseSchema, ReplacedAreaSchema,
} from '@issho/api-client';
import { screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { fake } from '../test/fakeApi';
import { renderPage } from '../test/render';
import { Cities } from './Cities';

vi.mock('../lib', async () => ({ api: (await import('../test/fakeApi')).fake.api }));

function serve(applied: ReturnType<typeof create<typeof ApplyCityPackResponseSchema>>) {
  fake
    .on('cityPacks.listCityPacks', () => create(ListCityPacksResponseSchema, {
      configured: true,
      packs: [create(CityPackSummarySchema, { id: 'tokyo', name: 'Tokyo', inRepo: true, seeded: true, areas: 3 })],
    }))
    .on('cityPacks.applyCityPack', () => applied);
}

describe('applying a city pack', () => {
  it('lists what the authoritative apply removed and moved', async () => {
    fake
      .on('cityPacks.listCityPacks', () => create(ListCityPacksResponseSchema, {
        configured: true,
        packs: [create(CityPackSummarySchema, { id: 'tokyo', name: 'Tokyo', inRepo: true, seeded: true, areas: 3 })],
      }))
      .on('cityPacks.applyCityPack', () => create(ApplyCityPackResponseSchema, {
        removedAreas: ['tokyo-gone'],
        replacedAreas: [create(ReplacedAreaSchema, { oldAreaId: 'tokyo-old', newAreaId: 'tokyo-new', people: 4, crews: 2, plans: 7 })],
        removedOverrides: ['ramen'],
      }));
    const { user } = renderPage(<Cities />);
    await user.click(await screen.findByRole('button', { name: 'Apply from main' }));

    expect(await screen.findByText('Removed areas: tokyo-gone.')).toBeTruthy();
    expect(screen.getByText('Replaced tokyo-old with tokyo-new: moved 4 people, 2 crews, 7 plans.')).toBeTruthy();
    expect(screen.getByText('Removed activity overrides: ramen.')).toBeTruthy();
    expect(fake.requests('cityPacks.applyCityPack')).toEqual([{ cityId: 'tokyo' }]);
  });

  it('uses singular forms for one row', async () => {
    serve(create(ApplyCityPackResponseSchema, {
      replacedAreas: [create(ReplacedAreaSchema, { oldAreaId: 'a', newAreaId: 'b', people: 1, crews: 1, plans: 1 })],
    }));
    const { user } = renderPage(<Cities />);
    await user.click(await screen.findByRole('button', { name: 'Apply from main' }));
    expect(await screen.findByText('Replaced a with b: moved 1 person, 1 crew, 1 plan.')).toBeTruthy();
  });

  it('shows none of the removal lines when nothing was removed', async () => {
    serve(create(ApplyCityPackResponseSchema, { newAreas: ['tokyo-x'] }));
    const { user } = renderPage(<Cities />);
    await user.click(await screen.findByRole('button', { name: 'Apply from main' }));
    expect(await screen.findByText(/^Applied; new areas tokyo-x/)).toBeTruthy();
    expect(screen.queryByText(/Removed areas|Replaced |Removed activity overrides/)).toBeNull();
  });
});

// The apply and the venue search are jobs: the page shows "Running…" and asks
// GetAdminJob every 2 s until the job ends. Timers are faked and advanced
// by hand; real time still passes (shouldAdvanceTime) so user-event works.
describe('long operations run as jobs', () => {
  beforeEach(() => {
    fake.reset();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });
  afterEach(() => vi.useRealTimers());

  const job = (state: AdminJobState, extra: Pick<MessageInitShape<typeof GetAdminJobResponseSchema>, 'result' | 'failure'> = {}) =>
    create(GetAdminJobResponseSchema, { jobId: 5n, state, ...extra });
  const list = (areasWithoutVenues: string[] = []) => create(ListCityPacksResponseSchema, {
    configured: true,
    packs: [create(CityPackSummarySchema, { id: 'tokyo', name: 'Tokyo', inRepo: true, seeded: true, areas: 3, areasWithoutVenues })],
  });
  const polls = () => fake.requests('admin.getAdminJob').length;

  it('shows Running…, polls every 2 s, then the report, and stops', async () => {
    let state = AdminJobState.QUEUED;
    fake
      .on('cityPacks.listCityPacks', () => list())
      .on('cityPacks.applyCityPack', () => create(ApplyCityPackResponseSchema, { jobId: 5n }))
      .on('admin.getAdminJob', () => (state === AdminJobState.SUCCEEDED
        ? job(state, { result: { case: 'applyCityPack', value: create(ApplyCityPackResponseSchema, { newAreas: ['tokyo-x'], travelTimesRebuilt: true, travelTimes: 12 }) } })
        : job(state)));
    const { user } = renderPage(<Cities />);
    await user.click(await screen.findByRole('button', { name: 'Apply from main' }));

    const running = await screen.findByRole('button', { name: 'Running…' });
    expect((running as HTMLButtonElement).disabled).toBe(true);
    expect(fake.requests('cityPacks.applyCityPack')).toEqual([{ cityId: 'tokyo' }]);
    expect(fake.requests('admin.getAdminJob')[0]).toEqual({ jobId: 5n });
    expect(screen.queryByText(/^Applied/)).toBeNull();

    // Still running after two seconds: asked again, nothing shown yet.
    const before = polls();
    state = AdminJobState.RUNNING;
    await vi.advanceTimersByTimeAsync(2000);
    expect(polls()).toBeGreaterThan(before);
    expect(screen.getByRole('button', { name: 'Running…' })).toBeTruthy();

    state = AdminJobState.SUCCEEDED;
    await vi.advanceTimersByTimeAsync(2000);
    expect(await screen.findByText(/^Applied; new areas tokyo-x; 12 travel times rebuilt/)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Apply from main' })).toBeTruthy();
    // The list was refetched for the new state, and polling stopped.
    expect(fake.requests('cityPacks.listCityPacks').length).toBeGreaterThanOrEqual(2);
    const after = polls();
    await vi.advanceTimersByTimeAsync(6000);
    expect(polls()).toBe(after);
  });

  it('shows why a job failed', async () => {
    fake
      .on('cityPacks.listCityPacks', () => list())
      .on('cityPacks.applyCityPack', () => create(ApplyCityPackResponseSchema, { jobId: 5n }))
      .on('admin.getAdminJob', () => job(AdminJobState.FAILED, {
        failure: create(AdminJobFailureSchema, { message: 'pack not applied: area tokyo-b: 1 people' }),
      }));
    const { user } = renderPage(<Cities />);
    await user.click(await screen.findByRole('button', { name: 'Apply from main' }));

    expect(await screen.findByText('Failed: pack not applied: area tokyo-b: 1 people')).toBeTruthy();
    expect(screen.queryByText(/^Applied/)).toBeNull();
    expect(screen.getByRole('button', { name: 'Apply from main' })).toBeTruthy();
    const after = polls();
    await vi.advanceTimersByTimeAsync(4000);
    expect(polls()).toBe(after);
  });

  it('follows the venue search the same way', async () => {
    let state = AdminJobState.RUNNING;
    fake
      .on('cityPacks.listCityPacks', () => list(['tokyo-a']))
      .on('cityPacks.discoverAreaVenues', () => create(DiscoverAreaVenuesResponseSchema, { jobId: 5n }))
      .on('admin.getAdminJob', () => (state === AdminJobState.SUCCEEDED
        ? job(state, { result: { case: 'discoverAreaVenues', value: create(DiscoverAreaVenuesResponseSchema, { searches: 9, venues: 31 }) } })
        : job(state)));
    const { user } = renderPage(<Cities />);
    await user.click(await screen.findByRole('button', { name: 'Find venues' }));

    expect(await screen.findByText(/^Running…/)).toBeTruthy();
    expect(fake.requests('cityPacks.discoverAreaVenues')).toEqual([{ cityId: 'tokyo', areaId: 'tokyo-a' }]);
    state = AdminJobState.SUCCEEDED;
    await vi.advanceTimersByTimeAsync(2000);
    expect(await screen.findByText('31 venues from 9 searches.')).toBeTruthy();
  });

  it('treats a failed poll as still running, and shows the error only once it persists', async () => {
    let calls = 0;
    fake
      .on('cityPacks.listCityPacks', () => list())
      .on('cityPacks.applyCityPack', () => create(ApplyCityPackResponseSchema, { jobId: 5n }))
      .on('admin.getAdminJob', () => {
        calls += 1;
        throw new ConnectError('upstream hiccup', Code.Unavailable);
      });
    const { user } = renderPage(<Cities />);
    await user.click(await screen.findByRole('button', { name: 'Apply from main' }));

    // Two failed polls: still Running…, nothing on screen about it.
    await vi.waitFor(() => expect(calls).toBe(1));
    await vi.advanceTimersByTimeAsync(2000);
    expect(calls).toBe(2);
    expect(screen.getByRole('button', { name: 'Running…' })).toBeTruthy();
    expect(screen.queryByText(/upstream hiccup/)).toBeNull();
    // The third in a row is shown, and the button is back.
    await vi.advanceTimersByTimeAsync(2000);
    expect(await screen.findByText(/upstream hiccup/)).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Apply from main' })).toBeTruthy();
  });

  it('a blip between good polls never shows an error', async () => {
    let calls = 0;
    fake
      .on('cityPacks.listCityPacks', () => list())
      .on('cityPacks.applyCityPack', () => create(ApplyCityPackResponseSchema, { jobId: 5n }))
      .on('admin.getAdminJob', () => {
        calls += 1;
        if (calls === 2 || calls === 3) throw new ConnectError('blip', Code.Unavailable);
        return job(calls < 5 ? AdminJobState.RUNNING : AdminJobState.SUCCEEDED, calls < 5 ? {} : {
          result: { case: 'applyCityPack', value: create(ApplyCityPackResponseSchema, { newAreas: ['tokyo-x'] }) },
        });
      });
    const { user } = renderPage(<Cities />);
    await user.click(await screen.findByRole('button', { name: 'Apply from main' }));
    for (let i = 0; i < 5; i += 1) {
      await vi.advanceTimersByTimeAsync(2000);
      expect(screen.queryByText(/blip/)).toBeNull();
    }
    expect(await screen.findByText(/^Applied; new areas tokyo-x/)).toBeTruthy();
  });

  it('shows the error when the call itself is refused, with no job to follow', async () => {
    fake
      .on('cityPacks.listCityPacks', () => list())
      .on('cityPacks.applyCityPack', () => { throw new ConnectError('timezone "Mars/Olympus": an IANA name like Asia/Tokyo', Code.InvalidArgument); });
    const { user } = renderPage(<Cities />);
    await user.click(await screen.findByRole('button', { name: 'Apply from main' }));
    expect(await screen.findByText(/timezone "Mars\/Olympus"/)).toBeTruthy();
    expect(polls()).toBe(0);
  });
});
