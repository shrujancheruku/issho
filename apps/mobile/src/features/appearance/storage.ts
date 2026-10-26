import { File, Paths } from 'expo-file-system';

const file = () => new File(Paths.document, 'appearance.txt');

/**
 * Synchronous, like the string cache (i18n/cache.ts), so the first frame is
 * already in the chosen theme. A missing or unreadable file means "system".
 */
export function readStored(): string | undefined {
  try {
    const f = file();
    return f.exists ? f.textSync().trim() : undefined;
  } catch {
    return undefined;
  }
}

export function writeStored(value: string) {
  try {
    const f = file();
    if (!f.exists) f.create();
    f.write(value);
  } catch {
    // Only a preference: the next launch follows the system again.
  }
}
