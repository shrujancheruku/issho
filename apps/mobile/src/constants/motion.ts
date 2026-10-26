import { Easing } from 'react-native-reanimated';

/**
 * Motion tokens, from the motion plan for the redesign. Components take
 * their timings from here (through hooks/use-motion) and never write their
 * own numbers.
 *
 * The rules behind them: move when people move (loops only on something
 * live), one live thing per screen, the button becomes its result, animate
 * changes rather than screens, joining things spring while serious things
 * (report, leave, delete, errors) don't, and reduced motion is a full
 * design of its own.
 */
export const Motion = {
  /** Every tappable thing, on touch down. */
  press: { duration: 100, scale: 0.97 },
  /** Colour, text and icon swaps; a button becoming its result. */
  change: 180,
  /** Things entering, leaving or changing places in a list. */
  move: 280,
  /** The thread growing, per segment. */
  draw: 450,
  /** Knots tying, avatars arriving, a crew starting: a small overshoot. */
  gather: { damping: 12, stiffness: 180 },
  /** Sheets and drags: no overshoot. */
  settle: { damping: 20, stiffness: 200 },
  /** The one live thing on a screen. */
  breathe: 2400,
  /** Map pins dropping in one after another, the first time a map opens. */
  pinStagger: 45,
  /** A route drawing itself, whatever its length. */
  routeDraw: 1600,
  /** The walker strolling a route, then resting before it goes again. */
  walk: 5200,
  walkRest: 1800,
} as const;

/** Ease out: quick to respond, gentle to land. */
export const EaseOut = Easing.bezier(0.2, 0.8, 0.2, 1);
