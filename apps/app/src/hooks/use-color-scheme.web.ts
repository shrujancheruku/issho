import { useSyncExternalStore } from 'react';
import { useColorScheme as useSystemScheme } from 'react-native';

const noopSubscribe = () => () => {};

/**
 * The browser's setting. It is only known on the client, so the server render
 * and the hydration pass draw light, then the client's value follows:
 * useSyncExternalStore returns the server snapshot (false) for both, without a
 * setState-in-effect re-render.
 */
export function useColorScheme(): 'light' | 'dark' {
  const hasHydrated = useSyncExternalStore(
    noopSubscribe,
    () => true,
    () => false,
  );
  const system = useSystemScheme();
  return hasHydrated && system === 'dark' ? 'dark' : 'light';
}
