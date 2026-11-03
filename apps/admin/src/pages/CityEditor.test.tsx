import { create } from '@bufbuild/protobuf';
import {
  CityPackPullRequestSchema, CityPackSchema, GetCityPackResponseSchema, PackAreaSchema, ProposeCityPackResponseSchema,
} from '@issho/api-client';
import { screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { fake } from '../test/fakeApi';
import { renderPage } from '../test/render';
import { CityEditor } from './CityEditor';

vi.mock('../lib', async () => ({ api: (await import('../test/fakeApi')).fake.api }));

const pack = create(CityPackSchema, {
  id: 'tokyo', name: { en: 'Tokyo' }, timezone: 'Asia/Tokyo', currency: 'JPY', languages: ['ja', 'en'], travelModes: ['walk'],
  areas: [create(PackAreaSchema, { id: 'tokyo-a', name: { en: 'Alpha' }, lat: 35.7, lng: 139.6 })],
});
const pr = create(CityPackPullRequestSchema, { number: 42, title: 'feat(data): update the Tokyo city pack', url: 'https://github.com/o/r/pull/42' });

describe('editing a city pack', () => {
  beforeEach(() => fake.reset());
  afterEach(() => fake.reset());

  it('opens a pull request from the default branch when none is pending', async () => {
    fake
      .on('cityPacks.getCityPack', () => create(GetCityPackResponseSchema, { pack }))
      .on('cityPacks.proposeCityPack', () => create(ProposeCityPackResponseSchema, { pullRequest: pr }));
    const { user } = renderPage(<CityEditor cityId="tokyo" />);
    await screen.findByRole('heading', { name: 'Edit Tokyo' });
    expect(screen.queryByText(/Editing a pending change/)).toBeNull();

    await user.click(screen.getByRole('button', { name: 'Open a pull request' }));
    expect(await screen.findByRole('heading', { name: 'Pull request opened' })).toBeTruthy();
  });

  it('shows it is editing a pending change and saves to that pull request', async () => {
    fake
      .on('cityPacks.getCityPack', () => create(GetCityPackResponseSchema, { pack, pullRequest: pr }))
      .on('cityPacks.proposeCityPack', () => create(ProposeCityPackResponseSchema, { pullRequest: pr }));
    const { user } = renderPage(<CityEditor cityId="tokyo" />);

    expect(await screen.findByText(/Editing a pending change/)).toBeTruthy();
    const link = screen.getByRole('link', { name: 'pull request #42' });
    expect(link.getAttribute('href')).toBe('https://github.com/o/r/pull/42');
    expect(screen.queryByRole('button', { name: 'Open a pull request' })).toBeNull();

    await user.click(screen.getByRole('button', { name: 'Save to the pull request' }));
    expect(await screen.findByRole('heading', { name: 'Added to the pull request' })).toBeTruthy();
    expect(screen.getByRole('link', { name: /#42 feat\(data\): update the Tokyo city pack/ })).toBeTruthy();
    const [sent] = fake.requests('cityPacks.proposeCityPack') as { pack: { id: string; areas: { id: string }[] } }[];
    expect(fake.requests('cityPacks.proposeCityPack')).toHaveLength(1);
    expect(sent.pack.id).toBe('tokyo');
    expect(sent.pack.areas.map((a) => a.id)).toEqual(['tokyo-a']);
  });
});
