import { useSyncExternalStore } from 'react';
import { Appearance, Platform } from 'react-native';

import { readStored, writeStored } from './storage';

/** Light, dark, or whatever the phone (or browser) is set to. */
export type AppearancePreference = 'system' | 'light' | 'dark';

const valid = (v: string | undefined): v is AppearancePreference => v === 'system' || v === 'light' || v === 'dark';

let current: AppearancePreference = (() => {
  const stored = readStored();
  return valid(stored) ? stored : 'system';
})();
const listeners = new Set<() => void>();

/**
 * Tells the platform, so what React Native doesn't draw follows too: native
 * tabs, alerts, keyboards and pickers on phones; scrollbars and form
 * controls on the web (global.css says `color-scheme: light dark`).
 */
function apply(pref: AppearancePreference) {
  if (Platform.OS === 'web') {
    if (typeof document !== 'undefined') document.documentElement.style.colorScheme = pref === 'system' ? 'light dark' : pref;
    return;
  }
  Appearance.setColorScheme(pref === 'system' ? 'unspecified' : pref);
}
// At import, before the first render, so nothing flashes in the other theme.
apply(current);

export function setAppearance(pref: AppearancePreference) {
  if (pref === current) return;
  current = pref;
  apply(pref);
  writeStored(pref);
  listeners.forEach((l) => l());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

/** The person's choice (You › Appearance). Defaults to the system's. */
export function useAppearance(): AppearancePreference {
  return useSyncExternalStore(subscribe, () => current, () => current);
}
