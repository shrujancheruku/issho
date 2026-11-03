import { focusManager } from '@tanstack/react-query';
import { AppState, Platform } from 'react-native';

/**
 * Makes returning to the app count as focus for TanStack Query, so queries
 * older than `staleTime` refetch (React Native has no window focus event;
 * web keeps the browser's). Deliberately no `onlineManager`: it would need a
 * network-state dependency. Returns the unsubscribe.
 */
export function bindFocusToAppState() {
  if (Platform.OS === 'web') return () => {};
  const sub = AppState.addEventListener('change', (status) => focusManager.setFocused(status === 'active'));
  return () => sub.remove();
}
