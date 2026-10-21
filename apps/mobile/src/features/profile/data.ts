import { useQuery } from '@tanstack/react-query';

import { useI18n } from '@/i18n/i18n-provider';
import { api } from '@/lib/api';

// Reference data for the profile steps, named in the current display
// languages. Keyed by language so a language change refetches.

export function useCities() {
  const { languages } = useI18n();
  return useQuery({
    queryKey: ['cities', languages.primary, languages.secondary],
    queryFn: () => api.catalog.listCities({ locale: languages.primary, secondaryLanguage: languages.secondary }),
    select: (r) => r.cities,
    staleTime: 60 * 60_000,
  });
}

export function useActivities(cityId: string) {
  const { languages } = useI18n();
  return useQuery({
    queryKey: ['activities', cityId, languages.primary, languages.secondary],
    queryFn: () =>
      api.catalog.listActivities({ cityId, locale: languages.primary, secondaryLanguage: languages.secondary }),
    select: (r) => r.activities,
    enabled: !!cityId,
    staleTime: 60 * 60_000,
  });
}

export function useLanguages() {
  const { languages } = useI18n();
  return useQuery({
    queryKey: ['languages', languages.primary],
    queryFn: () => api.catalog.listLanguages({ locale: languages.primary }),
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
