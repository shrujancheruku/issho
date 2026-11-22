import { returnPath } from '../return-path';

describe('where to go after signing in', () => {
  it.each([
    ['/plans', '/plans'],
    ['/profile?next=/you', '/profile?next=/you'],
    [undefined, '/'],
    ['', '/'],
    ['https://evil.example', '/'],
    ['//evil.example', '/'],
    ['/\\evil.example', '/'],
    // Browsers drop a tab or newline inside a URL, so these read as "//evil.example".
    ['/\t/evil.example', '/'],
    ['/\n/evil.example', '/'],
    ['/\r\\evil.example', '/'],
    ['/ /evil.example', '/'],
    ['/\u0000/evil.example', '/'],
    ['/ /evil.example', '/'],
  ])('%j goes to %j', (next, expected) => {
    expect(returnPath(next)).toBe(expected);
  });

  it('takes the first of several values', () => {
    expect(returnPath(['/plans', '//evil.example'])).toBe('/plans');
  });
});
