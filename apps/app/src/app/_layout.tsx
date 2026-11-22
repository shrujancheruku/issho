import { createApi } from '@issho/api-client';
import { Slot } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { useMemo } from 'react';

import { ApiProvider } from '@/api/api-provider';
import { apiUrl } from '@/api/api-url';
import { createAppSessionClient } from '@/auth/app-session-client';
import { AuthProvider } from '@/auth/auth-provider';
import { I18nProvider } from '@/i18n/i18n-provider';
import { useDeviceLanguage } from '@/i18n/use-device-language';
import '@/global.css';

export default function RootLayout() {
  const session = useMemo(() => createAppSessionClient(), []);
  // Every call carries the signed-in person's token; a visitor's go out with none, which only public RPCs accept.
  const api = useMemo(
    () =>
      createApi({
        baseUrl: apiUrl(),
        getAccessToken: async () => (await session.getSession().catch(() => null))?.accessToken ?? null,
      }),
    [session],
  );
  const lang = useDeviceLanguage();
  return (
    <ApiProvider api={api}>
      <AuthProvider client={session}>
        <I18nProvider lang={lang}>
          <StatusBar style="auto" />
          <Slot />
        </I18nProvider>
      </AuthProvider>
    </ApiProvider>
  );
}
