import { pickShown, resolveShowing } from '../shown';

describe('resolveShowing', () => {
  it.each([
    ['en', 'primary', 'primary'],
    ['ja', 'primary', 'primary'],
    ['ja', 'secondary', 'secondary'],
    // The switch can be left on secondary after the secondary language is removed.
    ['', 'secondary', 'primary'],
  ] as const)('secondary %j, switch on %s shows %s', (secondary, showing, expected) => {
    expect(resolveShowing(secondary, showing)).toBe(expected);
  });
});

describe('pickShown', () => {
  it('shows the primary text first when showing primary', () => {
    expect(pickShown('primary', 'Pottery', '陶芸')).toEqual({ name: 'Pottery', secondary: '陶芸' });
  });

  it('swaps the pair when showing secondary and a secondary text exists', () => {
    expect(pickShown('secondary', 'Pottery', '陶芸')).toEqual({ name: '陶芸', secondary: 'Pottery' });
  });

  it('keeps the primary text first when showing secondary but there is no secondary text', () => {
    expect(pickShown('secondary', 'Pottery', '')).toEqual({ name: 'Pottery', secondary: '' });
    expect(pickShown('secondary', 'Pottery', undefined)).toEqual({ name: 'Pottery', secondary: '' });
  });
});
