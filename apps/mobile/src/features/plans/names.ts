import { useMemo } from 'react';

import { useAuth } from '@/features/auth/auth-provider';
import { languageLabel, useActivities, useCities, useLanguages } from '@/features/profile/data';
import { useProfile } from '@/features/profile/use-profile';
import { useI18n } from '@/i18n/i18n-provider';

export type Names = { name: string; secondary: string };

/**
 * Plans carry ids only; this joins them to the names the app already loads
 * (in both display languages) from CatalogService for the person's city.
 * Unknown ids fall back to the id, so a plan never shows blank. Names come
 * in the order of the language being shown, so the language switch puts the
 * other language first.
 */
export function usePlanNames() {
  const profile = useProfile();
  // The city from GetMe, so activities load without waiting for the profile
  // (a request less in the chain on opening the app); the profile's for an
  // API from before GetMe sent it.
  const cityId = useAuth().cityId || (profile.data?.profile?.cityId ?? '');
  const activities = useActivities(cityId);
  const cities = useCities();
  const languages = useLanguages();
  const { showing } = useI18n();
  return useMemo(() => {
    const order = (n: Names): Names =>
      showing === 'secondary' && n.secondary ? { name: n.secondary, secondary: n.name } : n;
    const acts = new Map((activities.data ?? []).map((a) => [a.id, { name: a.name, secondary: a.secondaryName }]));
    const categories = new Map((activities.data ?? []).map((a) => [a.id, a.category]));
    const areas = new Map(
      (cities.data ?? []).flatMap((c) => c.areas).map((a) => [a.id, { name: a.name, secondary: a.secondaryName }]),
    );
    const langs = new Map((languages.data ?? []).map((l) => [l.code, l]));
    const coords = new Map((cities.data ?? []).flatMap((c) => c.areas).map((a) => [a.id, { lat: a.lat, lng: a.lng }]));
    return {
      ready: !!activities.data && !!cities.data,
      activity: (id: string): Names => order(acts.get(id) ?? { name: id, secondary: '' }),
      area: (id: string): Names => order(areas.get(id) ?? { name: id, secondary: '' }),
      /** The activity's category ("food", "outdoors"), for Plans' filters; "" if unknown. */
      category: (id: string) => categories.get(id) ?? '',
      language: (code: string) => languageLabel(langs, code),
      /** Area centres, for the map of plans. */
      coords,
      /** The person's home area centre: where the map starts when there are no pins. */
      home: coords.get(profile.data?.profile?.homeAreaId ?? ''),
    };
  }, [activities.data, cities.data, languages.data, showing, profile.data]);
}
