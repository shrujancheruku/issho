import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useLocales } from 'expo-localization';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type PropsWithChildren } from 'react';
import { AppState } from 'react-native';

import { useAuth } from '@/features/auth/auth-provider';
import { api } from '@/lib/api';

import { fromBundle } from './bundle';
import { readCached, writeCached, type CachedStrings } from './cache';
import { deviceLanguages } from './device';
import { catalogFingerprint, displayEnglish, lookup, type Params, type StringKey } from './strings';

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

/**
 * The language text is being shown in, for ThemedText's typeface; English
 * outside the provider (it never throws, unlike useI18n).
 */
export function useShownLanguage(): string {
  const ctx = useContext(I18nContext);
  if (!ctx) return 'en';
  return ctx.showing === 'secondary' && ctx.languages.secondary ? ctx.languages.secondary : ctx.languages.primary;
}

/** The other display language (what side-by-side secondary text is in), or ''. */
export function useOtherLanguage(): string {
  const ctx = useContext(I18nContext);
  if (!ctx || !ctx.languages.secondary) return '';
  return ctx.showing === 'secondary' ? ctx.languages.primary : ctx.languages.secondary;
}

export function useI18n() {
  const ctx = useContext(I18nContext);
  if (!ctx) throw new Error('useI18n must be used inside <I18nProvider>');
  return ctx;
}

/**
 * Complete, and fetched for this build's English text. Not "every key
 * present": a translation that lost a placeholder is left out for good, and
 * requiring every key made such a language download again at every launch.
 * Keys the API doesn't have yet (an app newer than the API) arrive with the
 * hourly check once it's deployed.
 */
function isCurrent(c: CachedStrings | undefined) {
  return !!c && c.complete && c.appCatalog === catalogFingerprint;
}

/**
 * How often a current catalogue is checked with the server: a reviewer's
 * correction, a new human version or a better translation changes the text
 * without a new build. A check that finds nothing new costs no strings.
 */
const CHECK_EVERY = 60 * 60 * 1000;

/**
 * The UI catalogue in one language: from the device cache straight away (or,
 * on a fresh install, the copy bundled for the city packs' languages), then
 * from the server when the cache is missing, incomplete or older than this
 * build's keys. While the server is still translating, it asks again every
 * 30 seconds; a current copy is checked hourly, including when the app comes
 * back to the foreground. Missing strings fall back (lookup), so this never
 * blocks the UI.
 */
function useUiStrings(lang: string) {
  const needed = !!lang && lang !== 'en';
  const queryClient = useQueryClient();
  const cached = useMemo(() => (needed ? (readCached(lang) ?? fromBundle(lang)) : undefined), [needed, lang]);
  const query = useQuery({
    queryKey: ['ui-strings', lang],
    enabled: needed,
    queryFn: async (): Promise<CachedStrings> => {
      // A current copy is sent by version: if it still is, the server says so
      // rather than sending every string again.
      const had = queryClient.getQueryData<CachedStrings>(['ui-strings', lang]);
      const res = await api.ui.getStrings({ language: lang, knownVersion: isCurrent(had) ? had!.catalogVersion : '' });
      const value: CachedStrings =
        res.unchanged && had
          ? { ...had, checkedAt: Date.now() }
          : {
              strings: res.strings,
              complete: res.complete,
              catalogVersion: res.catalogVersion,
              appCatalog: catalogFingerprint,
              checkedAt: Date.now(),
            };
      writeCached(lang, value);
      return value;
    },
    initialData: cached,
    // The cached copy is as old as its last check; caches from before checks
    // existed count as old and are checked straight away.
    initialDataUpdatedAt: cached?.checkedAt ?? 0,
    // A complete catalogue with every key, fetched for this build's English
    // text, is checked hourly; anything else is shown straight away but
    // fetched again.
    staleTime: (q) => (isCurrent(q.state.data) ? CHECK_EVERY : 0),
    gcTime: Infinity,
    refetchInterval: (q) => (!q.state.data ? false : q.state.data.complete ? CHECK_EVERY : 30_000),
  });

  // Timers don't run while the app is in the background, so check on the way
  // back to the foreground too, once the copy is an hour old.
  const { refetch, dataUpdatedAt } = query;
  useEffect(() => {
    if (!needed) return;
    const sub = AppState.addEventListener('change', (state) => {
      if (state === 'active' && Date.now() - dataUpdatedAt > CHECK_EVERY) void refetch();
    });
    return () => sub.remove();
  }, [needed, refetch, dataUpdatedAt]);

  if (lang === 'en') return displayEnglish;
  return query.data?.strings;
}
