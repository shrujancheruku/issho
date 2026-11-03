import { create } from '@bufbuild/protobuf';
import {
  Code, ConnectError, ListSuspendedResponseSchema, SearchPeopleResponseSchema, SuspendedPersonSchema, PersonSummarySchema,
} from '@issho/api-client';
import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { fake } from '../test/fakeApi';
import { renderPage } from '../test/render';
import { People } from './People';

vi.mock('../lib', async () => ({ api: (await import('../test/fakeApi')).fake.api }));

const userId = '11111111-1111-1111-1111-111111111111';

// People only reads: suspending and lifting happen on the person's page
// (Person.test.tsx), which refreshes this list. These check that list.
describe('People', () => {
  it('lists who is suspended now, with a link to each person', async () => {
    fake.on('moderation.listSuspended', () => create(ListSuspendedResponseSchema, { people: [create(SuspendedPersonSchema, { userId, name: 'Aoi', reason: 'spam' })] }));
    renderPage(<People />);

    const link = await screen.findByRole('link', { name: 'Aoi' });
    expect(link.getAttribute('href')).toBe(`#/person/${userId}`);
    expect(screen.getByText('spam')).toBeTruthy();
    expect(fake.requests('moderation.searchPeople')).toEqual([]); // no search under two characters
  });

  it('says nobody is suspended', async () => {
    fake.on('moderation.listSuspended', () => create(ListSuspendedResponseSchema));
    renderPage(<People />);
    expect(await screen.findByText('Nobody is suspended.')).toBeTruthy();
  });

  it('searches after a pause in typing and marks suspended results', async () => {
    fake
      .on('moderation.listSuspended', () => create(ListSuspendedResponseSchema))
      .on('moderation.searchPeople', () => create(SearchPeopleResponseSchema, { people: [create(PersonSummarySchema, { userId, name: 'Aoi', email: 'aoi@example.com', suspended: true })] }));
    const { user } = renderPage(<People />);
    await user.type(screen.getByPlaceholderText('Name, email or user id'), 'aoi');

    expect(await screen.findByText('aoi@example.com')).toBeTruthy();
    expect(fake.requests('moderation.searchPeople')).toEqual([{ query: 'aoi' }]);
    expect(screen.getAllByText('Suspended').length).toBeGreaterThan(0);
  });

  it('shows the error when the list fails', async () => {
    fake.on('moderation.listSuspended', () => { throw new ConnectError('not an admin', Code.PermissionDenied); });
    renderPage(<People />);
    expect(await screen.findByText(/not an admin/)).toBeTruthy();
  });
});
