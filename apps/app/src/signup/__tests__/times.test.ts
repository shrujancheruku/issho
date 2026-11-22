import { prefill, signupDates, tickKey } from '../times';

const danang = 'Asia/Ho_Chi_Minh';

describe('the dates a signup offers', () => {
  it('start 3 days ahead on the city clock and run for 5 days', () => {
    // 10:00 UTC on 22 November is 17:00 on the 22nd in Da Nang.
    expect(signupDates(new Date('2026-11-22T10:00:00Z'), danang)).toEqual(['2026-11-25', '2026-11-26', '2026-11-27', '2026-11-28', '2026-11-29']);
  });

  it("count today on the city's clock, not the device's", () => {
    // 18:00 UTC on the 22nd is already 01:00 on the 23rd in Da Nang.
    expect(signupDates(new Date('2026-11-22T18:00:00Z'), danang)[0]).toBe('2026-11-26');
  });

  it('cross a month and a year', () => {
    expect(signupDates(new Date('2026-12-29T03:00:00Z'), danang)).toEqual(['2027-01-01', '2027-01-02', '2027-01-03', '2027-01-04', '2027-01-05']);
  });
});

describe('the ticks prefilled from the last signup', () => {
  const dates = ['2026-11-25', '2026-11-26', '2026-11-27', '2026-11-28', '2026-11-29'];
  const last = [
    { date: '2026-11-20', part: 'evening' },
    { date: '2026-11-26', part: 'morning' },
    { date: '2026-11-28', part: 'evening' },
  ];

  it('keep the ticks that fall in the new window, on a part the plan allows', () => {
    expect([...prefill(last, dates, ['morning', 'evening'])].sort()).toEqual([tickKey('2026-11-26', 'morning'), tickKey('2026-11-28', 'evening')]);
    expect([...prefill(last, dates, ['morning'])]).toEqual([tickKey('2026-11-26', 'morning')]);
  });

  it('are empty with no earlier signup', () => {
    expect(prefill([], dates, ['morning']).size).toBe(0);
  });
});
