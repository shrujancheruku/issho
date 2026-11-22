import type { ReactNode } from 'react';

import { ApiProvider } from '@/api/api-provider';
import { I18nProvider } from '@/i18n/i18n-provider';
import type { Language } from '@/i18n/strings';

import type { fakeApi } from './fake-api';

/** What the root layout provides, with a chosen language and a fake API. */
export function TestProviders({ lang, api, children }: { lang: Language; api: ReturnType<typeof fakeApi>; children: ReactNode }) {
  return (
    <ApiProvider api={api as never}>
      <I18nProvider lang={lang}>{children}</I18nProvider>
    </ApiProvider>
  );
}
