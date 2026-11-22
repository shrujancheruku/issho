import { isAdult, isRealPastDate, validName } from '../answers';
import { languageName, searchLanguages } from '../languages';

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

describe('searching every language by its name, in the display language', () => {
  it('finds a language the short list does not hold, by its English name', () => {
    expect(searchLanguages('fili', 'en').map((l) => l.code)).toContain('fil');
    expect(languageName('fil', 'en')).toBe('Filipino');
  });

  it('shows and matches the names in the display language', () => {
    expect(languageName('ja', 'vi')).toBe('Tiếng Nhật');
    expect(searchLanguages('tiếng nhật', 'vi').map((l) => l.code)).toContain('ja');
    expect(searchLanguages('nhat', 'vi').map((l) => l.code)).toContain('ja');
  });

  it('ignores case and accents, and an empty search lists nothing', () => {
    expect(searchLanguages('HAWAIIAN', 'en').map((l) => l.code)).toContain('haw');
    expect(searchLanguages('  ', 'en')).toEqual([]);
  });

  it('offers codes the API accepts, once each, with no placeholder entries', () => {
    const all = searchLanguages('a', 'en', 10000);
    const codes = all.map((l) => l.code);
    expect(new Set(codes).size).toBe(codes.length);
    for (const code of codes) expect(code).toMatch(/^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$/);
    for (const pseudo of ['und', 'mul', 'zxx', 'mis']) expect(codes).not.toContain(pseudo);
    expect(codes).not.toContain('eng'); // "en" is the one code for English
  });
});
