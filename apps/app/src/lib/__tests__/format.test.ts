import { cityDateKey, formatCalendarDate, formatDate, formatDateTime, formatInstantDay, formatInstantWhen, formatOutingWhen, formatPrice, formatTime, formatTimeRange } from '../format';
import { viWeekday } from '@/test-support/weekday';

// 17:30 UTC on 21 November is already 00:30 on the 22nd in Da Nang (UTC+7, no
// daylight saving), so each expectation also shows the city's clock, not the
// device's, deciding the day.
const instant = new Date('2026-11-21T17:30:00Z');
const danang = 'Asia/Ho_Chi_Minh';

describe('formatDate', () => {
  it('writes the date in English on the city clock', () => {
    expect(formatDate(instant, { timeZone: danang, lang: 'en' })).toBe('Sun, Nov 22');
  });

  it('writes the date in Vietnamese on the city clock', () => {
    expect(formatDate(instant, { timeZone: danang, lang: 'vi' })).toBe(`${viWeekday('2026-11-22')}, 22 thg 11`);
  });

  it('reads another city clock for the same instant', () => {
    expect(formatDate(instant, { timeZone: 'America/Los_Angeles', lang: 'en' })).toBe('Sat, Nov 21');
  });
});

describe('formatTime', () => {
  it('uses the 12-hour clock in English and H:mm in Vietnamese', () => {
    expect(formatTime(instant, { timeZone: danang, lang: 'en' })).toBe('12:30 AM');
    expect(formatTime(instant, { timeZone: danang, lang: 'vi' })).toBe('0:30');
  });
});

describe('formatDateTime', () => {
  it('joins the date and the time in each language', () => {
    expect(formatDateTime(instant, { timeZone: danang, lang: 'en' })).toBe('Sun, Nov 22, 12:30 AM');
    expect(formatDateTime(instant, { timeZone: danang, lang: 'vi' })).toBe(`0:30 ${viWeekday('2026-11-22')}, 22 thg 11`);
  });
});

describe('formatPrice', () => {
  it('writes whole dong in the language’s conventions, never converted', () => {
    expect(formatPrice(350000, 'VND', 'en')).toBe('₫350,000');
    expect(formatPrice(350000, 'VND', 'vi').replace(/ /g, ' ')).toBe('350.000 ₫');
  });
});

describe('formatTimeRange', () => {
  it("joins a plan's start and end as the city's clock gives them", () => {
    expect(formatTimeRange('09:00', '12:30')).toBe('09:00–12:30');
  });
});

describe('formatCalendarDate', () => {
  it("writes the city's calendar date as it is, whatever the device's zone", () => {
    expect(formatCalendarDate('2026-11-28', 'en')).toBe('Sat, Nov 28');
    expect(formatCalendarDate('2026-11-28', 'vi')).toBe(`${viWeekday('2026-11-28')}, 28 thg 11`);
  });
});

describe('cityDateKey', () => {
  it("is the city's calendar date as a sortable YYYY-MM-DD key, whatever the device's zone and the display language", () => {
    expect(cityDateKey(instant, danang)).toBe('2026-11-22');
    expect(cityDateKey(instant, 'UTC')).toBe('2026-11-21');
  });
});

describe('formatOutingWhen', () => {
  // 09:00 on 25 November in Da Nang is 02:00 UTC, 18:00 the day before in Los Angeles.
  const when = { date: '2026-11-25', startsAt: '09:00', timeZone: danang };

  it('writes the city clock alone when the device is on it', () => {
    expect(formatOutingWhen({ ...when, lang: 'en', deviceZone: danang })).toEqual({ city: 'Wed, Nov 25, 9:00 AM', device: null });
  });

  it('writes the device time beside it when the device is elsewhere, with its own day', () => {
    expect(formatOutingWhen({ ...when, lang: 'en', deviceZone: 'America/Los_Angeles' })).toEqual({ city: 'Wed, Nov 25, 9:00 AM', device: 'Tue, Nov 24, 6:00 PM' });
  });

  it('writes both in Vietnamese', () => {
    expect(formatOutingWhen({ ...when, lang: 'vi', deviceZone: 'America/Los_Angeles' })).toEqual({ city: `9:00 ${viWeekday('2026-11-25')}, 25 thg 11`, device: `18:00 ${viWeekday('2026-11-24')}, 24 thg 11` });
  });

  it('writes the city clock alone for another zone with the same time (no daylight saving in Da Nang)', () => {
    expect(formatOutingWhen({ ...when, lang: 'en', deviceZone: 'Asia/Bangkok' })).toEqual({ city: 'Wed, Nov 25, 9:00 AM', device: null });
  });

  it('finds the instant across a daylight-saving change in the other zone', () => {
    // 09:00 on 15 March 2027 in Da Nang is 02:00 UTC: Los Angeles went to daylight time on the 14th (UTC-7), so 19:00 on the 14th.
    expect(formatOutingWhen({ date: '2027-03-15', startsAt: '09:00', timeZone: danang, lang: 'en', deviceZone: 'America/Los_Angeles' }).device).toBe('Sun, Mar 14, 7:00 PM');
  });
});

describe('formatInstantWhen', () => {
  // 01:30 UTC on 25 November: 08:30 in Da Nang, 17:30 the day before in Los Angeles.
  const at = new Date('2026-11-25T01:30:00Z');

  it('writes the city clock alone when the device is on it', () => {
    expect(formatInstantWhen(at, { timeZone: danang, lang: 'en', deviceZone: danang })).toEqual({ city: 'Wed, Nov 25, 8:30 AM', device: null });
  });

  it('writes the device time beside it when the device is elsewhere', () => {
    expect(formatInstantWhen(at, { timeZone: danang, lang: 'en', deviceZone: 'America/Los_Angeles' })).toEqual({ city: 'Wed, Nov 25, 8:30 AM', device: 'Tue, Nov 24, 5:30 PM' });
  });
});

describe('formatInstantDay', () => {
  // 05:30 UTC on 2 December: the 2nd in Da Nang (12:30), the 1st at 21:30 in Los Angeles.
  const at = new Date('2026-12-02T05:30:00Z');

  it('writes the city day alone when the device is on the same day at another offset', () => {
    expect(formatInstantDay(at, { timeZone: danang, lang: 'en', deviceZone: 'Asia/Tokyo' })).toEqual({ city: 'Wed, Dec 2', device: null });
  });

  it("writes the device's day and time beside it when the device is on another day", () => {
    expect(formatInstantDay(at, { timeZone: danang, lang: 'en', deviceZone: 'America/Los_Angeles' })).toEqual({ city: 'Wed, Dec 2', device: 'Tue, Dec 1, 9:30 PM' });
  });
});
