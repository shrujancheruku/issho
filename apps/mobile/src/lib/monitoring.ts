import * as Sentry from '@sentry/react-native';

// Crash and error reporting, on only when EXPO_PUBLIC_SENTRY_DSN is set at
// build time (a DSN is public: it can only send reports). Without it,
// development, Expo Go, tests and the web build send nothing. No personal
// data: sendDefaultPii is off (no IP address, headers or cookies).
//
// Performance tracing too: app start, each screen (named by its route, e.g.
// "/plan/[id]", never the id), its requests, and slow or frozen frames. The
// API gets the trace with each request, so a slow screen links to what the
// server did. Only our API gets it: Supabase and Google don't.
const dsn = process.env.EXPO_PUBLIC_SENTRY_DSN;
const apiUrl = process.env.EXPO_PUBLIC_API_URL;
// Every session while there are few testers; lower it (e.g. 0.2) as
// people join, through the EAS environment, without a code change.
const tracesSampleRate = Number(process.env.EXPO_PUBLIC_SENTRY_TRACES_SAMPLE_RATE ?? 1);

export const monitoring = !!dsn;

if (dsn) {
  Sentry.init({
    dsn,
    sendDefaultPii: false,
    environment: process.env.EXPO_PUBLIC_SENTRY_ENVIRONMENT ?? (__DEV__ ? 'development' : 'production'),
    // No session replay.
    tracesSampleRate: Number.isFinite(tracesSampleRate) ? tracesSampleRate : 1,
    tracePropagationTargets: apiUrl ? [apiUrl] : [],
    integrations: [Sentry.expoRouterIntegration()],
  });
}

/** Wraps the root component so render crashes are reported (when on). */
export function withMonitoring(root: React.ComponentType): React.ComponentType {
  return monitoring ? Sentry.wrap(root) : root;
}
