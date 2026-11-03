import { StyleSheet } from 'react-native';

import { Brand } from '@/constants/theme';

/**
 * The pins both maps draw (phone: react-native-maps; browser: Google's
 * JavaScript API), so a pin looks the same on either.
 */
export const PIN = 30;
export const PIN_SELECTED = 38;

// The stops are numbered as in the timeline, on the gold of its thread. The
// route is deep gold: plain gold is too faint on Google's pale map tiles.
export const stopColors = { background: Brand.gold, text: Brand.ink };

export const pinStyles = StyleSheet.create({
  // A round badge: a stop's number, or (phones) a plan count.
  pin: {
    width: PIN,
    height: PIN,
    borderRadius: PIN / 2,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 2,
    borderColor: '#ffffff',
  },
  // The browser's count pin grows with its digits.
  countPin: {
    minWidth: PIN,
    height: PIN,
    borderRadius: PIN / 2,
    paddingHorizontal: 8,
    alignItems: 'center',
    justifyContent: 'center',
    borderWidth: 2,
    borderColor: '#ffffff',
  },
  // Bigger rather than scaled, so it stays inside the measured box (phones).
  selected: { width: PIN_SELECTED, height: PIN_SELECTED, borderRadius: PIN_SELECTED / 2, borderColor: Brand.ink },
  // The browser's selected pin: HTML can be scaled freely.
  selectedWeb: { borderColor: Brand.ink, transform: [{ scale: 1.2 }] },
  label: { fontSize: 13, fontWeight: '700' },
  // Tall enough for a walk's stops (a kilometre or two, often north to
  // south) to show apart rather than stacked at district scale.
  routeFrame: { height: 320, borderRadius: 16, overflow: 'hidden' },
  halo: { width: 20, height: 20, borderRadius: 10, backgroundColor: 'rgba(222, 169, 44, 0.3)', alignItems: 'center', justifyContent: 'center' },
  walker: { width: 10, height: 10, borderRadius: 5, backgroundColor: Brand.gold },
});
