// localStorage can throw (private windows, blocked site data): fall back to "system".
export function readStored(): string | undefined {
  try {
    return globalThis.localStorage?.getItem('issho.appearance') ?? undefined;
  } catch {
    return undefined;
  }
}

export function writeStored(value: string) {
  try {
    globalThis.localStorage?.setItem('issho.appearance', value);
  } catch {
    // Only a preference.
  }
}
