/**
 * The Vietnamese short weekday for a calendar date ("YYYY-MM-DD"), as this
 * runtime's ICU writes it. CLDR changed it between versions (Saturday is
 * "Th 7" in CLDR 47 and "Thứ 7" in later data), and Node bundles its own ICU,
 * so a hardcoded weekday passes on one Node and fails on another. A test builds
 * the weekday here and spells out the rest, so it still pins the day, the
 * order and the city's clock, which are the app's logic, and not CLDR's word.
 */
export const viWeekday = (date: string) => new Intl.DateTimeFormat('vi', { weekday: 'short', timeZone: 'UTC' }).format(new Date(`${date}T12:00:00Z`));
