import { create } from '@bufbuild/protobuf';
import {
  Code, ConnectError, ListVenueCitiesResponseSchema, ListVenuesResponseSchema, VenueCitySchema, VenueSchema, VenueStatus,
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

// Venue status changes are reversible (a venue can go back to review) and the
// page has no confirm step, so there are no cancel tests here.
describe.each([
  ['approve', 'Approve', VenueStatus.APPROVED],
  ['reject', 'Reject', VenueStatus.REJECTED],
] as const)('%s a venue', (_, label, status) => {
  it('sends the city, the place and the status, then drops it from the list', async () => {
    serve();
    const { user } = renderPage(<Venues />);
    await user.click((await rowOf('place-a')).getByRole('button', { name: label }));

    expect(fake.requests('curation.setVenueStatus')).toEqual([{ cityId: 'tokyo', placeIds: ['place-a'], status }]);
    expect(await screen.findByText('Cafe place-b')).toBeTruthy();
    expect(screen.queryByText('Cafe place-a')).toBeNull();
  });

  it('shows the error and keeps the venue when the RPC fails', async () => {
    serve();
    fake.on('curation.setVenueStatus', () => { throw new ConnectError('no connection', Code.Unavailable); });
    const { user } = renderPage(<Venues />);
    await user.click((await rowOf('place-a')).getByRole('button', { name: label }));

    expect(await screen.findByText(/no connection/)).toBeTruthy();
    expect(screen.getByText('Cafe place-a')).toBeTruthy();
  });

  it('sends one RPC for a double click', async () => {
    const state = serve();
    const gate = deferred();
    fake.on('curation.setVenueStatus', async () => { await gate.promise; state.decided.add('place-a'); return {}; });
    const { user } = renderPage(<Venues />);
    const button = (await rowOf('place-a')).getByRole('button', { name: label });
    await user.click(button);
    await user.click(button);
    gate.release();

    await vi.waitFor(() => expect(screen.queryByText('Cafe place-a')).toBeNull());
    expect(fake.requests('curation.setVenueStatus')).toHaveLength(1);
  });

  it('sends every selected place in one RPC', async () => {
    serve();
    const { user } = renderPage(<Venues />);
    await user.click(await screen.findByLabelText('Select all'));
    await user.click(screen.getByRole('button', { name: `${label} selected` }));

    expect(fake.requests('curation.setVenueStatus')).toEqual([{ cityId: 'tokyo', placeIds, status }]);
    expect(await screen.findByText('Nothing here.')).toBeTruthy();
  });
});
