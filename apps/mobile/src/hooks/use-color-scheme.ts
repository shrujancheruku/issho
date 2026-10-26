import { useColorScheme as useSystemScheme } from 'react-native';

import { useAppearance } from '@/features/appearance/preference';

/**
 * The theme to draw: the person's choice, or the system's. On phones
 * Appearance.setColorScheme (features/appearance) already makes React
 * Native report the choice; reading it here too keeps one rule everywhere.
 */
export function useColorScheme(): 'light' | 'dark' {
  const pref = useAppearance();
  const system = useSystemScheme();
  if (pref !== 'system') return pref;
  return system === 'dark' ? 'dark' : 'light';
}
