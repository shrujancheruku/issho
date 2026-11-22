import { Redirect, usePathname } from 'expo-router';
import type { ReactNode } from 'react';

import { useAuth } from '@/auth/auth-provider';

/**
 * Wraps a signed-in screen, which says its own `path`. A signed-out visitor is
 * sent to sign-in with that path, which sign-in returns them to. While the
 * stored session is being read it shows nothing, so a restart does not bounce a
 * signed-in person. The server render is the same nothing: signed-in screens
 * render in the browser (docs/spec/architecture.md, "The web at one address").
 */
export function RequireSignIn({ path, children }: { path: '/together' | '/you'; children: ReactNode }) {
  const auth = useAuth();
  // A tab stays mounted after the person leaves it (signing out from You goes to Plans), so only the screen in view sends them anywhere.
  const inView = usePathname() === path;
  if (auth.status === 'loading') return null;
  if (auth.status === 'signedOut') return inView ? <Redirect href={{ pathname: '/sign-in', params: { next: path } }} /> : null;
  return <>{children}</>;
}
