/** What the app knows of a signed-in person: the token the API checks, and the email they signed in with. */
export interface Session {
  accessToken: string;
  email: string;
}

/**
 * The app's one view of Supabase Auth, so screens and tests never touch the
 * Supabase library (test-support/fake-session-client.ts is the fake).
 * Sign-in is a 6-digit email code and nothing else.
 */
export interface SessionClient {
  /** Names of the public build settings this client needs and was not given. Empty (or absent) when it can work. */
  readonly missingConfig?: readonly string[];
  /** The session the client holds, from storage after a restart; null when signed out. */
  getSession(): Promise<Session | null>;
  /** Called on every sign-in, sign-out and token refresh. Returns the way to stop listening. */
  onChange(listener: (session: Session | null) => void): () => void;
  /** Emails a code. Throws when it could not be sent, and a RateLimitedError when Auth is limiting sends. */
  sendCode(email: string): Promise<void>;
  /** Signs in with the code. False when the code is wrong or has expired; throws when it could not be checked (a RateLimitedError when Auth is limiting attempts). */
  verifyCode(email: string, code: string): Promise<boolean>;
  signOut(): Promise<void>;
}

/** Auth answered 429: too many codes sent or checked. Waiting is the only fix, so the screen says that instead of "something went wrong". */
export class RateLimitedError extends Error {
  constructor() {
    super('Too many sign-in attempts');
    this.name = 'RateLimitedError';
  }
}
