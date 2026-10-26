import { useEffect, useMemo, useState } from 'react';

import { Motion } from '@/constants/motion';
import { useMotion } from '@/hooks/use-motion';
import { useScreenActive } from '@/hooks/use-screen-active';

import { dropDelay } from './drop-in';
import { easeInOut, easeInOutInverse, measurePath, pathUpTo, pointAlong, stopFractions, type LatLng, type MeasuredPath } from './route-path';

// Routes already drawn this session (by their polyline): opening the same
// plan again shows the route straight away, as the motion plan asks.
const drawn = new Set<string>();

// Map overlays can't be animated on the UI thread, so the line and the
// walker are moved by re-rendering: about 30 updates a second for the line
// (it's over in 1.6 s) and 20 for the walker, which is slow enough not to
// need more.
const DRAW_FRAME = 33;
const WALK_FRAME = 50;

/**
 * A route drawing itself from the meeting point over 1.6 s, once per
 * session, with each stop popping in as the line reaches it. With reduced
 * motion, or a route drawn before, everything is there at once.
 *
 * `path` should be stable between renders (memoised), as it's measured.
 */
export function useRouteDraw(path: LatLng[], stops: LatLng[], key: string) {
  const motion = useMotion();
  const measured = useMemo(() => measurePath(path), [path]);
  const animate = !motion.reduced && measured.total > 0 && !drawn.has(key);
  const [len, setLen] = useState(animate ? 0 : Infinity);

  useEffect(() => {
    if (!animate) return;
    let raf = 0;
    let last = 0;
    const start = performance.now();
    const step = (now: number) => {
      const t = Math.min(1, (now - start) / Motion.routeDraw);
      if (t === 1) {
        drawn.add(key);
        setLen(Infinity);
        return;
      }
      if (now - last >= DRAW_FRAME) {
        last = now;
        setLen(easeInOut(t) * measured.total);
      }
      raf = requestAnimationFrame(step);
    };
    raf = requestAnimationFrame(step);
    return () => cancelAnimationFrame(raf);
  }, [animate, key, measured]);

  // Stops land as the line reaches them; with no line, one after another.
  const stopDelays = useMemo(
    () =>
      measured.total > 0
        ? stopFractions(measured, stops).map((f) => easeInOutInverse(f) * Motion.routeDraw)
        : stops.map((_, i) => dropDelay(i)),
    [measured, stops],
  );

  return {
    measured,
    path: len >= measured.total ? path : pathUpTo(measured, len),
    done: len >= measured.total,
    stopDelays,
  };
}

/**
 * Where the walker is: strolling the route over 5.2 s, resting 1.8 s, and
 * again. Only once the route is drawn, while the screen is on show, and
 * never with reduced motion (null then, and between walks).
 */
export function useWalker(measured: MeasuredPath, enabled: boolean) {
  const motion = useMotion();
  const active = useScreenActive();
  const walking = enabled && motion.loops && active && measured.total > 0;
  const [at, setAt] = useState<LatLng | null>(null);

  useEffect(() => {
    if (!walking) return;
    let raf = 0;
    let last = 0;
    let start = 0;
    let timer: ReturnType<typeof setTimeout>;
    const go = () => {
      start = performance.now();
      raf = requestAnimationFrame(step);
    };
    const step = (now: number) => {
      const t = Math.min(1, (now - start) / Motion.walk);
      if (t === 1) {
        setAt(null);
        timer = setTimeout(go, Motion.walkRest);
        return;
      }
      if (now - last >= WALK_FRAME) {
        last = now;
        setAt(pointAlong(measured, easeInOut(t) * measured.total));
      }
      raf = requestAnimationFrame(step);
    };
    // A beat after the route lands before the first walk.
    timer = setTimeout(go, 700);
    return () => {
      cancelAnimationFrame(raf);
      clearTimeout(timer);
    };
  }, [walking, measured]);

  return walking ? at : null;
}
