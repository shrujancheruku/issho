import { createApi } from '@issho/api-client';
import { Slot } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { useMemo } from 'react';

import { ApiProvider } from '@/api/api-provider';
import { apiUrl } from '@/api/api-url';
import { I18nProvider } from '@/i18n/i18n-provider';
import { useDeviceLanguage } from '@/i18n/use-device-language';
import '@/global.css';

export default function RootLayout() {
  // Sign-in (a later ticket) supplies the token.
  const api = useMemo(() => createApi({ baseUrl: apiUrl(), getAccessToken: async () => null }), []);
  const lang = useDeviceLanguage();
  return (
    <ApiProvider api={api}>
      <I18nProvider lang={lang}>
        <StatusBar style="auto" />
        <Slot />
      </I18nProvider>
    </ApiProvider>
  );
}
