import { isDevApi } from '../api-url';

// What decides whether a build shows what only a deploy's checker needs (the
// API's version on the You tab): the API's address, the one thing a build knows
// about where it runs.
describe('isDevApi', () => {
  it.each([
    'http://localhost:8080',
    'http://127.0.0.1:8080',
    'http://192.168.1.20:8080',
    'http://[::1]:8080',
    'https://dev.joinissho.com',
    'https://api-dev.joinissho.com',
    'https://admin-dev.joinissho.com',
  ])('%s holds no real people', (url) => {
    expect(isDevApi(url)).toBe(true);
  });

  it.each(['https://joinissho.com', 'https://api.joinissho.com', 'https://developer.joinissho.com', 'https://devil.example.com', '', 'not a url'])(
    '%s is treated as production',
    (url) => {
      expect(isDevApi(url)).toBe(false);
    },
  );
});
