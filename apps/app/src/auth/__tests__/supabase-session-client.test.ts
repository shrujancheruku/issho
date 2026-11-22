import { createSupabaseSessionClient } from '../supabase-session-client';

// The adapter against a stand-in for Supabase Auth's HTTP API: the fetch is
// ours, so nothing reaches a network. The storage is shared between two
// clients, as the device's storage is across a restart.
const URL = 'https://auth.example.test';

function memoryStorage() {
  const data = new Map<string, string>();
  return {
    getItem: async (k: string) => data.get(k) ?? null,
    setItem: async (k: string, v: string) => void data.set(k, v),
    removeItem: async (k: string) => void data.delete(k),
  };
}

function fakeAuthServer() {
  const calls: { path: string; body: Record<string, unknown> }[] = [];
  const respond = (status: number, body: unknown) =>
    new Response(JSON.stringify(body), {
      status,
      headers: { 'content-type': 'application/json' },
    });
  const fetch = jest.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new globalThis.URL(String(input));
    const body = init?.body ? JSON.parse(String(init.body)) : {};
    calls.push({ path: url.pathname, body });
    if (url.pathname.endsWith('/otp')) return respond(200, {});
    if (url.pathname.endsWith('/verify')) {
      if (body.token !== '123456')
        return respond(403, {
          code: 403,
          error_code: 'otp_expired',
          msg: 'Token has expired or is invalid',
        });
      return respond(200, {
        access_token: 'access-1',
        refresh_token: 'refresh-1',
        token_type: 'bearer',
        expires_in: 3600,
        expires_at: Math.floor(Date.now() / 1000) + 3600,
        user: {
          id: 'u1',
          aud: 'authenticated',
          email: String(body.email),
          app_metadata: {},
          user_metadata: {},
          created_at: '2026-11-22T00:00:00Z',
        },
      });
    }
    if (url.pathname.endsWith('/logout')) return new Response(null, { status: 204 });
    return respond(500, {});
  });
  return { fetch, calls };
}

const make = (server: ReturnType<typeof fakeAuthServer>, storage = memoryStorage()) =>
  createSupabaseSessionClient({
    url: URL,
    key: 'sb_publishable_test',
    storage,
    fetch: server.fetch as never,
  });

describe('the Supabase session client', () => {
  it('asks Supabase to email a code, and to make the account for a first-time email', async () => {
    const server = fakeAuthServer();
    await make(server).sendCode('ada@example.com');
    expect(server.calls.find((c) => c.path.endsWith('/otp'))?.body).toMatchObject({ email: 'ada@example.com', create_user: true });
  });

  it('signs in with the right code and holds the session', async () => {
    const server = fakeAuthServer();
    const client = make(server);
    expect(await client.verifyCode('ada@example.com', '123456')).toBe(true);
    expect(await client.getSession()).toEqual({
      accessToken: 'access-1',
      email: 'ada@example.com',
    });
    expect(server.calls.find((c) => c.path.endsWith('/verify'))?.body).toMatchObject({
      type: 'email',
      email: 'ada@example.com',
      token: '123456',
    });
  });

  it('answers false for a wrong or expired code, and holds no session', async () => {
    const client = make(fakeAuthServer());
    expect(await client.verifyCode('ada@example.com', '000000')).toBe(false);
    expect(await client.getSession()).toBeNull();
  });

  it('throws when the code could not be sent or checked at all', async () => {
    const server = fakeAuthServer();
    server.fetch.mockRejectedValue(new TypeError('network down'));
    const client = make(server);
    await expect(client.sendCode('ada@example.com')).rejects.toThrow();
    await expect(client.verifyCode('ada@example.com', '123456')).rejects.toThrow();
  });

  it('survives a restart: a new client on the same storage is still signed in', async () => {
    const storage = memoryStorage();
    await make(fakeAuthServer(), storage).verifyCode('ada@example.com', '123456');
    const afterRestart = make(fakeAuthServer(), storage);
    expect(await afterRestart.getSession()).toEqual({
      accessToken: 'access-1',
      email: 'ada@example.com',
    });
  });

  it('forgets the session on sign-out, for good', async () => {
    const storage = memoryStorage();
    const client = make(fakeAuthServer(), storage);
    await client.verifyCode('ada@example.com', '123456');
    await client.signOut();
    expect(await client.getSession()).toBeNull();
    expect(await make(fakeAuthServer(), storage).getSession()).toBeNull();
  });

  it('tells a listener about sign-in and sign-out', async () => {
    const client = make(fakeAuthServer());
    const seen: (string | null)[] = [];
    const stop = client.onChange((s) => seen.push(s?.email ?? null));
    await client.verifyCode('ada@example.com', '123456');
    await client.signOut();
    stop();
    expect(seen).toEqual(expect.arrayContaining(['ada@example.com', null]));
    expect(seen.lastIndexOf(null)).toBeGreaterThan(seen.indexOf('ada@example.com'));
  });
});
