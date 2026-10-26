import { useIsFocused } from 'expo-router';
import { useEffect, useState } from 'react';
import { AppState } from 'react-native';

/**
 * Whether the screen is on show: focused and the app in the foreground.
 * Loops pause when it isn't, so nothing animates behind another screen.
 */
export function useScreenActive() {
  const focused = useIsFocused();
  const [foreground, setForeground] = useState(AppState.currentState !== 'background');
  useEffect(() => {
    const sub = AppState.addEventListener('change', (s) => setForeground(s !== 'background'));
    return () => sub.remove();
  }, []);
  return focused && foreground;
}
