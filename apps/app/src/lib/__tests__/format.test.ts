import { cityDateKey, formatCalendarDate, formatDate, formatDateTime, formatPrice, formatTime, formatTimeRange } from '../format';

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
    expect(formatDate(instant, { timeZone: danang, lang: 'vi' })).toBe('CN, 22 thg 11');
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
    expect(formatDateTime(instant, { timeZone: danang, lang: 'vi' })).toBe('0:30 CN, 22 thg 11');
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
    expect(formatCalendarDate('2026-11-28', 'vi')).toBe('Th 7, 28 thg 11');
  });
});

describe('cityDateKey', () => {
  it("is the city's calendar date as a sortable YYYY-MM-DD key, whatever the device's zone and the display language", () => {
    expect(cityDateKey(instant, danang)).toBe('2026-11-22');
    expect(cityDateKey(instant, 'UTC')).toBe('2026-11-21');
  });
});
