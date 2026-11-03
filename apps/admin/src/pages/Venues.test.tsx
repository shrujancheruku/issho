import { create } from '@bufbuild/protobuf';
import {
  Code, ConnectError, FoundVenueSchema, ListVenueCitiesResponseSchema, ListVenuesResponseSchema, SearchVenuesResponseSchema,
  VenueCitySchema, VenueSchema, VenueStatus,
} from '@issho/api-client';
import { screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { deferred } from '../test/deferred';
import { fake } from '../test/fakeApi';
import { renderPage } from '../test/render';
import { Venues } from './Venues';

vi.mock('../lib', async () => ({ api: (await import('../test/fakeApi')).fake.api }));

const placeIds = ['place-a', 'place-b'];

/** Two venues to review; setVenueStatus moves them out of the "to review" tab. */
function serve() {
  const state = { decided: new Set<string>() };
  fake
    .on('curation.listVenueCities', () => create(ListVenueCitiesResponseSchema, { cities: [create(VenueCitySchema, { id: 'tokyo', name: 'Tokyo', candidates: 2 })] }))
    .on('curation.listVenues', () => {
      const venues = placeIds.filter((id) => !state.decided.has(id))
        .map((placeId) => create(VenueSchema, { placeId, name: `Cafe ${placeId}`, status: VenueStatus.CANDIDATE }));
      return create(ListVenuesResponseSchema, { venues, total: venues.length });
    })
    .on('curation.setVenueStatus', (req: { placeIds: string[] }) => { req.placeIds.forEach((id) => state.decided.add(id)); return {}; });
  return state;
}

const rowOf = async (placeId: string) => within((await screen.findByText(`Cafe ${placeId}`)).closest('tr')!);

// In the review list every status change asks first (ConfirmButton), and the
// bulk buttons ask once for the whole selection, naming how many.
describe.each([
  ['approve', 'Approve', 'Yes, approve Cafe place-a', 'Cancel approving Cafe place-a', 'Yes, approve 2 venues', VenueStatus.APPROVED],
  ['reject', 'Reject', 'Yes, reject Cafe place-a', 'Cancel rejecting Cafe place-a', 'Yes, reject 2 venues', VenueStatus.REJECTED],
] as const)('%s a venue', (_, label, yes, cancel, yesBulk, status) => {
  it('sends the city, the place and the status once confirmed, then drops it from the list', async () => {
    serve();
    const { user } = renderPage(<Venues />);
    const row = await rowOf('place-a');
    await user.click(row.getByRole('button', { name: label }));
    expect(fake.requests('curation.setVenueStatus')).toEqual([]); // the first click only asks
    await user.click(row.getByRole('button', { name: yes }));

    expect(fake.requests('curation.setVenueStatus')).toEqual([{ cityId: 'tokyo', placeIds: ['place-a'], status }]);
    expect(await screen.findByText('Cafe place-b')).toBeTruthy();
    expect(screen.queryByText('Cafe place-a')).toBeNull();
  });

  it('sends nothing when the confirmation is cancelled', async () => {
    serve();
    const { user } = renderPage(<Venues />);
    const row = await rowOf('place-a');
    await user.click(row.getByRole('button', { name: label }));
    await user.click(row.getByRole('button', { name: cancel }));

    expect(fake.requests('curation.setVenueStatus')).toEqual([]);
    expect(row.getByRole('button', { name: label })).toBeTruthy();
    expect(screen.getByText('Cafe place-a')).toBeTruthy();
  });

  it('shows the error and keeps the venue when the RPC fails', async () => {
    serve();
    fake.on('curation.setVenueStatus', () => { throw new ConnectError('no connection', Code.Unavailable); });
    const { user } = renderPage(<Venues />);
    const row = await rowOf('place-a');
    await user.click(row.getByRole('button', { name: label }));
    await user.click(row.getByRole('button', { name: yes }));

    expect(await screen.findByText(/no connection/)).toBeTruthy();
    expect(screen.getByText('Cafe place-a')).toBeTruthy();
  });

  it('sends one RPC for a double click on the confirmation', async () => {
    const state = serve();
    const gate = deferred();
    fake.on('curation.setVenueStatus', async () => { await gate.promise; state.decided.add('place-a'); return {}; });
    const { user } = renderPage(<Venues />);
    const row = await rowOf('place-a');
    await user.click(row.getByRole('button', { name: label }));
    await user.click(row.getByRole('button', { name: yes }));
    expect(row.queryByRole('button', { name: yes })).toBeNull(); // folded back
    const again = row.getByRole('button', { name: label });
    expect((again as HTMLButtonElement).disabled).toBe(true); // the RPC hasn't answered yet
    await user.click(again);
    gate.release();

    await vi.waitFor(() => expect(screen.queryByText('Cafe place-a')).toBeNull());
    expect(fake.requests('curation.setVenueStatus')).toHaveLength(1);
  });

  it('asks once for a whole selection, naming the count, and sends every place in one RPC', async () => {
    serve();
    const { user } = renderPage(<Venues />);
    await user.click(await screen.findByLabelText('Select all'));
    await user.click(screen.getByRole('button', { name: `${label} selected` }));
    expect(fake.requests('curation.setVenueStatus')).toEqual([]);
    await user.click(screen.getByRole('button', { name: yesBulk }));

    expect(fake.requests('curation.setVenueStatus')).toEqual([{ cityId: 'tokyo', placeIds, status }]);
    expect(await screen.findByText('Nothing here.')).toBeTruthy();
  });

  it('sends nothing when the confirmation of a whole selection is cancelled', async () => {
    serve();
    const { user } = renderPage(<Venues />);
    await user.click(await screen.findByLabelText('Select all'));
    await user.click(screen.getByRole('button', { name: `${label} selected` }));
    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(fake.requests('curation.setVenueStatus')).toEqual([]);
    expect(screen.getByText('Cafe place-a')).toBeTruthy();
    expect(screen.getByText('Cafe place-b')).toBeTruthy();
  });
});

// Search results: these are reversible status sets and stay first-click.
describe('venue search results', () => {
  async function search() {
    serve();
    fake.on('curation.searchVenues', () => create(SearchVenuesResponseSchema, {
      venues: [
        create(FoundVenueSchema, { placeId: 'found-known', name: 'Known bar', known: true, status: VenueStatus.CANDIDATE }),
        create(FoundVenueSchema, { placeId: 'found-new', name: 'New bar', known: false, addableCategoryIds: ['bar'] }),
      ],
    }));
    const page = renderPage(<Venues />);
    await page.user.type(await screen.findByPlaceholderText(/Find places by name/), 'bar');
    await page.user.click(screen.getByRole('button', { name: 'Search' }));
    await screen.findByText('Known bar');
    return page.user;
  }

  it('approves a known place at the first click', async () => {
    const user = await search();
    fake.on('curation.setVenueStatus', () => ({}));
    await user.click(within((await screen.findByText('Known bar')).closest('tr')!).getByRole('button', { name: 'Approve' }));

    expect(fake.requests('curation.setVenueStatus')).toEqual([{ cityId: 'tokyo', placeIds: ['found-known'], status: VenueStatus.APPROVED }]);
    expect(fake.requests('curation.searchVenues')).toHaveLength(2); // the search runs again so statuses are current
  });

  it('rejects a known place at the first click', async () => {
    const user = await search();
    fake.on('curation.setVenueStatus', () => ({}));
    await user.click(within((await screen.findByText('Known bar')).closest('tr')!).getByRole('button', { name: 'Reject' }));

    expect(fake.requests('curation.setVenueStatus')).toEqual([{ cityId: 'tokyo', placeIds: ['found-known'], status: VenueStatus.REJECTED }]);
  });

  it('adds a missing place, approved, in the chosen kind', async () => {
    const user = await search();
    fake.on('curation.addVenue', () => ({}));
    await user.click(within((await screen.findByText('New bar')).closest('tr')!).getByRole('button', { name: 'Add, approved' }));

    expect(fake.requests('curation.addVenue')).toEqual([{ cityId: 'tokyo', placeId: 'found-new', categoryId: 'bar' }]);
  });

  it('shows the error when approving a found place fails', async () => {
    const user = await search();
    fake.on('curation.setVenueStatus', () => { throw new ConnectError('no connection', Code.Unavailable); });
    await user.click(within((await screen.findByText('Known bar')).closest('tr')!).getByRole('button', { name: 'Approve' }));

    expect(await screen.findByText(/no connection/)).toBeTruthy();
  });
});
