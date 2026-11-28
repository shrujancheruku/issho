import { Code, ConnectError, createClient, type Interceptor } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';

import { AccountService } from './gen/issho/v1/account_pb';
import { HealthService } from './gen/issho/v1/health_pb';
import { JoinLinkService } from './gen/issho/v1/join_link_pb';
import { PlanService } from './gen/issho/v1/plan_pb';
import { OutingService } from './gen/issho/v1/outing_pb';
import { ProfileService } from './gen/issho/v1/profile_pb';
import { SignupService } from './gen/issho/v1/signup_pb';

export * from './gen/issho/v1/access_pb';
export * from './gen/issho/v1/account_pb';
export * from './gen/issho/v1/health_pb';
export * from './gen/issho/v1/join_link_pb';
export * from './gen/issho/v1/plan_pb';
export * from './gen/issho/v1/outing_pb';
export * from './gen/issho/v1/profile_pb';
export * from './gen/issho/v1/signup_pb';

export interface ApiOptions {
  baseUrl: string;
  /** Called before every request; a token is sent as `Authorization: Bearer`. */
  getAccessToken: () => Promise<string | null>;
  /**
   * Called when the API answers that the caller is not signed in (a missing,
   * expired or revoked token), so the app can send the person to sign-in from
   * one place instead of in every screen. The error is still thrown.
   */
  onUnauthenticated?: () => void;
  /** For tests; defaults to the global fetch. */
  fetch?: typeof globalThis.fetch;
}

/** The one client the app, the Expo server and the admin use to call the API. */
export function createApi({ baseUrl, getAccessToken, onUnauthenticated, fetch }: ApiOptions) {
  const bearer: Interceptor = (next) => async (req) => {
    const token = await getAccessToken();
    if (token) req.header.set('Authorization', `Bearer ${token}`);
    return next(req);
  };
  const unauthenticated: Interceptor = (next) => async (req) => {
    try {
      return await next(req);
    } catch (err) {
      if (ConnectError.from(err).code === Code.Unauthenticated) onUnauthenticated?.();
      throw err;
    }
  };
  // JSON, not binary: it works with React Native's fetch and reads in a network
  // inspector.
  const transport = createConnectTransport({
    baseUrl,
    useBinaryFormat: false,
    interceptors: [bearer, unauthenticated],
    fetch,
  });
  return {
    health: createClient(HealthService, transport),
    account: createClient(AccountService, transport),
    plans: createClient(PlanService, transport),
    profile: createClient(ProfileService, transport),
    signup: createClient(SignupService, transport),
    outing: createClient(OutingService, transport),
    joinLink: createClient(JoinLinkService, transport),
  };
}

export type Api = ReturnType<typeof createApi>;

// The profile screens (#614): reading the reason a profile RPC refused with.
export { profileProblem } from './profile-problem';
