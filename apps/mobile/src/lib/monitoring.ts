import * as Sentry from '@sentry/react-native';
import Constants from 'expo-constants';

// Crash and error reporting, on only when EXPO_PUBLIC_SENTRY_DSN is set at
// build time (a DSN is public: it can only send reports). Without it,
// development, Expo Go, tests and the web build send nothing. No personal
// data: sendDefaultPii is off (no IP address, headers or cookies).
const dsn = process.env.EXPO_PUBLIC_SENTRY_DSN;

export const monitoring = !!dsn;

if (dsn) {
  Sentry.init({
    dsn,
    sendDefaultPii: false,
    environment: process.env.EXPO_PUBLIC_SENTRY_ENVIRONMENT ?? (__DEV__ ? 'development' : 'production'),
    release: Constants.expoConfig?.version,
    // Errors only, no performance tracing or session replay for now.
    tracesSampleRate: 0,
  });
}

/** Wraps the root component so render crashes are reported (when on). */
export function withMonitoring(root: React.ComponentType): React.ComponentType {
  return monitoring ? Sentry.wrap(root) : root;
}
