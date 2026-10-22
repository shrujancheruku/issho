import { PushPlatform } from '@issho/api-client';
import Constants, { ExecutionEnvironment } from 'expo-constants';
import * as Device from 'expo-device';
import * as Notifications from 'expo-notifications';
import { AndroidImportance } from 'expo-notifications';
import { useEffect } from 'react';
import { Platform } from 'react-native';

import { api } from '@/lib/api';

/**
 * Push notifications through Expo (a plan you signed up for has a date;
 * your group is going ahead or cancelled). The server sends them; this file
 * gets the device's Expo push token and registers it with the API.
 *
 * Remote push needs a development or store build with an EAS project id
 * and push credentials (see apps/mobile/README.md, "Push notifications").
 * Everywhere else (Expo Go, simulators, the web, no project id) it does
 * nothing, quietly: the app works the same without notifications.
 */

// Shown while the app is open, too: the person may be looking at the feed
// when their group is confirmed.
Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowBanner: true,
    shouldShowList: true,
    shouldPlaySound: true,
    shouldSetBadge: false,
  }),
});

/** The EAS project id push tokens are issued for, if the app has one. */
function projectId(): string | undefined {
  return Constants.expoConfig?.extra?.eas?.projectId ?? Constants.easConfig?.projectId;
}

/** Why push is skipped, in development builds only (the code never throws, so it would be silent). */
function devLog(...args: unknown[]) {
  if (__DEV__) console.log('[push]', ...args);
}

/** Whether this build can receive remote push at all. */
function pushCapable() {
  // Expo Go dropped remote push on Android in SDK 53.
  const expoGo = Constants.executionEnvironment === ExecutionEnvironment.StoreClient;
  const capable = Platform.OS !== 'web' && Device.isDevice && !expoGo && !!projectId();
  if (!capable) devLog('skipped:', { os: Platform.OS, isDevice: Device.isDevice, expoGo, projectId: projectId() });
  return capable;
}

let registered: string | null = null;

/**
 * Registers this device for the signed-in person. With ask, it asks for
 * permission if it hasn't been decided (call it right after something that
 * makes notifications worth having, like signing up for a plan); without,
 * it only registers when permission was already given. Returns whether the
 * device is registered. Never throws.
 */
export async function registerForPush({ ask }: { ask: boolean }): Promise<boolean> {
  if (!pushCapable()) return false;
  try {
    if (Platform.OS === 'android') {
      // Android 13+ doesn't ask for permission until a channel exists; the
      // server sends to this channel id.
      await Notifications.setNotificationChannelAsync('default', {
        name: 'Issho',
        importance: AndroidImportance.HIGH,
      });
    }
    let { granted, canAskAgain } = await Notifications.getPermissionsAsync();
    if (!granted && ask && canAskAgain) ({ granted } = await Notifications.requestPermissionsAsync());
    if (!granted) {
      devLog('no permission', { ask, canAskAgain });
      return false;
    }
    const { data: token } = await Notifications.getExpoPushTokenAsync({ projectId: projectId() });
    if (token === registered) return true;
    await api.push.registerPushToken({
      token,
      platform: Platform.OS === 'ios' ? PushPlatform.IOS : PushPlatform.ANDROID,
    });
    registered = token;
    devLog('registered');
    return true;
  } catch (e) {
    // Offline, or the token service failed: the next app start tries again.
    devLog('failed:', e instanceof Error ? e.message : e);
    return false;
  }
}

/** Stops notifications to this device; call before signing out. Never throws. */
export async function unregisterPush() {
  const token = registered;
  if (!token) return;
  registered = null;
  try {
    await api.push.unregisterPushToken({ token });
  } catch {
    // The token stays with the old account until someone else registers it.
  }
}

/**
 * Opens the event a tapped notification is about, including the one that
 * launched the app. Mount where the person is signed in and onboarded.
 */
export function useNotificationTaps(open: (eventId: string) => void) {
  const response = Notifications.useLastNotificationResponse();
  useEffect(() => {
    const id = response?.notification.request.content.data?.eventId;
    if (typeof id !== 'string') return;
    open(id);
    // Otherwise the same notification would open the event again next launch.
    void Notifications.clearLastNotificationResponseAsync().catch(() => {});
  }, [response, open]);
}
