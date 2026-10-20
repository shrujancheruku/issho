import { createClient } from '@supabase/supabase-js';
import { AppState } from 'react-native';

import { secureStorage } from '@/lib/secure-storage';

// Both values are public by design: the publishable key only allows what Supabase
// Auth exposes to anyone (sign-in). The app never talks to the database directly;
// all data goes through the Go API. See docs/secrets.md.
const url = process.env.EXPO_PUBLIC_SUPABASE_URL;
const publishableKey = process.env.EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY;
if (!url || !publishableKey) {
  throw new Error('EXPO_PUBLIC_SUPABASE_URL and EXPO_PUBLIC_SUPABASE_PUBLISHABLE_KEY must be set (see apps/mobile/.env.example)');
}

export const supabase = createClient(url, publishableKey, {
  auth: {
    storage: secureStorage,
    autoRefreshToken: true,
    persistSession: true,
    // There is no browser URL to read a session from in a native app.
    detectSessionInUrl: false,
  },
});

// Refresh tokens only while the app is in the foreground. Background timers are
// unreliable on mobile; when the app comes back, refreshing resumes and renews an
// expired access token before the next API call.
AppState.addEventListener('change', (state) => {
  if (state === 'active') supabase.auth.startAutoRefresh();
  else supabase.auth.stopAutoRefresh();
});
