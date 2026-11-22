import { Slot } from 'expo-router';
import { StatusBar } from 'expo-status-bar';
import { useMemo } from 'react';

import { ApiProvider } from '@/api/api-provider';
import { createAppApi } from '@/api/app-api';
import { createAppSessionClient } from '@/auth/app-session-client';
import { AuthProvider } from '@/auth/auth-provider';
import { I18nProvider } from '@/i18n/i18n-provider';
import { useDeviceLanguage } from '@/i18n/use-device-language';
import '@/global.css';

export default function RootLayout() {
  const session = useMemo(() => createAppSessionClient(), []);
  const api = useMemo(() => createAppApi(session), [session]);
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
