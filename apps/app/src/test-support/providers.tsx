import type { ReactNode } from 'react';
import { SafeAreaProvider } from 'react-native-safe-area-context';

import { ApiProvider } from '@/api/api-provider';
import { AuthProvider } from '@/auth/auth-provider';
import { I18nProvider } from '@/i18n/i18n-provider';
import type { Language } from '@/i18n/strings';
import { MyOutingsProvider } from '@/outings/use-my-outings';

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
    // Screens that keep clear of the status bar read the insets; a test has none, unless it mocks them.
    <SafeAreaProvider initialMetrics={{ frame: { x: 0, y: 0, width: 390, height: 780 }, insets: { top: 0, left: 0, right: 0, bottom: 0 } }}>
      <ApiProvider api={api as never}>
        <AuthProvider client={session}>
          <I18nProvider lang={lang}>
            <MyOutingsProvider>{children}</MyOutingsProvider>
          </I18nProvider>
        </AuthProvider>
      </ApiProvider>
    </SafeAreaProvider>
  );
}
