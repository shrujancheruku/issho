/**
 * Supabase session storage for the web build (Metro picks this file over
 * secure-storage.ts on web).
 *
 * Browsers have no equivalent of the Keychain/Keystore, so the session lives
 * in localStorage, as supabase-js does by default. Unlike on a phone, any
 * script running on the page could read the refresh token. That's acceptable
 * for development builds; before a public web release, decide between this
 * plus a strict Content-Security-Policy (no third-party scripts) and
 * cookie-based sessions (see docs/ROADMAP.md, "Web release").
 *
 * Every call is guarded: localStorage is missing during a build and can throw
 * in private browsing. A failure reads as "no session", so the person signs in
 * again rather than seeing an error.
 */
function storage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage;
  } catch {
    return null;
  }
}

export const secureStorage = {
  async getItem(key: string): Promise<string | null> {
    try {
      return storage()?.getItem(key) ?? null;
    } catch {
      return null;
    }
  },
  async setItem(key: string, value: string): Promise<void> {
    try {
      storage()?.setItem(key, value);
    } catch {
      // Full or blocked: the session lasts until the page is closed.
    }
  },
  async removeItem(key: string): Promise<void> {
    try {
      storage()?.removeItem(key);
    } catch {
      // Nothing stored.
    }
  },
};
