import { createApi } from '@issho/api-client';

import { apiUrl } from '@/api/api-url';
import type { SessionClient } from '@/auth/session-client';

/**
 * The one API client the app uses. Every call carries the signed-in person's
 * token (a visitor's go out with none, which only public RPCs accept). When the
 * API says the token is not good (revoked, expired past refreshing), the
 * session ends here, once, for every screen: the signed-in screens' guard then
 * sends the person to sign-in and back (components/require-sign-in.tsx).
 */
export function createAppApi(session: Pick<SessionClient, 'getSession' | 'signOut'>, options: { fetch?: typeof globalThis.fetch } = {}) {
  return createApi({
    baseUrl: apiUrl(),
    getAccessToken: async () => (await session.getSession().catch(() => null))?.accessToken ?? null,
    onUnauthenticated: () => void session.signOut().catch(() => {}),
    fetch: options.fetch,
  });
}
