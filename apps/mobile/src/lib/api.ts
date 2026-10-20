import { createApi } from '@issho/api-client';

// Point at the Go API. On a physical device use your machine's LAN IP.
const baseUrl = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';

export const api = createApi({
  baseUrl,
  // Supabase sessions are wired in during the auth milestone.
  getAccessToken: async () => null,
});
