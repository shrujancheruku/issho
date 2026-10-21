/// <reference types="jest" />
import { TimeBlock } from '@issho/api-client';

import { slotKey } from '@/lib/slots';

import { initialTimes, toSignup, toggle, windowDates } from '../signup';

const usual = [slotKey(6, TimeBlock.AFTERNOON), slotKey(0, TimeBlock.MORNING)];

describe('windowDates', () => {
  it('lists the window across a month end', () => {
    expect(windowDates('2030-01-30', 4)).toEqual(['2030-01-30', '2030-01-31', '2030-02-01', '2030-02-02']);
  });
});

describe('initialTimes', () => {
  it('starts from the usual availability when not signed up', () => {
    expect(initialTimes(undefined, usual)).toEqual({ pattern: usual, blocked: [], keep: [] });
  });

  it('uses a saved signup, splitting whole days from parts of days', () => {
    const signup = {
      pattern: [{ weekday: 3, block: TimeBlock.EVENING }],
      unavailable: [
        { date: '2030-01-09', block: TimeBlock.UNSPECIFIED },
        { date: '2030-01-12', block: TimeBlock.EVENING },
      ],
    };
    const t = initialTimes(signup, usual);
    expect(t.pattern).toEqual([slotKey(3, TimeBlock.EVENING)]);
    expect(t.blocked).toEqual(['2030-01-09']);
    expect(t.keep).toEqual([{ date: '2030-01-12', block: TimeBlock.EVENING }]);
  });

  it('shows the usual availability for a signup that kept it', () => {
    expect(initialTimes({ pattern: [], unavailable: [] }, usual).pattern).toEqual(usual);
  });
});

describe('toSignup', () => {
  it('sends the usual availability as an empty pattern, in any order', () => {
    expect(toSignup({ pattern: [...usual].reverse(), blocked: [], keep: [] }, usual).pattern).toEqual([]);
  });

  it('sends a changed pattern and sorted blocked dates', () => {
    const s = toSignup({ pattern: [slotKey(3, TimeBlock.EVENING)], blocked: ['2030-01-10', '2030-01-08'], keep: [] }, usual);
    expect(s.pattern).toEqual([{ weekday: 3, block: TimeBlock.EVENING }]);
    expect(s.unavailable).toEqual([
      { date: '2030-01-08', block: TimeBlock.UNSPECIFIED },
      { date: '2030-01-10', block: TimeBlock.UNSPECIFIED },
    ]);
  });

  it('keeps parts of days unless the whole day is now blocked', () => {
    const keep = [
      { date: '2030-01-12', block: TimeBlock.EVENING },
      { date: '2030-01-13', block: TimeBlock.MORNING },
    ];
    const s = toSignup({ pattern: usual, blocked: ['2030-01-13'], keep }, usual);
    expect(s.unavailable).toEqual([
      { date: '2030-01-13', block: TimeBlock.UNSPECIFIED },
      { date: '2030-01-12', block: TimeBlock.EVENING },
    ]);
  });
});

it('toggles a value in a list', () => {
  expect(toggle(['a'], 'b')).toEqual(['a', 'b']);
  expect(toggle(['a', 'b'], 'a')).toEqual(['b']);
});
