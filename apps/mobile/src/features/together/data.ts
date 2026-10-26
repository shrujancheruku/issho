import { useMemo } from 'react';

import { useMyCrews } from '@/features/crews/data';
import { useMyEvents } from '@/features/events/data';
import { useFeed } from '@/features/plans/data';
import { useNow } from '@/hooks/use-now';

import { sortTogether } from './sort';

export type { NeedsYou } from './sort';

/** Together's sections, with the queries behind them for loading and refreshing. */
export function useTogether() {
  const events = useMyEvents();
  const crews = useMyCrews();
  const feed = useFeed();
  const now = useNow();
  const together = useMemo(
    () => sortTogether(events.data ?? [], crews.data ?? [], feed.data ?? [], now),
    [events.data, crews.data, feed.data, now],
  );
  return { ...together, events, crews, feed };
}
