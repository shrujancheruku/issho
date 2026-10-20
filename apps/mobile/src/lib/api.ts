import { createApi } from '@issho/api-client';

import { supabase } from '@/lib/supabase';

// Point at the Go API. On a physical device use your machine's LAN IP.
const baseUrl = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';

export const api = createApi({
  baseUrl,
  // getSession() returns the cached session and refreshes it first if the access
  // token has expired, so each API call carries a valid token.
  getAccessToken: async () => (await supabase.auth.getSession()).data.session?.access_token ?? null,
});
