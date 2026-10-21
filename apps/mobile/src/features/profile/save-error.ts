import { Code, ConnectError } from '@issho/api-client';

import type { StringKey } from '@/i18n/strings';

/**
 * What to tell the person when saving fails. invalid_argument messages name
 * the field (e.g. "birth_year: must be at least 18"); the steps check the
 * same rules first, so they're rare and shown as they are.
 */
export function saveErrorMessage(err: unknown, t: (k: StringKey) => string): string {
  if (err instanceof ConnectError && err.code === Code.InvalidArgument) return err.rawMessage;
  return t('error.network');
}
