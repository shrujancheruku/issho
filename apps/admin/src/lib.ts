import { createApi } from '@issho/api-client';
import { createClient } from '@supabase/supabase-js';

// Both values are public by design: the publishable key only allows what
// Supabase Auth exposes to anyone (sign-in). Data comes from the Go API,
// which checks the admin role in the access token. See docs/secrets.md.
const supabaseUrl = import.meta.env.VITE_SUPABASE_URL;
const publishableKey = import.meta.env.VITE_SUPABASE_PUBLISHABLE_KEY;
const apiUrl = import.meta.env.VITE_API_URL;
if (!supabaseUrl || !publishableKey || !apiUrl) {
  throw new Error('VITE_API_URL, VITE_SUPABASE_URL and VITE_SUPABASE_PUBLISHABLE_KEY must be set (see apps/admin/.env.example)');
}

export const supabase = createClient(supabaseUrl, publishableKey, {
  // Sign-in is by emailed code, never a link, so there's no session in the URL.
  auth: { persistSession: true, autoRefreshToken: true, detectSessionInUrl: false },
});

export const api = createApi({
  baseUrl: apiUrl,
  getAccessToken: async () => (await supabase.auth.getSession()).data.session?.access_token ?? null,
});
