import { useQuery } from '@tanstack/react-query';
import { useLocales } from 'expo-localization';
import { createContext, useCallback, useContext, useMemo, useState, type PropsWithChildren } from 'react';

import { useAuth } from '@/features/auth/auth-provider';
import { api } from '@/lib/api';

import { readCached, writeCached, type CachedStrings } from './cache';
import { deviceLanguages } from './device';
import { coversApp, english, lookup, type Params, type StringKey } from './strings';

export type Languages = { primary: string; secondary: string };

type I18nState = {
  /** The display languages in effect ('' = no secondary). */
  languages: Languages;
  /** The device's preferred languages (deviceLanguages): where onboarding starts. */
  device: Languages;
  /** Which of the two languages longer text is shown in (the switch). */
  showing: 'primary' | 'secondary';
  toggleShowing: () => void;
  /**
   * Shows the app in other languages before they're saved, e.g. while the
   * language step of onboarding is open. null goes back to the account's.
   */
  previewLanguages: (langs: Languages | null) => void;
  /** UI text in the language being shown. */
  t: (key: StringKey, params?: Params) => string;
  /** UI text in both languages, for short labels shown side by side. */
  pair: (key: StringKey, params?: Params) => { primary: string; secondary: string };
};

const I18nContext = createContext<I18nState | null>(null);

/**
 * The language rules for the app's own text (see docs/architecture.md#languages):
 *   - signed out (sign-in screens): the device's most preferred language that
 *     we support (deviceLanguages)
 *   - signed in: the profile's primary + secondary language (from GetMe),
 *     or the device language until onboarding sets them
 * Short labels show both languages side by side (pair); longer text shows one
 * at a time, with a switch (t, showing).
 */
export function I18nProvider({ children }: PropsWithChildren) {
  const { session, languages: account } = useAuth();
  // useLocales re-renders when the device language changes (Android can
  // change it while the app runs).
  const locales = useLocales();
  const { primary: devicePrimary, secondary: deviceSecondary } = deviceLanguages(locales);
  const [preview, setPreview] = useState<{ userId: string; langs: Languages } | null>(null);
  const [showing, setShowing] = useState<'primary' | 'secondary'>('primary');

  // A preview belongs to the person who started it: after a sign-out or a
  // switch of account it no longer applies.
  const userId = session?.user.id;
  const previewing = preview && preview.userId === userId ? preview.langs : null;
  const previewLanguages = useCallback(
    (langs: Languages | null) => setPreview(langs && userId ? { userId, langs } : null),
    [userId],
  );

  // Sign-in is shown in one language: nobody has chosen a second yet.
  const languages: Languages = session
    ? (previewing ?? account ?? { primary: devicePrimary, secondary: '' })
    : { primary: devicePrimary, secondary: '' };
  const primary = useUiStrings(languages.primary);
  const secondary = useUiStrings(languages.secondary);
  const shown = showing === 'secondary' && languages.secondary ? 'secondary' : 'primary';

  const t = useCallback(
    (key: StringKey, params?: Params) =>
      lookup(key, shown === 'primary' ? [primary, secondary] : [secondary, primary], params),
    [shown, primary, secondary],
  );
  const pair = useCallback(
    (key: StringKey, params?: Params) => {
      const p = lookup(key, [primary, secondary], params);
      const s = languages.secondary ? lookup(key, [secondary], params) : '';
      return { primary: p, secondary: s === p ? '' : s };
    },
    [primary, secondary, languages.secondary],
  );

  const value = useMemo<I18nState>(
    () => ({
      languages,
      device: { primary: devicePrimary, secondary: deviceSecondary },
      showing: shown,
      toggleShowing: () => setShowing((s) => (s === 'primary' ? 'secondary' : 'primary')),
      previewLanguages,
      t,
      pair,
    }),
    // languages is rebuilt each render; its fields are what matter.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [languages.primary, languages.secondary, devicePrimary, deviceSecondary, shown, t, pair, previewLanguages],
  );
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error('useI18n must be used inside <I18nProvider>');
  return ctx;
}

/**
 * The UI catalogue in one language: from the device cache straight away, then
 * from the server when the cache is missing, incomplete or older than this
 * build's keys. While the server is still translating, it asks again every
 * 30 seconds. Missing strings fall back (lookup), so this never blocks the UI.
 */
function useUiStrings(lang: string) {
  const needed = !!lang && lang !== 'en';
  const query = useQuery({
    queryKey: ['ui-strings', lang],
    enabled: needed,
    queryFn: async (): Promise<CachedStrings> => {
      const res = await api.ui.getStrings({ language: lang });
      const value = { strings: res.strings, complete: res.complete, catalogVersion: res.catalogVersion };
      writeCached(lang, value);
      return value;
    },
    initialData: () => (needed ? readCached(lang) : undefined),
    // A complete cache with every key needs no request; anything else is
    // stale from the start.
    initialDataUpdatedAt: () => {
      const cached = needed ? readCached(lang) : undefined;
      return cached?.complete && coversApp(cached.strings) ? Date.now() : 0;
    },
    staleTime: Infinity,
    gcTime: Infinity,
    refetchInterval: (q) => (q.state.data && !q.state.data.complete ? 30_000 : false),
  });
  if (lang === 'en') return english;
  return query.data?.strings;
}
