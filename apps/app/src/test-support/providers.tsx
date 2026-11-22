import type { ReactNode } from 'react';

import { ApiProvider } from '@/api/api-provider';
import { AuthProvider } from '@/auth/auth-provider';
import { I18nProvider } from '@/i18n/i18n-provider';
import type { Language } from '@/i18n/strings';

import type { fakeApi } from './fake-api';
import { fakeSessionClient, type FakeSessionClient } from './fake-session-client';

/**
 * What the root layout provides, with a chosen language, a fake API and a fake
 * session client. A test that gives no session client gets a signed-in person.
 */
export function TestProviders({
  lang,
  api,
  session = fakeSessionClient({
    accessToken: 'token',
    email: 'ada@example.com',
  }),
  children,
}: {
  lang: Language;
  api: ReturnType<typeof fakeApi>;
  session?: FakeSessionClient;
  children: ReactNode;
}) {
  return (
    <ApiProvider api={api as never}>
      <AuthProvider client={session}>
        <I18nProvider lang={lang}>{children}</I18nProvider>
      </AuthProvider>
    </ApiProvider>
  );
}
