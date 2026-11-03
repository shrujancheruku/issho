import { create } from '@bufbuild/protobuf';
import {
  ApplyCityPackResponseSchema, CityPackSummarySchema, ListCityPacksResponseSchema, ReplacedAreaSchema,
} from '@issho/api-client';
import { screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { fake } from '../test/fakeApi';
import { renderPage } from '../test/render';
import { Cities } from './Cities';

vi.mock('../lib', async () => ({ api: (await import('../test/fakeApi')).fake.api }));

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
});
