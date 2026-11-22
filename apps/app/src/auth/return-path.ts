/**
 * Where to go after signing in: the screen the person was sent from, when it is
 * a path inside the app. Anything else (another site, a protocol-relative
 * address, a script) is ignored, so a crafted sign-in link cannot send a
 * person somewhere else once they are signed in. A tab, newline or other
 * whitespace or control character before the second slash is refused too:
 * browsers drop some of them inside a URL, so "/\t/evil.example" would read as
 * "//evil.example".
 */
export function returnPath(next: string | string[] | undefined): string {
  const value = Array.isArray(next) ? next[0] : next;
  if (!value || !value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return '/';
  const second = value.indexOf('/', 1);
  const head = second === -1 ? value : value.slice(0, second + 1);
  if (/[\s\u0000-\u001f\u007f]/.test(head)) return '/';
  return value;
}
