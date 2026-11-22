/**
 * The Go API's address this build was given. EXPO_PUBLIC_API_URL is public (it
 * is bundled), and read where it is used so a test can set it.
 */
export function apiUrl(): string {
  return process.env.EXPO_PUBLIC_API_URL ?? 'http://localhost:8080';
}

/**
 * Whether an API at this address holds no real people: one on this machine or
 * the local network (an address, not a name), or dev's (`dev.joinissho.com`,
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
  if (host === 'localhost' || host.startsWith('[') || /^\d{1,3}(\.\d{1,3}){3}$/.test(host)) return true;
  const first = host.split('.')[0];
  return first === 'dev' || first.endsWith('-dev');
}
