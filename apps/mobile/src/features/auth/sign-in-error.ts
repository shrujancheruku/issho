import type { StringKey } from '@/i18n/strings';

/**
 * supabase-js `AuthError.code` values the sign-in screen words itself. The
 * `message` of an AuthError is English and written for developers, so it's
 * never shown; any other code (or none) reads as the generic error.
 */
const CODE_KEYS: Record<string, StringKey> = {
  otp_expired: 'signIn.errorExpired',
  over_email_send_rate_limit: 'signIn.errorRateLimit',
  email_address_invalid: 'signIn.errorInvalid',
  validation_failed: 'signIn.errorInvalid',
};

export function signInErrorText(error: { code?: string }, t: (key: StringKey) => string): string {
  const key = error.code ? CODE_KEYS[error.code] : undefined;
  return t(key ?? 'error.generic');
}
