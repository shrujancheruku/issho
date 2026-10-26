import * as Haptics from 'expo-haptics';

/**
 * The app's three haptics, from the motion plan: selection for chips and
 * grid cells, a light tap when a knot ties, success for checking in and a
 * crew starting. Nothing on errors. Failures (no haptic engine) are ignored.
 */
export const haptics = {
  selection: () => void Haptics.selectionAsync().catch(() => {}),
  tap: () => void Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light).catch(() => {}),
  success: () => void Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success).catch(() => {}),
};
