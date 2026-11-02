/**
 * Web version of push.ts (Metro picks this file on web): no push
 * notifications in the browser yet. expo-notifications' native functions
 * aren't available there (useLastNotificationResponse throws), so it isn't
 * imported at all.
 */
import type { NotificationTarget } from '@/lib/notification-target';

export async function registerForPush(_: { ask: boolean; userId: string }): Promise<boolean> {
  return false;
}

export async function unregisterPush() {}

export function useNotificationTaps(_open: (target: NotificationTarget) => void) {}
