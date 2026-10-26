import type { PlanStep } from '@issho/api-client';
import { useState } from 'react';
import { StyleSheet, View } from 'react-native';

import { SectionTitle } from '@/components/screen-header';
import { TextLink } from '@/components/text-link';
import { Spacing } from '@/constants/theme';
import { useI18n } from '@/i18n/i18n-provider';

import type { usePlanNames } from './names';
import { RouteMap } from './route-map';
import { Timeline } from './timeline';

/** The map and the stops in order, under "The walk" for a walk, else "The stops". */
export function PlanStops({
  steps,
  routePolyline,
  names,
  closed,
}: {
  steps: PlanStep[];
  routePolyline: string;
  names: ReturnType<typeof usePlanNames>;
  closed?: boolean;
}) {
  const { t } = useI18n();
  if (!steps.length) return null;
  // A walk's stops say how long to stay; other plans' don't.
  const walk = steps.some((s) => s.minutes > 0);
  return (
    <>
      <RouteMap steps={steps} routePolyline={routePolyline} />
      <View style={styles.section}>
        <SectionTitle>{t(walk ? 'plan.theWalk' : 'plan.theStops')}</SectionTitle>
        <Timeline steps={steps} names={names} closed={closed} />
      </View>
    </>
  );
}

/**
 * The stops and map behind a link, on outing screens where the one thing
 * to do comes first. Opening it is the person's action, so no motion.
 */
export function StopsToggle(props: Parameters<typeof PlanStops>[0]) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  if (!props.steps.length) return null;
  return (
    <>
      <TextLink label={t(open ? 'plan.hideStops' : 'plan.seeStops')} onPress={() => setOpen(!open)} />
      {open && <PlanStops {...props} />}
    </>
  );
}

const styles = StyleSheet.create({
  section: { gap: Spacing.three },
});
