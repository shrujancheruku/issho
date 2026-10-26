import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';

import { useI18n } from '@/i18n/i18n-provider';
import { api } from '@/lib/api';

import { readStored, writeStored } from './catalog-cache';

// Reference data for the profile steps, named in the current display
// languages. Keyed by language so a language change refetches.
//
// Cities and activities are also kept on the device (catalog-cache.ts): the
// app opens with last time's names and refreshes them once they're an hour
// old, instead of waiting for them on every launch.
const catalogStale = 60 * 60_000;

/** Last time's copy for a query, read from the device once per key. */
function useStored<T>(key: string) {
  const stored = useMemo(() => readStored<T>(key), [key]);
  return { initialData: stored?.value, initialDataUpdatedAt: stored?.at };
}

export function useCities() {
  const { languages } = useI18n();
  const key = `cities-${languages.primary}-${languages.secondary}`;
  const stored = useStored<Awaited<ReturnType<typeof api.catalog.listCities>>>(key);
  return useQuery({
    queryKey: ['cities', languages.primary, languages.secondary],
    queryFn: async () => {
      const res = await api.catalog.listCities({ locale: languages.primary, secondaryLanguage: languages.secondary });
      writeStored(key, res);
      return res;
    },
    ...stored,
    select: (r) => r.cities,
    staleTime: catalogStale,
  });
}

export function useActivities(cityId: string) {
  const { languages } = useI18n();
  const key = `activities-${cityId}-${languages.primary}-${languages.secondary}`;
  const stored = useStored<Awaited<ReturnType<typeof api.catalog.listActivities>>>(key);
  return useQuery({
    queryKey: ['activities', cityId, languages.primary, languages.secondary],
    queryFn: async () => {
      const res = await api.catalog.listActivities({ cityId, locale: languages.primary, secondaryLanguage: languages.secondary });
      writeStored(key, res);
      return res;
    },
    ...(cityId ? stored : {}),
    select: (r) => r.activities,
    enabled: !!cityId,
    staleTime: catalogStale,
  });
}

/**
 * Every language, named in the primary display language. With display, for
 * choosing a display language: Chinese is listed twice, Simplified (zh) and
 * Traditional (zh-Hant). Spoken languages list Chinese once.
 */
export function useLanguages(display = false) {
  const { languages } = useI18n();
  return useQuery({
    queryKey: ['languages', languages.primary, display],
    queryFn: () => api.catalog.listLanguages({ locale: languages.primary, display }),
    select: (r) => r.languages,
    staleTime: Infinity,
  });
}

/** "日本語 · Japanese", or just the autonym when both are the same. */
export function languageLabel(names: Map<string, { autonym: string; name: string }>, code: string) {
  const l = names.get(code);
  if (!l) return code;
  return l.autonym === l.name ? l.autonym : `${l.autonym} · ${l.name}`;
}
