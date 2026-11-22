import { afterEach, describe, expect, it, vi } from 'vitest';

import worker from './index';

const env = { API_ORIGIN: 'https://api.example.test' };

afterEach(() => vi.unstubAllGlobals());

// The deployed admin and the API are on different addresses and the API has no
// CORS, so the admin's Worker hands the API's paths to the API: the browser
// only ever talks to the admin's own origin, as it does through Vite's proxy in
// development.
describe('the admin Worker', () => {
  it("forwards an RPC to the API's origin with its method, headers and body, and returns the answer", async () => {
    const upstream = vi.fn().mockResolvedValue(new Response('{"version":"a1b2c3d"}', { status: 200, headers: { 'content-type': 'application/json' } }));
    vi.stubGlobal('fetch', upstream);

    const res = await worker.fetch(
      new Request('https://admin.example.test/issho.v1.HealthService/Check?x=1', {
        method: 'POST',
        headers: { 'content-type': 'application/json', authorization: 'Bearer token' },
        body: '{}',
      }),
      env,
    );

    expect(res.status).toBe(200);
    expect(await res.json()).toEqual({ version: 'a1b2c3d' });
    const [target, init] = upstream.mock.calls[0] as [string, RequestInit];
    expect(target).toBe('https://api.example.test/issho.v1.HealthService/Check?x=1');
    expect(init.method).toBe('POST');
    expect(new Headers(init.headers).get('authorization')).toBe('Bearer token');
    expect(await new Response(init.body).text()).toBe('{}');
  });

  it("passes the API's error through unchanged", async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('{"code":"unauthenticated"}', { status: 401 })));
    const res = await worker.fetch(new Request('https://admin.example.test/issho.v1.StaffService/List', { method: 'POST', body: '{}' }), env);
    expect(res.status).toBe(401);
  });

  it('forwards nothing outside the API paths, so it is not an open proxy', async () => {
    const upstream = vi.fn();
    vi.stubGlobal('fetch', upstream);
    for (const path of ['/', '/index.html', '/issho.v1', '/%2e%2e/secret', '//evil.example.test/issho.v1.X/Y']) {
      const res = await worker.fetch(new Request(`https://admin.example.test${path}`, { method: 'POST', body: '{}' }), env);
      expect(res.status, path).toBe(404);
    }
    expect(upstream).not.toHaveBeenCalled();
  });

  // Issho keeps no IP address anywhere (docs/spec/privacy.md), so the member's
  // address, which Cloudflare puts in these, never reaches the API.
  it("does not pass the member's address on to the API", async () => {
    const upstream = vi.fn().mockResolvedValue(new Response('{}'));
    vi.stubGlobal('fetch', upstream);
    const address = {
      'cf-connecting-ip': '198.51.100.7',
      'cf-connecting-ipv6': '2001:db8::7',
      'cf-pseudo-ipv4': '198.51.100.8',
      'cf-ipcountry': 'VN',
      'x-forwarded-for': '198.51.100.7, 203.0.113.1',
      'x-real-ip': '198.51.100.7',
      forwarded: 'for=198.51.100.7;proto=https',
      'true-client-ip': '198.51.100.7',
      'x-client-ip': '198.51.100.7',
      'x-cluster-client-ip': '198.51.100.7',
      'client-ip': '198.51.100.7',
    };

    await worker.fetch(
      new Request('https://admin.example.test/issho.v1.HealthService/Check', {
        method: 'POST',
        headers: { ...address, authorization: 'Bearer token' },
        body: '{}',
      }),
      env,
    );

    const sent = new Headers((upstream.mock.calls[0] as [string, RequestInit])[1].headers);
    for (const name of Object.keys(address)) expect(sent.get(name), name).toBeNull();
    expect(sent.get('authorization')).toBe('Bearer token');
  });

  it('answers 502 when the API cannot be reached', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('network')));
    const res = await worker.fetch(new Request('https://admin.example.test/issho.v1.HealthService/Check', { method: 'POST', body: '{}' }), env);
    expect(res.status).toBe(502);
  });
});
