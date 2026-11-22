import { createContext, useContext, useMemo, type ReactNode } from 'react';

import { translate, type Language, type StringKey } from './strings';

type T = (key: StringKey, params?: Record<string, string | number>) => string;
const I18nContext = createContext<{ lang: Language; t: T } | null>(null);

/** The display language for everything below it; a person has exactly one. */
export function I18nProvider({ lang, children }: { lang: Language; children: ReactNode }) {
  const value = useMemo(() => ({ lang, t: ((key, params) => translate(lang, key, params)) as T }), [lang]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const value = useContext(I18nContext);
  if (!value) throw new Error('useI18n needs an I18nProvider above it');
  return value;
}
