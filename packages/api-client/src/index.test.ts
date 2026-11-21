import { describe, expect, it } from 'vitest';

import { createApi } from './index';

// A fake fetch that answers the health check and keeps the request it saw.
function fakeFetch() {
  const seen: Request[] = [];
  const fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    seen.push(new Request(input, init));
    return new Response(JSON.stringify({ version: '1.2.3' }), {
      headers: { 'content-type': 'application/json' },
    });
  };
  return { fetch, seen };
}

describe('createApi', () => {
  it('calls the health check and reads the version', async () => {
    const { fetch, seen } = fakeFetch();
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => null, fetch });

    const res = await api.health.check({});

    expect(res.version).toBe('1.2.3');
    expect(seen[0].url).toBe('https://api.example.test/issho.v1.HealthService/Check');
  });

  it('sends the session token as a bearer token when there is one', async () => {
    const { fetch, seen } = fakeFetch();
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => 'tok-1', fetch });

    await api.health.check({});

    expect(seen[0].headers.get('authorization')).toBe('Bearer tok-1');
  });

  it('sends no authorization header when signed out', async () => {
    const { fetch, seen } = fakeFetch();
    const api = createApi({ baseUrl: 'https://api.example.test', getAccessToken: async () => null, fetch });

    await api.health.check({});

    expect(seen[0].headers.has('authorization')).toBe(false);
  });
});
