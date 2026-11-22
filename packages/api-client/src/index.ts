import { createClient, type Interceptor } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';

import { AccountService } from './gen/issho/v1/account_pb';
import { HealthService } from './gen/issho/v1/health_pb';

export * from './gen/issho/v1/access_pb';
export * from './gen/issho/v1/account_pb';
export * from './gen/issho/v1/health_pb';

export interface ApiOptions {
  baseUrl: string;
  /** Called before every request; a token is sent as `Authorization: Bearer`. */
  getAccessToken: () => Promise<string | null>;
  /** For tests; defaults to the global fetch. */
  fetch?: typeof globalThis.fetch;
}

/** The one client the app, the Expo server and the admin use to call the API. */
export function createApi({ baseUrl, getAccessToken, fetch }: ApiOptions) {
  const bearer: Interceptor = (next) => async (req) => {
    const token = await getAccessToken();
    if (token) req.header.set('Authorization', `Bearer ${token}`);
    return next(req);
  };
  // JSON, not binary: it works with React Native's fetch and reads in a network
  // inspector.
  const transport = createConnectTransport({
    baseUrl,
    useBinaryFormat: false,
    interceptors: [bearer],
    fetch,
  });
  return {
    health: createClient(HealthService, transport),
    account: createClient(AccountService, transport),
  };
}

export type Api = ReturnType<typeof createApi>;
