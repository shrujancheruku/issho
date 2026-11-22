import { Platform, Share } from 'react-native';

/** What happened to a link the member asked to share: `copied` is the web's fallback, where there is no share sheet to open. */
export type ShareResult = 'shared' | 'copied' | 'failed';

/** Opens the phone's share sheet with `message`; on the web, the browser's, or else copies `url` for the member to paste. */
export async function shareJoinLink(message: string, url: string): Promise<ShareResult> {
  try {
    if (Platform.OS !== 'web') {
      await Share.share({ message });
      return 'shared';
    }
    if (typeof navigator !== 'undefined' && typeof navigator.share === 'function') {
      await navigator.share({ text: message });
      return 'shared';
    }
    await navigator.clipboard.writeText(url);
    return 'copied';
  } catch (err) {
    // Closing the sheet is how a person says no; it is not a failure to tell them about.
    if (err instanceof Error && err.name === 'AbortError') return 'shared';
    return 'failed';
  }
}
