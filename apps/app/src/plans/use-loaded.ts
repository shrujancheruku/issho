import { useCallback, useEffect, useState } from 'react';

import { useI18n } from '@/i18n/i18n-provider';
import type { Language } from '@/i18n/strings';

type State<T> = { status: 'loading' } | { status: 'failed' } | { status: 'ready'; data: T };
type Fetched<T> = { lang: Language; key: string; attempt: number; state: State<T> };

/**
 * Loads a screen's data in the display language, starting from what the server
 * already fetched when it was fetched in this language. The server asks in
 * English (it doesn't know the visitor), so a Vietnamese reader's browser takes
 * the server's HTML first and then fetches the Vietnamese text. A server that
 * could not fetch hands over nothing (`seed` is undefined), and the screen
 * fetches as a phone does. `retry` tries again after a failure.
 */
export function useLoaded<T extends { lang: Language }>(load: (lang: Language) => Promise<T>, seed: T | undefined, key: string) {
  const { lang } = useI18n();
  const [attempt, setAttempt] = useState(0);
  const [fetched, setFetched] = useState<Fetched<T> | null>(null);
  const seeded = seed && seed.lang === lang && attempt === 0 ? seed : undefined;

  useEffect(() => {
    if (seeded) return;
    let current = true;
    const done = (state: State<T>) => current && setFetched({ lang, key, attempt, state });
    load(lang).then(
      (data) => done({ status: 'ready', data }),
      () => done({ status: 'failed' }),
    );
    return () => {
      current = false;
    };
    // `load` is rebuilt every render; what it asks for is named by `key` and `lang`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lang, key, attempt, !!seeded]);

  const retry = useCallback(() => setAttempt((n) => n + 1), []);

  let state: State<T> = { status: 'loading' };
  if (seeded) state = { status: 'ready', data: seeded };
  else if (fetched) {
    const fresh = fetched.lang === lang && fetched.key === key && fetched.attempt === attempt;
    // Between languages (or plans) keep what is showing, so the page doesn't blank while the next text arrives.
    if (fresh || (fetched.key === key && fetched.state.status === 'ready')) state = fetched.state;
  }
  return { state, retry };
}
