import { useEffect, useState } from 'react';

/**
 * The current time, updated every minute, for screens that depend on it
 * (an event's answer deadline, its meeting-point reveal). Reading the clock
 * during render would be impure, and a screen left open wouldn't notice a
 * deadline passing.
 */
export function useNow(intervalMs = 60_000) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const timer = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(timer);
  }, [intervalMs]);
  return now;
}
