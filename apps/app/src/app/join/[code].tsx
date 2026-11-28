import { useLocalSearchParams, useRouter } from 'expo-router';
import { useEffect } from 'react';

import { setPendingJoin } from '@/join/pending-join';

/**
 * Where a join link lands (`issho://join/<code>` on a development build, the
 * web address's `/join/<code>`). It takes the code out of the address at once,
 * into memory, and moves on to `/join`, so the code is not left in the URL bar
 * or in history, nor carried in the return path through sign-in. The server
 * render shows nothing: there is no one to admit until the browser runs.
 */
export default function JoinWithCode() {
  const { code } = useLocalSearchParams<{ code: string }>();
  const router = useRouter();
  useEffect(() => {
    if (typeof code === 'string' && code) setPendingJoin(code);
    router.replace('/join');
  }, [code, router]);
  return null;
}
