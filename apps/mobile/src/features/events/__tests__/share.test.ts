/// <reference types="jest" />
import { shareText } from '../share';

const t = (key: string, p: Record<string, string>) =>
  ({ 'share.intro': `I'm going to ${p.title}`, 'share.when': `When: ${p.date}`, 'share.where': `Where: ${p.place}`, 'share.map': `Map: ${p.url}` })[key] ?? key;

it('lists what, when, where, each stop and a map link', () => {
  const text = shareText(
    { title: 'Temples and shrines, then food tour' },
    'Sat, Oct 24, 2:00 PM',
    [
      { venueName: 'Koenji Tenso Shrine', venueAddress: 'Suginami City', venueMapsUrl: 'https://maps.example/1' },
      { venueName: '', venueAddress: '', venueMapsUrl: '' },
      { venueName: 'Misoichi', venueAddress: '', venueMapsUrl: 'https://maps.example/2' },
    ],
    'Koenji',
    t,
  );
  expect(text).toBe(
    [
      "I'm going to Temples and shrines, then food tour",
      'When: Sat, Oct 24, 2:00 PM',
      'Where: Koenji',
      '1. Koenji Tenso Shrine, Suginami City',
      '2. Misoichi',
      'Map: https://maps.example/1',
    ].join('\n'),
  );
});
