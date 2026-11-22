import { isAdult, isRealPastDate, validName } from '../answers';

describe('18 on the day', () => {
  it.each([
    ['2000-06-15', '2018-06-15', true],
    ['2000-06-15', '2018-06-14', false],
    // A 29 February birthday turns 18 on 1 March in a common year, as the API reads it.
    ['2004-02-29', '2022-02-28', false],
    ['2004-02-29', '2022-03-01', true],
    ['2000-02-29', '2018-02-28', false],
  ])('born %s, on %s: %s', (dob, today, adult) => {
    expect(isAdult(dob, today)).toBe(adult);
  });
});

describe('a date of birth', () => {
  it.each([
    ['1995-04-23', true],
    ['1995-02-30', false],
    ['95-04-23', false],
    ['2999-01-01', false],
    ['1850-01-01', false],
  ])('%s is a real past date: %s', (dob, ok) => {
    expect(isRealPastDate(dob, '2026-11-22')).toBe(ok);
  });
});

describe('a name', () => {
  it('is 1 to 40 characters once trimmed', () => {
    expect(validName('  ')).toBe(false);
    expect(validName('a')).toBe(true);
    expect(validName('a'.repeat(40))).toBe(true);
    expect(validName('a'.repeat(41))).toBe(false);
  });
});
