import { useMemo } from 'react';

import { languageLabel, useActivities, useCities, useLanguages } from '@/features/profile/data';
import { useProfile } from '@/features/profile/use-profile';

export type Names = { name: string; secondary: string };

/**
 * Plans carry ids only; this joins them to the names the app already loads
 * (in both display languages) from CatalogService for the person's city.
 * Unknown ids fall back to the id, so a plan never shows blank.
 */
export function usePlanNames() {
  const profile = useProfile();
  const cityId = profile.data?.profile?.cityId ?? '';
  const activities = useActivities(cityId);
  const cities = useCities();
  const languages = useLanguages();
  return useMemo(() => {
    const acts = new Map((activities.data ?? []).map((a) => [a.id, { name: a.name, secondary: a.secondaryName }]));
    const areas = new Map(
      (cities.data ?? []).flatMap((c) => c.areas).map((a) => [a.id, { name: a.name, secondary: a.secondaryName }]),
    );
    const langs = new Map((languages.data ?? []).map((l) => [l.code, l]));
    return {
      ready: !!activities.data && !!cities.data,
      activity: (id: string): Names => acts.get(id) ?? { name: id, secondary: '' },
      area: (id: string): Names => areas.get(id) ?? { name: id, secondary: '' },
      language: (code: string) => languageLabel(langs, code),
    };
  }, [activities.data, cities.data, languages.data]);
}
