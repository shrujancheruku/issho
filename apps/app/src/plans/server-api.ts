import { createApi } from '@issho/api-client';

import { apiUrl } from '@/api/api-url';

/**
 * The client the Expo server's loaders use. It sends no token: the server holds
 * none, so what it renders is the same for everyone and can be cached
 * (docs/spec/architecture.md, "The web at one address"). It calls the API at
 * `API_ORIGIN` when the server has one (the same address it forwards the
 * browser's calls to, apps/web/README.md), so a page's data does not go out to
 * the public name and back in; otherwise at the build's own `EXPO_PUBLIC_API_URL`.
 */
export function createServerApi() {
  return createApi({ baseUrl: process.env.API_ORIGIN || apiUrl(), getAccessToken: async () => null });
}
