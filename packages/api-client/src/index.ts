import { createClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { CatalogService } from "./gen/issho/v1/catalog_pb";
import { CrewService } from "./gen/issho/v1/crews_pb";
import { EventService } from "./gen/issho/v1/events_pb";
import { HealthService } from "./gen/issho/v1/health_pb";
import { IdentityService } from "./gen/issho/v1/identity_pb";
import { PlanService } from "./gen/issho/v1/plans_pb";
import { ProfileService } from "./gen/issho/v1/profile_pb";
import { PushService } from "./gen/issho/v1/push_pb";
import { UIService } from "./gen/issho/v1/ui_pb";

// Error types for callers (code checks like `err.code === Code.Unauthenticated`), so apps
// don't need their own dependency on @connectrpc/connect.
export { Code, ConnectError } from "@connectrpc/connect";
export * from "./gen/issho/v1/catalog_pb";
export * from "./gen/issho/v1/crews_pb";
export * from "./gen/issho/v1/events_pb";
export * from "./gen/issho/v1/health_pb";
export * from "./gen/issho/v1/identity_pb";
export * from "./gen/issho/v1/plans_pb";
export * from "./gen/issho/v1/profile_pb";
export * from "./gen/issho/v1/push_pb";
export * from "./gen/issho/v1/ui_pb";

export type ApiClientOptions = {
  baseUrl: string;
  /** Returns the current Supabase access token, or null when signed out. */
  getAccessToken: () => Promise<string | null>;
};

export function createApi({ baseUrl, getAccessToken }: ApiClientOptions) {
  const auth: Interceptor = (next) => async (req) => {
    const token = await getAccessToken();
    if (token) req.header.set("Authorization", `Bearer ${token}`);
    return next(req);
  };
  // JSON over HTTP/1.1 works everywhere, including React Native's fetch.
  const transport = createConnectTransport({ baseUrl, useBinaryFormat: false, interceptors: [auth] });
  return {
    catalog: createClient(CatalogService, transport),
    crews: createClient(CrewService, transport),
    events: createClient(EventService, transport),
    health: createClient(HealthService, transport),
    identity: createClient(IdentityService, transport),
    plans: createClient(PlanService, transport),
    profile: createClient(ProfileService, transport),
    push: createClient(PushService, transport),
    ui: createClient(UIService, transport),
  };
}

export type Api = ReturnType<typeof createApi>;
