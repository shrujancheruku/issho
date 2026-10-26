import { useState } from 'react';

/**
 * The city a page shows: the one last chosen there (kept in localStorage
 * under key), or the first city once the list has loaded if that one's
 * gone. Derived while rendering rather than corrected in an effect, which
 * would render the page twice.
 */
export function useRememberedCity(key: string, cities: { id: string }[] | undefined): [string, (id: string) => void] {
  const [chosen, setChosen] = useState(() => localStorage.getItem(key) ?? '');
  const id = cities?.length && !cities.some((c) => c.id === chosen) ? cities[0].id : chosen;
  const choose = (next: string) => {
    setChosen(next);
    localStorage.setItem(key, next);
  };
  return [id, choose];
}
