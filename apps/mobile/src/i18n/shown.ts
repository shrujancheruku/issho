export type Showing = 'primary' | 'secondary';

/**
 * Which language the switch really shows: the secondary only when the person
 * has one, so no screen has to guard against an empty language code.
 */
export function resolveShowing(secondaryLanguage: string, showing: Showing): Showing {
  return showing === 'secondary' && secondaryLanguage ? 'secondary' : 'primary';
}

/**
 * Orders a text the API sent in both display languages for the language being
 * shown: `name` goes first, `secondary` beside it. A missing translation
 * (the server sends '' while one is still being made) never swaps, so the
 * text that exists is always the one shown.
 */
export function pickShown(
  showing: Showing,
  primaryText: string,
  secondaryText: string | undefined,
): { name: string; secondary: string } {
  const secondary = secondaryText ?? '';
  return showing === 'secondary' && secondary
    ? { name: secondary, secondary: primaryText }
    : { name: primaryText, secondary };
}
