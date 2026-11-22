/**
 * The Go API's address this build was given. EXPO_PUBLIC_API_URL is public (it
 * is bundled), and read where it is used so a test can set it.
 */
export function apiUrl(): string {
  return process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';
}

/**
 * Whether an API at this address holds no real people: one on this machine or
 * the local network (a loopback or private address, not a name; a public one is
 * production's), or dev's (`dev.joinissho.com`,
 * `api-dev.joinissho.com`: a first label of `dev` or ending in `-dev`). The build
 * knows nothing else about where it runs, and this decides from what it must
 * know anyway. An address it cannot tell is treated as production's, so a new
 * name hides what is behind this until it is listed here.
 */
export function isDevApi(url: string): boolean {
  let host: string;
  try {
    host = new URL(url).hostname;
  } catch {
    return false;
  }
  if (host === 'localhost' || isLocalAddress(host)) return true;
  const first = host.split('.')[0];
  return first === 'dev' || first.endsWith('-dev');
}

/** A loopback or private IPv4 literal, or a loopback, unique-local or link-local IPv6 one in brackets. */
function isLocalAddress(host: string): boolean {
  const v4 = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.\d{1,3}$/.exec(host);
  if (v4) {
    const [a, b] = [Number(v4[1]), Number(v4[2])];
    return a === 127 || a === 10 || (a === 172 && b >= 16 && b <= 31) || (a === 192 && b === 168) || (a === 169 && b === 254);
  }
  if (!host.startsWith('[')) return false;
  const v6 = host.slice(1, -1).toLowerCase();
  return v6 === '::1' || /^f[cd][0-9a-f]{2}:/.test(v6) || /^fe[89ab][0-9a-f]:/.test(v6);
}
