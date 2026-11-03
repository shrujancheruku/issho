import { Code, ConnectError } from '@issho/api-client';

import type { StringKey } from '@/i18n/strings';
import { errorText } from '@/lib/error-text';

/**
 * What to tell the person when saving fails. The server's messages are
 * English and name fields ("birth_year: must be at least 18"), so they're
 * never shown: the steps check the same rules first, which makes a rejection
 * rare, and it gets a localized message of its own. Anything else reads as errorText.
 */
export function saveErrorMessage(err: unknown, t: (k: StringKey) => string): string {
  if (err instanceof ConnectError && err.code === Code.InvalidArgument) return t('error.saveRejected');
  return errorText(err, t);
}
