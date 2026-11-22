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
    'http://10.0.2.2:8080',
    'http://172.20.1.1:8080',
    'http://[fd12:3456::1]:8080',
    'https://dev.joinissho.com',
    'https://api-dev.joinissho.com',
    'https://admin-dev.joinissho.com',
  ])('%s holds no real people', (url) => {
    expect(isDevApi(url)).toBe(true);
  });

  it.each(['http://8.8.8.8:8080', 'http://172.32.0.1:8080', 'http://[2001:db8::1]:8080', 'https://joinissho.com', 'https://api.joinissho.com', 'https://developer.joinissho.com', 'https://devil.example.com', '', 'not a url'])(
    '%s is treated as production',
    (url) => {
      expect(isDevApi(url)).toBe(false);
    },
  );
});
