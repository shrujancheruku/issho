/**
 * Where to go after signing in: the screen the person was sent from, when it is
 * a path inside the app. Anything else (another site, a protocol-relative
 * address, a script) is ignored, so a crafted sign-in link cannot send a
 * person somewhere else once they are signed in.
 */
export function returnPath(next: string | string[] | undefined): string {
  const value = Array.isArray(next) ? next[0] : next;
  if (!value || !value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return '/';
  return value;
}
