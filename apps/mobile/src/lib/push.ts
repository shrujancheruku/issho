import { PushPlatform } from '@issho/api-client';
import { useQueryClient } from '@tanstack/react-query';
import Constants, { ExecutionEnvironment } from 'expo-constants';
import * as Device from 'expo-device';
import * as Notifications from 'expo-notifications';
import { AndroidImportance } from 'expo-notifications';
import { useEffect } from 'react';
import { Platform } from 'react-native';

import { api } from '@/lib/api';
import type { NotificationTarget } from '@/lib/notification-target';
import { type PushRef, pushReceived } from '@/lib/queries';

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

/**
 * The token last registered, and for whom. Per account, not per device: the
 * same phone can be signed into another account next (the server moves the
 * token to whoever registers it last).
 */
let registered: { userId: string; token: string } | null = null;

/**
 * Registers this device for the signed-in person (userId). With ask, it asks for
 * permission if it hasn't been decided (call it right after something that
 * makes notifications worth having, like signing up for a plan); without,
 * it only registers when permission was already given. Returns whether the
 * device is registered. Never throws.
 */
export async function registerForPush({ ask, userId }: { ask: boolean; userId: string }): Promise<boolean> {
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
    if (registered?.userId === userId && registered.token === token) return true;
    await api.push.registerPushToken({
      token,
      platform: Platform.OS === 'ios' ? PushPlatform.IOS : PushPlatform.ANDROID,
    });
    registered = { userId, token };
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
  const current = registered;
  if (!current) return;
  registered = null;
  try {
    await api.push.unregisterPushToken({ token: current.token });
  } catch {
    // The token stays with the old account until someone else registers it.
  }
}

/** The kind and ids a notification carries (`backend/internal/notifications` `data()`). */
function pushData(data: Record<string, unknown> | undefined): { kind: unknown; ref: PushRef } {
  const id = (v: unknown) => (typeof v === 'string' ? v : undefined);
  return { kind: data?.kind, ref: { eventId: id(data?.eventId), planId: id(data?.planId), crewId: id(data?.crewId) } };
}

/**
 * Refreshes what a push made stale while the app is open, without a tap
 * (a confirmed group shows on the event and the list). The table is
 * `pushKinds` in lib/queries.ts. Mount where the person is signed in.
 */
export function usePushRefresh() {
  const qc = useQueryClient();
  useEffect(() => {
    const sub = Notifications.addNotificationReceivedListener((n) => {
      const { kind, ref } = pushData(n.request.content.data);
      void pushReceived(qc, kind, ref);
    });
    return () => sub.remove();
  }, [qc]);
}

/**
 * Opens what a tapped notification is about, including the one that
 * launched the app. Mount where the person is signed in and onboarded.
 */
export function useNotificationTaps(open: (target: NotificationTarget) => void) {
  const qc = useQueryClient();
  const response = Notifications.useLastNotificationResponse();
  useEffect(() => {
    const data = response?.notification.request.content.data;
    // A tap may be the first the app hears of it (it launched from the push): refresh too.
    if (response) {
      const { kind, ref } = pushData(data);
      void pushReceived(qc, kind, ref);
    }
    if (typeof data?.eventId === 'string') open({ eventId: data.eventId });
    else if (typeof data?.planId === 'string') open({ planId: data.planId });
    else if (typeof data?.crewId === 'string') open({ crewId: data.crewId });
    else return;
    // Otherwise the same notification would open the event again next launch.
    void Notifications.clearLastNotificationResponseAsync().catch(() => {});
  }, [response, open, qc]);
}
