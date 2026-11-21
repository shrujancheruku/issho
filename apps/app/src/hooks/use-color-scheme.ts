import { useColorScheme as useSystemScheme } from 'react-native';

/**
 * The theme to draw: the device's light or dark setting. A person's own choice
 * in You comes with that screen.
 */
export function useColorScheme(): 'light' | 'dark' {
  return useSystemScheme() === 'dark' ? 'dark' : 'light';
}
