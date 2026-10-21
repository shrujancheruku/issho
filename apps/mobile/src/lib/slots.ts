import { TimeBlock } from '@issho/api-client';

/** Parts of the day, in order. */
export const blocks = [TimeBlock.MORNING, TimeBlock.AFTERNOON, TimeBlock.EVENING];
/** Monday first; values are 0 = Sunday as on the server. */
export const weekdays = [1, 2, 3, 4, 5, 6, 0];

/** A weekly slot as a string, so lists of them compare and dedupe simply. */
export const slotKey = (weekday: number, block: TimeBlock) => `${weekday}:${block}`;

export function parseSlotKey(key: string): { weekday: number; block: TimeBlock } {
  const [weekday, block] = key.split(':').map(Number);
  return { weekday, block: block as TimeBlock };
}
