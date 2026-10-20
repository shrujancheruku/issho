import { createClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { CatalogService } from "./gen/issho/v1/catalog_pb";
import { HealthService } from "./gen/issho/v1/health_pb";
import { IdentityService } from "./gen/issho/v1/identity_pb";

export * from "./gen/issho/v1/catalog_pb";
export * from "./gen/issho/v1/health_pb";
export * from "./gen/issho/v1/identity_pb";

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
    health: createClient(HealthService, transport),
    identity: createClient(IdentityService, transport),
  };
}

export type Api = ReturnType<typeof createApi>;
