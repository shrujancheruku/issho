import { lastSignup, prefill, signupDates, tickKey } from '../times';

const danang = 'Asia/Ho_Chi_Minh';
// 10:00 UTC on 22 November is 17:00 on the 22nd in Da Nang.
const now = new Date('2026-11-22T10:00:00Z');

describe('the dates a signup offers', () => {
  it("run from 3 days ahead on the city clock to 7 days after the round's last day", () => {
    expect(signupDates(now, danang, '2026-11-28')).toEqual([
      '2026-11-25', '2026-11-26', '2026-11-27', '2026-11-28', '2026-11-29', '2026-11-30', '2026-12-01', '2026-12-02', '2026-12-03', '2026-12-04', '2026-12-05',
    ]);
  });

  it("count today on the city's clock, not the device's", () => {
    // 18:00 UTC on the 22nd is already 01:00 on the 23rd in Da Nang.
    expect(signupDates(new Date('2026-11-22T18:00:00Z'), danang, '2026-11-28')[0]).toBe('2026-11-26');
  });

  it('cross a month and a year', () => {
    expect(signupDates(new Date('2026-12-29T03:00:00Z'), danang, '2027-01-02')).toEqual([
      '2027-01-01', '2027-01-02', '2027-01-03', '2027-01-04', '2027-01-05', '2027-01-06', '2027-01-07', '2027-01-08', '2027-01-09',
    ]);
  });

  it("fall back to the days a round always has when the plan's round end is missing or already past (an old cached page)", () => {
    const week = ['2026-11-25', '2026-11-26', '2026-11-27', '2026-11-28', '2026-11-29'];
    expect(signupDates(now, danang, '')).toEqual(week);
    expect(signupDates(now, danang, '2026-11-20')).toEqual(week);
  });
});

describe('the last signup', () => {
  const at = (seconds: number) => ({ $typeName: 'google.protobuf.Timestamp' as const, seconds: BigInt(seconds), nanos: 0 });
  const signup = (planId: string, hearBy: string, signedAt?: number) => ({ planId, hearBy, ticks: [], signedAt: signedAt === undefined ? undefined : at(signedAt) });

  it('is the newest by when it was made or last changed, not the one that is heard about last', () => {
    const signups = [signup('a', '2026-11-24', 300), signup('b', '2026-11-30', 100), signup('c', '2026-11-26', 200)];
    expect(lastSignup(signups as never)?.planId).toBe('a');
  });

  it('breaks a tie in the second by its fraction', () => {
    const a = { ...signup('a', '2026-11-24', 100), signedAt: { ...at(100), nanos: 5 } };
    const b = { ...signup('b', '2026-11-30', 100), signedAt: { ...at(100), nanos: 9 } };
    expect(lastSignup([a, b] as never)?.planId).toBe('b');
  });

  it('is nothing with no signups', () => {
    expect(lastSignup([])).toBeUndefined();
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
