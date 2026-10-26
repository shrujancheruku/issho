import { useEffect, useState } from 'react';
import { StyleSheet, View } from 'react-native';
import Animated, { useAnimatedStyle, useSharedValue, withDelay } from 'react-native-reanimated';

import { ThemedText } from '@/components/themed-text';
import { Motion } from '@/constants/motion';
import { Brand, Spacing } from '@/constants/theme';
import { useMotion } from '@/hooks/use-motion';
import { useScreenActive } from '@/hooks/use-screen-active';
import { useTheme } from '@/hooks/use-theme';
import { isRightToLeft } from '@/i18n/direction';
import { useI18n } from '@/i18n/i18n-provider';
import type { StringKey } from '@/i18n/strings';
import { haptics } from '@/lib/haptics';

/** An outing's five stages, in order. */
export const stageKeys: readonly StringKey[] = [
  'thread.signedUp',
  'thread.dateSet',
  'thread.going',
  'thread.onTheDay',
  'thread.howWasIt',
];
const LAST = stageKeys.length - 1;

// Outings whose thread has drawn itself this session: opening one again
// shows it drawn, so the draw-in doesn't replay on every visit.
const drawn = new Set<string>();

/**
 * The gold thread at the top of an outing: a knot per stage, tied up to the
 * current one. Opening an outing for the first time in a session draws the
 * line from the first knot to the current one, tying each knot as the line
 * reaches it. If the stage moves on while the screen is open, the line
 * grows one segment and the new knot ties with a light tap.
 *
 * The current knot breathes (a ring that grows and fades) while the screen
 * is on show: the screen's one live thing. Pass `live={false}` where
 * something else is live (on the day, the live dot by the time).
 */
export function Thread({ stage, id, live = true }: { stage: number; id: string; live?: boolean }) {
  const { t, languages } = useI18n();
  const theme = useTheme();
  const m = useMotion();
  const active = useScreenActive();
  const current = Math.max(0, Math.min(LAST, stage));
  const rtl = isRightToLeft(languages.primary);

  // First visit this session: start empty and draw in (once).
  const [drawIn] = useState(() => !drawn.has(id) && !m.reduced);
  const fill = useSharedValue(drawIn ? 0 : current / LAST);
  const [shownStage, setShownStage] = useState(current);
  const [tie, setTie] = useState<{ index: number; delay: number }[]>(() =>
    drawIn ? Array.from({ length: current + 1 }, (_, i) => ({ index: i, delay: i * Motion.draw })) : [],
  );
  // The stage moved on while the screen is open: tie the new knot.
  if (shownStage !== current) {
    setShownStage(current);
    if (current > shownStage) setTie([{ index: current, delay: Motion.draw * 0.6 }]);
  }

  useEffect(() => {
    drawn.add(id);
    fill.value = m.draw(current / LAST, drawIn ? current : 1);
  }, [current, id, drawIn, fill, m]);
  useEffect(() => {
    if (current > 0 && tie.length === 1 && tie[0].index === current) haptics.tap();
  }, [tie, current]);

  const fillStyle = useAnimatedStyle(() => ({ transform: [{ scaleX: fill.value }] }));

  return (
    <View
      accessible
      accessibilityRole="progressbar"
      accessibilityLabel={t('thread.label', { stage: t(stageKeys[current]) })}
      accessibilityValue={{ min: 1, max: stageKeys.length, now: current + 1 }}>
      <View style={[styles.track, { backgroundColor: theme.backgroundSelected }]} />
      <Animated.View
        style={[styles.track, styles.fill, { backgroundColor: theme.goldText, transformOrigin: rtl ? 'right' : 'left' }, fillStyle]}
      />
      <View style={styles.row}>
        {stageKeys.map((key, i) => {
          const tied = tie.find((k) => k.index === i);
          return (
            <View key={key} style={styles.cell}>
              <View style={styles.knotBox}>
                <Knot
                  state={i < current ? 'done' : i === current ? 'now' : 'next'}
                  tieDelay={tied?.delay}
                  breathing={i === current && live && m.loops}
                  paused={!active}
                />
              </View>
              <ThemedText
                type="small"
                themeColor={i === current ? 'text' : 'textSecondary'}
                style={[styles.label, i === current && styles.labelNow]}
                numberOfLines={2}>
                {t(key)}
              </ThemedText>
            </View>
          );
        })}
      </View>
    </View>
  );
}

const breathe = {
  '0%': { transform: [{ scale: 1 }], opacity: 0.7 },
  '70%': { transform: [{ scale: 2.1 }], opacity: 0 },
  '100%': { transform: [{ scale: 2.1 }], opacity: 0 },
};

function Knot({
  state,
  tieDelay,
  breathing,
  paused,
}: {
  state: 'done' | 'now' | 'next';
  tieDelay?: number;
  breathing: boolean;
  paused: boolean;
}) {
  const theme = useTheme();
  const m = useMotion();
  const scale = useSharedValue(tieDelay === undefined ? 1 : 0.4);
  useEffect(() => {
    if (tieDelay !== undefined) {
      scale.value = 0.4;
      scale.value = withDelay(tieDelay, m.gather(1));
    }
  }, [tieDelay, scale, m]);
  const tied = useAnimatedStyle(() => ({ transform: [{ scale: scale.value }] }));

  if (state === 'next') {
    return <View style={[styles.next, { borderColor: theme.lineStrong, backgroundColor: theme.background }]} />;
  }
  if (state === 'done') {
    return <Animated.View style={[styles.done, { backgroundColor: theme.goldText }, tied]} />;
  }
  return (
    <Animated.View style={[styles.now, { borderColor: theme.goldText, backgroundColor: theme.background }, tied]}>
      {breathing && (
        // A Reanimated CSS animation: runs on the UI thread, paused off screen.
        <Animated.View
          pointerEvents="none"
          style={[
            styles.ring,
            {
              borderColor: Brand.gold,
              animationName: breathe,
              animationDuration: Motion.breathe,
              animationIterationCount: 'infinite',
              animationTimingFunction: 'ease-out',
              animationPlayState: paused ? 'paused' : 'running',
            },
          ]}
        />
      )}
      <View style={[styles.core, { backgroundColor: theme.goldText }]} />
    </Animated.View>
  );
}

const KNOT_BOX = 22;

const styles = StyleSheet.create({
  // The track runs from the first knot's centre to the last one's: 10% in
  // from each side of five equal columns.
  track: { position: 'absolute', start: '10%', end: '10%', top: KNOT_BOX / 2 - 1, height: 2 },
  fill: {},
  row: { flexDirection: 'row' },
  cell: { flex: 1, alignItems: 'center', gap: Spacing.one + 2 },
  knotBox: { height: KNOT_BOX, alignItems: 'center', justifyContent: 'center' },
  label: { fontSize: 12, lineHeight: 16, textAlign: 'center' },
  labelNow: { fontWeight: 700 },
  next: { width: 12, height: 12, borderRadius: 6, borderWidth: 2 },
  done: { width: 14, height: 14, borderRadius: 7 },
  now: { width: 22, height: 22, borderRadius: 11, borderWidth: 3, alignItems: 'center', justifyContent: 'center' },
  core: { width: 8, height: 8, borderRadius: 4 },
  ring: { position: 'absolute', top: -6, start: -6, end: -6, bottom: -6, borderRadius: 999, borderWidth: 2 },
});
