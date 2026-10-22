import { createApi } from '@issho/api-client';

import { supabase } from '@/lib/supabase';

// Point at the Go API. On a physical device use your machine's LAN IP.
const baseUrl = process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';

// getSession() returns the cached session and refreshes it first if the access
// token has expired, so each API call carries a valid token.
export const getAccessToken = async () => (await supabase.auth.getSession()).data.session?.access_token ?? null;

export const api = createApi({ baseUrl, getAccessToken });

/** The API's WebSocket base (ws:// or wss://), for live group chat. */
export const wsBaseUrl = baseUrl.replace(/^http/, 'ws');
