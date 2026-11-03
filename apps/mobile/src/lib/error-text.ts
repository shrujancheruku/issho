import { Code, ConnectError, ErrorReason, errorReason } from '@issho/api-client';

import type { StringKey } from '@/i18n/strings';

/** What each reason the API names reads as (reasons are in proto/issho/v1/errors.proto). */
const REASON_KEYS: Partial<Record<ErrorReason, StringKey>> = {
  [ErrorReason.SUSPENDED]: 'error.suspended',
  [ErrorReason.NO_PROFILE]: 'error.noProfile',
  [ErrorReason.EVENT_OVER]: 'error.eventOver',
  [ErrorReason.TOO_LATE]: 'event.tooLate',
  [ErrorReason.NOT_NOW]: 'error.notNow',
  [ErrorReason.CHECK_IN_CLOSED]: 'error.checkInClosed',
  [ErrorReason.FEEDBACK_CLOSED]: 'feedback.closed',
};

/** The request never got an answer: the server is unreachable, or fetch itself failed. */
function isNetworkFailure(err: unknown): boolean {
  if (err instanceof TypeError) return true; // fetch rejects with TypeError when offline
  if (!(err instanceof ConnectError)) return false;
  return err.code === Code.Unavailable || err.cause instanceof TypeError;
}

/**
 * The one way the app turns an error into text for the person. It is keyed on
 * the reason the API attached, then on whether the network failed, and
 * otherwise says something generic. The server's `message` is never shown: it
 * is English and written for logs.
 */
export function errorText(err: unknown, t: (key: StringKey) => string): string {
  const key = REASON_KEYS[errorReason(err) ?? ErrorReason.UNSPECIFIED];
  if (key) return t(key);
  return t(isNetworkFailure(err) ? 'error.network' : 'error.generic');
}
