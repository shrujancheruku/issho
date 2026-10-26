import { useSyncExternalStore } from 'react';
import { useColorScheme as useSystemScheme } from 'react-native';

import { useAppearance } from '@/features/appearance/preference';

const noopSubscribe = () => () => {};

/**
 * The person's choice, or the browser's. The system value is only known on
 * the client: useSyncExternalStore returns the server snapshot (false)
 * during static rendering and hydration, then the client one, without a
 * setState-in-effect re-render.
 */
export function useColorScheme(): 'light' | 'dark' {
  const hasHydrated = useSyncExternalStore(
    noopSubscribe,
    () => true,
    () => false,
  );
  const pref = useAppearance();
  const system = useSystemScheme();
  if (pref !== 'system') return pref;
  return hasHydrated && system === 'dark' ? 'dark' : 'light';
}
